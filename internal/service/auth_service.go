package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	mrand "math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"starcrystal/server/internal/antifraud"
	"starcrystal/server/internal/errx"
	"starcrystal/server/internal/httpx"
	"starcrystal/server/internal/logger"
	providertypes "starcrystal/server/internal/provider"
	"starcrystal/server/internal/regpolicy"
	"starcrystal/server/internal/starcrystaljson"
	"starcrystal/server/internal/store"
)

type AuthService struct {
	mu     sync.Mutex
	secret []byte
	// 手机号验证码（本地内存；生产应接真实短信与 Redis）
	smsState                      map[string]*smsRecord
	playerRepo                    store.PlayerRepository
	goldLedger                    *GoldLedgerService
	goldRedis                     GoldRedisStore
	welfareRank                   *WelfareRankSync
	adsGate                       *antifraud.AdsGate
	gmSilentRestoreByAccount      map[string]time.Time
	maxAccountsPerDevice          int
	deviceSilentSecondsGuest      int
	deviceSilentSecondsEmailPhone int
	taskInviteHook                func(ctx context.Context, inviterAccountID string)
}

type smsRecord struct {
	Code   string
	Expire time.Time
	// 上次成功发送时间（简单冷却）
	lastSent time.Time
}

type userRecord struct {
	ID                string
	Email             string
	Phone             string
	PasswordHash      string
	Provider          string
	ProviderSub       string
	DisplayName       string
	InviteCode        string
	CurGold           float64
	TotalGold         float64
	CurToken          float64
	TotalToken        float64
	InvitedUserID     string
	InvitedUserID2    string
	InvitedUserName   string
	InvitedUserName2  string
	AdRewardsDisabled bool // 注册风控：不参与激励广告发奖
}

// PublicUser 对外返回的用户信息（不含密码）。
type PublicUser struct {
	ID, Email, Phone, DisplayName, Provider, InviteCode, InvitedUserID, InvitedUserID2, InvitedUserName, InvitedUserName2 string
	CurGold, TotalGold, CurToken, TotalToken                                                                              float64
	AdRewardsDisabled                                                                                                     bool // true=不发激励广告奖励
}

const (
	accountStatusBanned                  = 2
	deviceBanReason                      = "同设备账号同意注销旧账号"
	defaultMaxAccountsPerDevice          = 1
	defaultDeviceSilentSecondsGuest      = 6 * 60 * 60
	defaultDeviceSilentSecondsEmailPhone = 6 * 60 * 60
)

type DeviceAccountConflictError struct {
	ExistingAccountID string
	ExistingUserName  string
	QuietUntil        *time.Time
	RemainingSec      int64
	SilentSeconds     int64
	NeedConfirmation  bool
	Message           string
}

func (e *DeviceAccountConflictError) Error() string {
	if e == nil {
		return ""
	}
	if strings.TrimSpace(e.Message) != "" {
		return e.Message
	}
	if e.NeedConfirmation {
		return "该设备已有账号，请先确认注销旧账号"
	}
	if e.RemainingSec > 0 {
		return fmt.Sprintf("该设备旧账号静默中，请%d秒后再注册", e.RemainingSec)
	}
	return "该设备旧账号静默中，请稍后再注册"
}

type AccountBannedError struct {
	Reason string
}

func (e *AccountBannedError) Error() string {
	if e == nil || strings.TrimSpace(e.Reason) == "" {
		return "账号已被封禁"
	}
	return "账号已被封禁：" + strings.TrimSpace(e.Reason)
}

type AccountSilentError struct {
	QuietUntil   *time.Time
	RemainingSec int64
	AccountID    string
	CanCancel    bool
}

func (e *AccountSilentError) Error() string {
	if e == nil {
		return ""
	}
	if e.RemainingSec > 0 {
		return fmt.Sprintf("账号处于静默期，请%d秒后再试", e.RemainingSec)
	}
	return "账号处于静默期，请稍后再试"
}

func NewAuthService() *AuthService {
	secret := os.Getenv("AUTH_HMAC_SECRET")
	if strings.TrimSpace(secret) == "" {
		secret = "dev-insecure-auth-secret-change-me"
	}
	as := &AuthService{
		smsState:                      make(map[string]*smsRecord),
		secret:                        []byte(secret),
		adsGate:                       antifraud.NewAdsGateFromEnv(),
		gmSilentRestoreByAccount:      make(map[string]time.Time),
		maxAccountsPerDevice:          defaultMaxAccountsPerDevice,
		deviceSilentSecondsGuest:      defaultDeviceSilentSecondsGuest,
		deviceSilentSecondsEmailPhone: defaultDeviceSilentSecondsEmailPhone,
	}
	scCfg := loadStarcrystalServerConfig()
	as.maxAccountsPerDevice, as.deviceSilentSecondsGuest, as.deviceSilentSecondsEmailPhone = policyFromStarcrystal(scCfg)
	logger.Info(logger.TopicAuth, "starcrystal.json device policy: maxAccountsPerDevice=%d silentGuestSec=%d silentEmailPhoneSec=%d",
		as.maxAccountsPerDevice, as.deviceSilentSecondsGuest, as.deviceSilentSecondsEmailPhone)

	sms := strings.TrimSpace(os.Getenv("AUTH_SMS_MOCK"))
	if sms == "" {
		logger.FatalNotice(logger.TopicMain, "AUTH_SMS_MOCK=(unset)")
	} else {
		logger.FatalNotice(logger.TopicMain, "AUTH_SMS_MOCK=%s", sms)
	}

	fromEnv := strings.TrimSpace(os.Getenv("AUTH_MYSQL_DSN")) != ""
	dsn := resolveAuthMysqlDSN(scCfg)
	if dsn == "" {
		logger.FatalNotice(logger.TopicMain, "AUTH_MYSQL_DSN for this process: none (set env AUTH_MYSQL_DSN or configs/starcrystal.json authMysqlDsn; release/startsvr.ps1 -AuthMySqlDsn sets env for the child process)")
		logger.Fatal(logger.TopicAuth, "MySQL DSN required: set env AUTH_MYSQL_DSN or release/configs/starcrystal.json \"authMysqlDsn\"")
	}
	if fromEnv {
		logger.FatalNotice(logger.TopicMain, "AUTH_MYSQL_DSN for this process: source=environment AUTH_MYSQL_DSN (overrides starcrystal.json when set); effective=%s", redactAuthMysqlDSN(dsn))
	} else {
		logger.FatalNotice(logger.TopicMain, "AUTH_MYSQL_DSN for this process: source=starcrystal.json authMysqlDsn; effective=%s", redactAuthMysqlDSN(dsn))
	}
	ms, err := store.NewMySQLPlayerRepository(dsn)
	if err != nil {
		logger.Fatal(logger.TopicAuth, "MySQL repository init failed: %v", err)
	}
	as.playerRepo = ms
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	pingErr := ms.Ping(pingCtx)
	pingCancel()
	if pingErr != nil {
		logger.Fatal(logger.TopicAuth, "MySQL unreachable at startup (ping): %v", pingErr)
	}
	persistEffectiveAuthMysqlDSN(dsn)
	logger.FatalNotice(logger.TopicMain, "MySQL auth database: connected, startup ping ok (%s)", redactAuthMysqlDSN(dsn))
	go as.mysqlWatchdogLoop()
	return as
}

const mysqlWatchdogInterval = 15 * time.Second
const mysqlWatchdogRecoverRetries = 100
const mysqlWatchdogRetryDelay = 200 * time.Millisecond

func (s *AuthService) mysqlWatchdogLoop() {
	ticker := time.NewTicker(mysqlWatchdogInterval)
	defer ticker.Stop()
	ping := func() error {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.playerRepo.Ping(ctx)
	}
	first := true
	for {
		if !first {
			<-ticker.C
		}
		first = false
		pingErr := ping()
		if pingErr == nil {
			continue
		}
		logger.Warn(logger.TopicAuth, "mysql watchdog: ping failed, recovering (up to %d retries) err=%v", mysqlWatchdogRecoverRetries, pingErr)
		recovered := false
		for attempt := 1; attempt <= mysqlWatchdogRecoverRetries; attempt++ {
			time.Sleep(mysqlWatchdogRetryDelay)
			if err2 := ping(); err2 == nil {
				logger.Info(logger.TopicAuth, "mysql watchdog: recovered after %d retries", attempt)
				recovered = true
				break
			} else if attempt == 1 || attempt == 25 || attempt == 50 || attempt == 75 || attempt == mysqlWatchdogRecoverRetries {
				logger.Warn(logger.TopicAuth, "mysql watchdog: retry %d/%d still failing err=%v", attempt, mysqlWatchdogRecoverRetries, err2)
			}
		}
		if !recovered {
			logger.Fatal(logger.TopicAuth, "mysql watchdog: database unreachable after %d retries, exiting", mysqlWatchdogRecoverRetries)
		}
	}
}

type starCrystalConfig struct {
	Policy struct {
		MaxAccountsPerDevice          int `json:"maxAccountsPerDevice"`
		DeviceSilentSeconds           int `json:"deviceSilentSeconds,omitempty"`
		DeviceSilentSecondsGuest      int `json:"deviceSilentSecondsGuest"`
		DeviceSilentSecondsEmailPhone int `json:"deviceSilentSecondsEmailPhone"`
	} `json:"policy"`
	// AuthMysqlDsn Go MySQL DSN；可与 Unity StarCrystalConfig 同级字段对齐。非空时生效；env AUTH_MYSQL_DSN 优先覆盖。
	AuthMysqlDsn string `json:"authMysqlDsn,omitempty"`
}

func starcrystalConfigFileCandidates() []string {
	return starcrystaljson.ConfigCandidates()
}

func loadStarcrystalServerConfig() starCrystalConfig {
	var zero starCrystalConfig
	for _, p := range starcrystalConfigFileCandidates() {
		raw, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var cfg starCrystalConfig
		if err := json.Unmarshal(raw, &cfg); err != nil {
			continue
		}
		return cfg
	}
	return zero
}

// persistEffectiveAuthMysqlDSN 写入 release/log/last-auth-mysql-dsn.txt，供 PowerShell clean/rebuild 与真实连接一致。
func persistEffectiveAuthMysqlDSN(dsn string) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return
	}
	for _, logDir := range starcrystalReleaseLogDirsForPersist() {
		if err := os.MkdirAll(logDir, 0755); err != nil {
			logger.Warn(logger.TopicAuth, "auth mysql dsn record: mkdir %s: %v", logDir, err)
			continue
		}
		p := filepath.Join(logDir, "last-auth-mysql-dsn.txt")
		if err := os.WriteFile(p, []byte(dsn+"\n"), 0644); err != nil {
			logger.Warn(logger.TopicAuth, "auth mysql dsn record: write %s: %v", p, err)
		} else {
			logger.Info(logger.TopicAuth, "auth mysql dsn record (clean/rebuild scripts): %s", p)
		}
	}
}

// starcrystalReleaseLogDirBesideConfigs 与 cmd/api logDirBesideExecutable 一致：
// 配置在 <base>/configs/starcrystal.json 时日志目录为 <base>/log（勿再拼一层 release）。
func starcrystalReleaseLogDirBesideConfigs(baseDir string) (string, bool) {
	baseDir = filepath.Clean(baseDir)
	if baseDir == "" || baseDir == "." {
		return "", false
	}
	st, err := os.Stat(filepath.Join(baseDir, "configs", "starcrystal.json"))
	if err != nil || st.IsDir() {
		return "", false
	}
	return filepath.Join(baseDir, "log"), true
}

func starcrystalReleaseLogDirsForPersist() []string {
	var dirs []string
	seen := make(map[string]struct{})
	add := func(logDir string) {
		logDir = filepath.Clean(logDir)
		if logDir == "" || logDir == "." {
			return
		}
		if _, ok := seen[logDir]; ok {
			return
		}
		seen[logDir] = struct{}{}
		dirs = append(dirs, logDir)
	}
	for _, cfgPath := range starcrystalConfigFileCandidates() {
		if st, err := os.Stat(cfgPath); err != nil || st.IsDir() {
			continue
		}
		absCfg, err := filepath.Abs(cfgPath)
		if err != nil {
			absCfg = cfgPath
		}
		// .../release/configs/file.json -> .../release/log
		add(filepath.Join(filepath.Dir(filepath.Dir(absCfg)), "log"))
	}
	if wd, err := os.Getwd(); err == nil {
		if logDir, ok := starcrystalReleaseLogDirBesideConfigs(wd); ok {
			add(logDir)
		} else if logDir, ok := starcrystalReleaseLogDirBesideConfigs(filepath.Join(wd, "release")); ok {
			add(logDir)
		}
	}
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		if logDir, ok := starcrystalReleaseLogDirBesideConfigs(exeDir); ok {
			add(logDir)
		} else if logDir, ok := starcrystalReleaseLogDirBesideConfigs(filepath.Join(exeDir, "..")); ok {
			add(logDir)
		} else if logDir, ok := starcrystalReleaseLogDirBesideConfigs(filepath.Join(exeDir, "..", "release")); ok {
			add(logDir)
		}
	}
	return dirs
}

func redactAuthMysqlDSN(dsn string) string {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return ""
	}
	i := strings.Index(dsn, "@tcp(")
	if i <= 0 {
		return "(dsn format not logged)"
	}
	head := dsn[:i]
	tail := dsn[i:]
	c := strings.LastIndex(head, ":")
	if c <= 0 {
		return "(dsn format not logged)"
	}
	return head[:c+1] + "***" + tail
}

func policyFromStarcrystal(cfg starCrystalConfig) (maxAccountsPerDevice, silentGuestSec, silentEmailPhoneSec int) {
	maxAccountsPerDevice = defaultMaxAccountsPerDevice
	silentGuestSec = defaultDeviceSilentSecondsGuest
	silentEmailPhoneSec = defaultDeviceSilentSecondsEmailPhone
	if cfg.Policy.MaxAccountsPerDevice >= 1 {
		maxAccountsPerDevice = cfg.Policy.MaxAccountsPerDevice
	}
	// deviceSilentSeconds: one value for all account types (guest + email/phone); per-type keys below override.
	if cfg.Policy.DeviceSilentSeconds >= 1 {
		silentGuestSec = cfg.Policy.DeviceSilentSeconds
		silentEmailPhoneSec = cfg.Policy.DeviceSilentSeconds
	}
	if cfg.Policy.DeviceSilentSecondsGuest >= 1 {
		silentGuestSec = cfg.Policy.DeviceSilentSecondsGuest
	}
	if cfg.Policy.DeviceSilentSecondsEmailPhone >= 1 {
		silentEmailPhoneSec = cfg.Policy.DeviceSilentSecondsEmailPhone
	}
	return maxAccountsPerDevice, silentGuestSec, silentEmailPhoneSec
}

// resolveAuthMysqlDSN 优先使用环境变量 AUTH_MYSQL_DSN，否则使用 starcrystal.json 的 authMysqlDsn。
func resolveAuthMysqlDSN(cfg starCrystalConfig) string {
	if v := strings.TrimSpace(os.Getenv("AUTH_MYSQL_DSN")); v != "" {
		return v
	}
	return strings.TrimSpace(cfg.AuthMysqlDsn)
}

func (s *AuthService) resolveDeviceSilentSecondsByAccountType(accountType string) int {
	accountType = strings.ToLower(strings.TrimSpace(accountType))
	if accountType == providertypes.Guest {
		if s.deviceSilentSecondsGuest > 0 {
			return s.deviceSilentSecondsGuest
		}
		return defaultDeviceSilentSecondsGuest
	}
	if s.deviceSilentSecondsEmailPhone > 0 {
		return s.deviceSilentSecondsEmailPhone
	}
	return defaultDeviceSilentSecondsEmailPhone
}

// assignInviteCodeForNewAccount 新注册账号成功后写入 auth_invite_codes；失败则删除账号（级联设备映射/邀请关系）并返回固定中文错误。
func (s *AuthService) assignInviteCodeForNewAccount(ctx context.Context, accountID string, u *userRecord) error {
	if u == nil {
		return nil
	}
	inviteCode, ice := s.playerRepo.AllocateInviteCodeOnRegister(ctx, accountID)
	if ice != nil || strings.TrimSpace(inviteCode) == "" {
		_ = s.playerRepo.DeleteAuthAccountCascade(ctx, accountID)
		return fmt.Errorf("注册账号失败")
	}
	u.InviteCode = strings.TrimSpace(inviteCode)
	return nil
}

func (s *AuthService) validateAccountCanLogin(rec *store.AuthUserRecord) error {
	if rec == nil {
		return nil
	}
	if rec.Status == accountStatusBanned {
		return &AccountBannedError{Reason: strings.TrimSpace(rec.BanReason)}
	}
	if rec.DeviceSilentUntil != nil {
		until := rec.DeviceSilentUntil.UTC()
		now := time.Now()
		if now.Before(until) {
			remain := int64(time.Until(until).Seconds())
			if remain < 1 {
				remain = 1
			}
			return &AccountSilentError{
				QuietUntil:   rec.DeviceSilentUntil,
				RemainingSec: remain,
				AccountID:    strings.TrimSpace(rec.AccountID),
				CanCancel:    true,
			}
		}
	}
	return nil
}

// ensureInviterActiveForRegistration rejects registration when the invite code maps to a banned inviter account.
func (s *AuthService) ensureInviterActiveForRegistration(ctx context.Context, inviterAccountID string) error {
	inviterAccountID = strings.TrimSpace(inviterAccountID)
	if inviterAccountID == "" {
		return nil
	}
	rec, err := s.playerRepo.GetByAccountID(ctx, inviterAccountID)
	if err != nil {
		if store.IsNotFound(err) {
			return fmt.Errorf("invalid inviteCode")
		}
		return fmt.Errorf("db load inviter failed: %v", err)
	}
	if rec == nil {
		return fmt.Errorf("invalid inviteCode")
	}
	if rec.Status == accountStatusBanned {
		logger.Warn(logger.TopicAuth, "[invite] inviter banned inviter_account_id=%q reason=%q", inviterAccountID, strings.TrimSpace(rec.BanReason))
		return fmt.Errorf("邀请人账号已被封禁，无法使用此邀请码注册")
	}
	return nil
}

func (s *AuthService) handleSilentDuringLogin(rec *store.AuthUserRecord, passwordKey, password string, cancelSilent bool) error {
	if rec == nil || rec.DeviceSilentUntil == nil {
		return nil
	}
	until := rec.DeviceSilentUntil.UTC()
	if !time.Now().Before(until) {
		// Silent period expired: clear flag on login path and allow normal login.
		if strings.TrimSpace(rec.AccountID) != "" {
			_ = s.playerRepo.ClearDeviceSilentByAccountID(context.Background(), rec.AccountID)
		}
		return nil
	}
	if !cancelSilent {
		return s.validateAccountCanLogin(rec)
	}
	if rec.PasswordHash == "" {
		return errx.ErrPasswordLoginUnavailable
	}
	if !checkPasswordHash(rec.PasswordHash, passwordKey, password) {
		return errx.ErrWrongPassword
	}
	return s.playerRepo.ClearDeviceSilentByAccountID(context.Background(), rec.AccountID)
}

func (s *AuthService) enforceDevicePolicyOnRegister(ctx context.Context, accountID string, deviceID string, confirmDeactivateOldAccount bool) error {
	deviceID = truncateDeviceID(deviceID)
	if deviceID == "" {
		logger.Debug(logger.TopicAuth, "[device-policy] skip: empty device id account_id=%q", strings.TrimSpace(accountID))
		return nil
	}
	maxAccountsPerDevice := s.maxAccountsPerDevice
	if maxAccountsPerDevice < 1 {
		maxAccountsPerDevice = defaultMaxAccountsPerDevice
	}
	activeCount, err := s.playerRepo.CountActiveAccountsByDeviceID(ctx, deviceID)
	if err != nil {
		return err
	}
	logger.Info(logger.TopicAuth, "[device-policy] account_id=%q device_id=%q active_count=%d limit=%d confirm_deactivate=%t",
		strings.TrimSpace(accountID), deviceID, activeCount, maxAccountsPerDevice, confirmDeactivateOldAccount)
	if activeCount < maxAccountsPerDevice {
		logger.Debug(logger.TopicAuth, "[device-policy] pass under limit account_id=%q device_id=%q", strings.TrimSpace(accountID), deviceID)
		return nil
	}
	existing, err := s.playerRepo.FindLatestActiveAccountByDeviceID(ctx, deviceID)
	if err != nil || existing == nil {
		return err
	}
	if strings.TrimSpace(existing.AccountID) == strings.TrimSpace(accountID) {
		return nil
	}
	existingUserName := strings.TrimSpace(existing.DisplayName)
	if existingUserName == "" {
		existingUserName = strings.TrimSpace(existing.Email)
	}
	if existingUserName == "" {
		existingUserName = strings.TrimSpace(existing.Phone)
	}
	if existingUserName == "" {
		existingUserName = strings.TrimSpace(existing.AccountID)
	}
	now := time.Now()
	if existing.DeviceSilentUntil != nil {
		until := existing.DeviceSilentUntil.UTC()
		if now.Before(until) {
			remain := int64(time.Until(until).Seconds())
			if remain < 1 {
				remain = 1
			}
			silentSeconds := s.resolveDeviceSilentSecondsByAccountType(existing.AccountType)
			return &DeviceAccountConflictError{
				ExistingAccountID: strings.TrimSpace(existing.AccountID),
				ExistingUserName:  existingUserName,
				QuietUntil:        existing.DeviceSilentUntil,
				RemainingSec:      remain,
				SilentSeconds:     int64(silentSeconds),
				NeedConfirmation:  false,
				Message:           "该设备旧账号已进入静默期，请到期后再注册新账号",
			}
		}
		logger.Info(logger.TopicAuth, "[device-policy] silent expired -> ban existing account account_id=%q account_type=%q reason=%q",
			strings.TrimSpace(existing.AccountID), strings.TrimSpace(existing.AccountType), deviceBanReason)
		if be := s.playerRepo.BanAccountByAccountID(ctx, existing.AccountID, deviceBanReason); be != nil {
			if store.IsNotFound(be) {
				logger.Warn(logger.TopicAuth, "[device-policy] ban existing account skipped(not found) account_id=%q", strings.TrimSpace(existing.AccountID))
			} else {
				logger.Error(logger.TopicAuth, "[device-policy] ban existing account failed account_id=%q err=%v", strings.TrimSpace(existing.AccountID), be)
				return be
			}
		} else {
			logger.Info(logger.TopicAuth, "[device-policy] ban existing account success account_id=%q account_type=%q",
				strings.TrimSpace(existing.AccountID), strings.TrimSpace(existing.AccountType))
		}
		return nil
	}
	if !confirmDeactivateOldAccount {
		silentSeconds := s.resolveDeviceSilentSecondsByAccountType(existing.AccountType)
		return &DeviceAccountConflictError{
			ExistingAccountID: strings.TrimSpace(existing.AccountID),
			ExistingUserName:  existingUserName,
			SilentSeconds:     int64(silentSeconds),
			NeedConfirmation:  true,
			Message:           "该设备已有账号，请先确认注销旧账号",
		}
	}
	silentSeconds := s.resolveDeviceSilentSecondsByAccountType(existing.AccountType)
	until := now.Add(time.Duration(silentSeconds) * time.Second)
	logger.Info(logger.TopicAuth, "[device-policy] set silent existing account_id=%q account_type=%q silent_seconds=%d until=%s",
		strings.TrimSpace(existing.AccountID), strings.TrimSpace(existing.AccountType), silentSeconds, until.UTC().Format(time.RFC3339))
	if se := s.playerRepo.SetDeviceSilentUntilByAccountID(ctx, existing.AccountID, until); se != nil {
		return se
	}
	return &DeviceAccountConflictError{
		ExistingAccountID: strings.TrimSpace(existing.AccountID),
		ExistingUserName:  existingUserName,
		QuietUntil:        &until,
		RemainingSec:      int64(silentSeconds),
		SilentSeconds:     int64(silentSeconds),
		NeedConfirmation:  false,
		Message:           fmt.Sprintf("已受理注销旧账号申请，请%d秒后再注册新账号", silentSeconds),
	}
}

func (s *AuthService) LoginEmailPassword(email, password string, cancelSilent bool) (*userRecord, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return nil, "", errx.ErrInvalidEmailFormat
	}
	ctx := context.Background()
	accountID := "email_" + email
	rec, e := s.playerRepo.GetByAccountID(ctx, accountID)
	if e != nil {
		if store.IsNotFound(e) {
			return nil, "", errx.ErrAccountNotFound
		}
		return nil, "", fmt.Errorf("db login failed: %v", e)
	}
	if rec.PasswordHash == "" {
		return nil, "", errx.ErrPasswordLoginUnavailable
	}
	if rec.Status == accountStatusBanned {
		return nil, "", &AccountBannedError{Reason: strings.TrimSpace(rec.BanReason)}
	}
	if se := s.handleSilentDuringLogin(rec, email, password, cancelSilent); se != nil {
		return nil, "", se
	}
	if !checkPasswordHash(rec.PasswordHash, email, password) {
		return nil, "", errx.ErrWrongPassword
	}
	u := &userRecord{
		ID:           rec.UserID,
		Email:        rec.Email,
		Phone:        rec.Phone,
		PasswordHash: rec.PasswordHash,
		Provider:     rec.Provider,
		DisplayName:  rec.DisplayName,
		CurGold:      rec.CurGold,
		TotalGold:    rec.TotalGold,
		CurToken:     rec.CurToken,
		TotalToken:   rec.TotalToken,
	}
	u.AdRewardsDisabled = rec.AdRewardsDisabled != 0
	if inviteCode, ice := s.playerRepo.GetInviteCodeByAccountID(ctx, accountID); ice == nil {
		u.InviteCode = strings.TrimSpace(inviteCode)
	}
	directAccountID, directNickname, secondAccountID, secondNickname, ie := s.playerRepo.GetInviterInfoByAccountID(ctx, accountID)
	if ie == nil {
		u.InvitedUserID = strings.TrimSpace(directAccountID)
		u.InvitedUserID2 = strings.TrimSpace(secondAccountID)
		u.InvitedUserName = strings.TrimSpace(directNickname)
		u.InvitedUserName2 = strings.TrimSpace(secondNickname)
	}
	tok, te := s.issueToken(accountID)
	return u, tok, te
}

// normPhone 保留 + 与数字，便于统一键。
func normPhone(phone string) string {
	phone = strings.TrimSpace(phone)
	var b strings.Builder
	for _, r := range phone {
		if r == '+' {
			if b.Len() == 0 {
				b.WriteRune(r)
			}
			continue
		}
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func normAccountID(account string) (kind, key string, err error) {
	a := strings.TrimSpace(account)
	if strings.Contains(a, "@") {
		k := strings.ToLower(a)
		if len(k) < 5 {
			return "", "", errx.ErrInvalidEmailFormat
		}
		return "email", k, nil
	}
	p := normPhone(a)
	if len(p) < 8 {
		return "", "", errx.ErrInvalidPhoneFormat
	}
	return "phone", p, nil
}

// InferAccountKind returns "email" or "phone" using the same rules as verification/login account normalization.
func InferAccountKind(account string) (string, error) {
	k, _, e := normAccountID(account)
	return k, e
}

// SendPhoneSms 记录验证码与冷却；开发环境可设置 env AUTH_SMS_MOCK=1 在 JSON 中返回 devVerifyCode。
func (s *AuthService) SendPhoneSms(phone string) (cooldownSec int, devCode string, err error) {
	_, p, e := normAccountID(phone)
	if e != nil {
		return 0, "", e
	}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.smsState[p]
	if ok && st.lastSent.After(now) {
		wait := int(st.lastSent.Sub(now).Seconds() + 0.5)
		if wait < 0 {
			wait = 0
		}
		if wait > 0 {
			return wait, "", fmt.Errorf("send too often, try after %d seconds", wait)
		}
	}
	code := fmt.Sprintf("%06d", 100000+mrand.Intn(900000))
	if st == nil {
		s.smsState[p] = &smsRecord{}
		st = s.smsState[p]
	}
	st.Code = code
	st.Expire = now.Add(10 * time.Minute)
	st.lastSent = now.Add(60 * time.Second) // 冷却 60s
	// 真实环境在此调用短信网关
	if os.Getenv("AUTH_SMS_MOCK") == "1" {
		devCode = code
	}
	return 60, devCode, nil
}

func (s *AuthService) SendVerifyCode(account string) (cooldownSec int, devCode string, err error) {
	kind, key, e := normAccountID(account)
	if e != nil {
		return 0, "", e
	}
	if kind == "phone" {
		return s.SendPhoneSms(key)
	}
	// email flow: send verification code by SMTP
	now := time.Now()
	s.mu.Lock()
	st, ok := s.smsState[key]
	if ok && st.lastSent.After(now) {
		wait := int(st.lastSent.Sub(now).Seconds() + 0.5)
		if wait > 0 {
			s.mu.Unlock()
			return wait, "", fmt.Errorf("send too often, try after %d seconds", wait)
		}
	}
	code := fmt.Sprintf("%06d", 100000+mrand.Intn(900000))
	if st == nil {
		s.smsState[key] = &smsRecord{}
		st = s.smsState[key]
	}
	st.Code = code
	st.Expire = now.Add(10 * time.Minute)
	st.lastSent = now.Add(60 * time.Second)
	s.mu.Unlock()

	if se := deliverRegisterVerifyEmail(key, code); se != nil {
		return 0, "", se
	}
	if isAuthSmsMockEnabledFromSMTPFile() {
		devCode = code
	}
	return 60, devCode, nil
}

func (s *AuthService) ensureRegisteredForPasswordReset(kind, key string) error {
	accountID := kind + "_" + key
	if _, ge := s.playerRepo.GetByAccountID(context.Background(), accountID); ge != nil {
		if store.IsNotFound(ge) {
			return errx.ErrPasswordResetAccountNotFound
		}
		return fmt.Errorf("db lookup account: %w", ge)
	}
	return nil
}

func (s *AuthService) PasswordResetSend(account string) (cooldownSec int, devCode string, err error) {
	account = strings.TrimSpace(account)
	if account == "" {
		return 0, "", fmt.Errorf("account is required")
	}
	kind, key, e := normAccountID(account)
	if e != nil {
		return 0, "", e
	}
	if err := s.ensureRegisteredForPasswordReset(kind, key); err != nil {
		return 0, "", err
	}
	if kind == "phone" {
		return s.SendPhoneSms(key)
	}

	emailKey := key
	now := time.Now()
	s.mu.Lock()
	st, ok := s.smsState[emailKey]
	if ok && st.lastSent.After(now) {
		wait := int(st.lastSent.Sub(now).Seconds() + 0.5)
		if wait > 0 {
			s.mu.Unlock()
			return wait, "", fmt.Errorf("send too often, try after %d seconds", wait)
		}
	}
	code := fmt.Sprintf("%06d", 100000+mrand.Intn(900000))
	if st == nil {
		s.smsState[emailKey] = &smsRecord{}
		st = s.smsState[emailKey]
	}
	st.Code = code
	st.Expire = now.Add(10 * time.Minute)
	st.lastSent = now.Add(60 * time.Second)
	s.mu.Unlock()

	if se := deliverPasswordResetEmail(emailKey, code); se != nil {
		return 0, "", se
	}
	if isAuthSmsMockEnabledFromSMTPFile() {
		devCode = code
	}
	return 60, devCode, nil
}

func (s *AuthService) PasswordResetConfirm(email, code, newPassword, deviceID, clientIP string) (*userRecord, string, error) {
	kind, key, err := normAccountID(strings.TrimSpace(email))
	if err != nil {
		return nil, "", err
	}
	if len(strings.TrimSpace(newPassword)) < 6 {
		return nil, "", fmt.Errorf("password must be at least 6 chars")
	}
	s.mu.Lock()
	st, ok := s.smsState[key]
	if !ok || st.Code == "" || time.Now().After(st.Expire) {
		s.mu.Unlock()
		return nil, "", fmt.Errorf("verification code expired, send again")
	}
	if st.Code != strings.TrimSpace(code) {
		s.mu.Unlock()
		return nil, "", fmt.Errorf("wrong verification code")
	}
	st.Code = ""
	s.mu.Unlock()

	pwHash, pe := hashPasswordPBKDF2Style(key, newPassword)
	if pe != nil {
		return nil, "", pe
	}
	accountID := kind + "_" + key
	if ue := s.playerRepo.UpdatePasswordByAccountID(context.Background(), accountID, pwHash); ue != nil {
		if store.IsNotFound(ue) {
			return nil, "", errx.ErrPasswordResetAccountNotFound
		}
		return nil, "", fmt.Errorf("db reset password failed: %v", ue)
	}
	rec, ge := s.playerRepo.GetByAccountID(context.Background(), accountID)
	if ge != nil {
		if store.IsNotFound(ge) {
			return nil, "", errx.ErrPasswordResetAccountNotFound
		}
		return nil, "", fmt.Errorf("db load account after reset: %v", ge)
	}
	if se := s.validateAccountCanLogin(rec); se != nil {
		return nil, "", se
	}
	u := &userRecord{
		ID:           rec.UserID,
		Email:        rec.Email,
		Phone:        rec.Phone,
		PasswordHash: rec.PasswordHash,
		Provider:     rec.Provider,
		DisplayName:  rec.DisplayName,
		CurGold:      rec.CurGold,
		TotalGold:    rec.TotalGold,
		CurToken:     rec.CurToken,
		TotalToken:   rec.TotalToken,
	}
	u.AdRewardsDisabled = rec.AdRewardsDisabled != 0
	if inviteCode, ice := s.playerRepo.GetInviteCodeByAccountID(context.Background(), accountID); ice == nil {
		u.InviteCode = strings.TrimSpace(inviteCode)
	}
	directAccountID, directNickname, secondAccountID, secondNickname, ie := s.playerRepo.GetInviterInfoByAccountID(context.Background(), accountID)
	if ie == nil {
		u.InvitedUserID = strings.TrimSpace(directAccountID)
		u.InvitedUserID2 = strings.TrimSpace(secondAccountID)
		u.InvitedUserName = strings.TrimSpace(directNickname)
		u.InvitedUserName2 = strings.TrimSpace(secondNickname)
	}
	tok, te := s.issueToken(accountID)
	return u, tok, te
}

// RegisterPhone 使用短信验证码 + 密码注册。
func (s *AuthService) RegisterPhone(phone, code, password, displayName string, deviceID, clientIP string) (*userRecord, string, error) {
	p := normPhone(phone)
	if len(p) < 8 {
		return nil, "", fmt.Errorf("invalid phone")
	}
	if len(code) < 4 || len(password) < 6 {
		return nil, "", fmt.Errorf("invalid code or password (min 6 chars)")
	}
	return s.registerPhoneWithStore(p, code, password, displayName, deviceID, clientIP)
}

func (s *AuthService) registerPhoneWithStore(p, code, password, displayName string, deviceID, clientIP string) (*userRecord, string, error) {
	ctx := context.Background()
	accountID := "phone_" + p

	existing, ge := s.playerRepo.GetByAccountID(ctx, accountID)
	if ge == nil && existing != nil {
		if se := s.validateAccountCanLogin(existing); se != nil {
			return nil, "", se
		}
		return nil, "", fmt.Errorf("phone already registered")
	}
	if ge != nil && !store.IsNotFound(ge) {
		return nil, "", fmt.Errorf("db register lookup failed: %v", ge)
	}

	s.mu.Lock()
	st, ok := s.smsState[p]
	if !ok || st.Code == "" {
		s.mu.Unlock()
		return nil, "", fmt.Errorf("send sms first or code expired")
	}
	if time.Now().After(st.Expire) {
		s.mu.Unlock()
		return nil, "", fmt.Errorf("code expired, request a new one")
	}
	if st.Code != code {
		s.mu.Unlock()
		return nil, "", fmt.Errorf("wrong verification code")
	}
	s.mu.Unlock()

	disabled, err := s.computeAdRewardsDisabledNewRegistration(ctx, deviceID, clientIP)
	if err != nil {
		return nil, "", err
	}
	pwHash, err := hashPasswordPBKDF2Style(p, password)
	if err != nil {
		return nil, "", err
	}
	rec := &store.AuthUserRecord{
		UserID:         accountID,
		AccountID:      accountID,
		AccountType:    providertypes.Phone,
		AccountValue:   p,
		Phone:          p,
		PasswordHash:   pwHash,
		Provider:       providertypes.Phone,
		DisplayName:    ensureDisplayName(displayName),
		DeviceID:       truncateDeviceID(deviceID),
		RegistrationIP: strings.TrimSpace(clientIP),
	}
	if de := s.enforceDevicePolicyOnRegister(ctx, accountID, rec.DeviceID, false); de != nil {
		return nil, "", de
	}
	s.mu.Lock()
	st2, ok2 := s.smsState[p]
	if !ok2 || st2.Code == "" {
		s.mu.Unlock()
		return nil, "", fmt.Errorf("send sms first or code expired")
	}
	if time.Now().After(st2.Expire) {
		s.mu.Unlock()
		return nil, "", fmt.Errorf("code expired, request a new one")
	}
	if st2.Code != code {
		s.mu.Unlock()
		return nil, "", fmt.Errorf("wrong verification code")
	}
	st2.Code = ""
	s.mu.Unlock()

	if disabled {
		rec.AdRewardsDisabled = 1
	}
	if ce := s.playerRepo.CreateByAccountID(ctx, rec); ce != nil {
		return nil, "", fmt.Errorf("db register create failed: %v", ce)
	}
	if ume := s.playerRepo.UpsertDeviceAccountMap(ctx, rec.DeviceID, accountID); ume != nil {
		_ = s.playerRepo.DeleteAuthAccountCascade(ctx, accountID)
		return nil, "", fmt.Errorf("db upsert device map failed: %v", ume)
	}

	u := &userRecord{
		ID:                accountID,
		Phone:             p,
		PasswordHash:      pwHash,
		Provider:          providertypes.Phone,
		DisplayName:       rec.DisplayName,
		AdRewardsDisabled: disabled,
	}
	if err := s.assignInviteCodeForNewAccount(ctx, accountID, u); err != nil {
		return nil, "", err
	}
	tok, te := s.issueToken(accountID)
	return u, tok, te
}

func (s *AuthService) LoginByAccount(account, password string, cancelSilent bool) (*userRecord, string, error) {
	kind, key, err := normAccountID(account)
	if err != nil {
		return nil, "", err
	}
	accountID := kind + "_" + key
	ctx := context.Background()
	rec, e := s.playerRepo.GetByAccountID(ctx, accountID)
	if e != nil {
		if store.IsNotFound(e) {
			return nil, "", errx.ErrAccountNotFound
		}
		return nil, "", fmt.Errorf("db login failed: %v", e)
	}
	if rec.PasswordHash == "" {
		return nil, "", errx.ErrPasswordLoginUnavailable
	}
	if rec.Status == accountStatusBanned {
		return nil, "", &AccountBannedError{Reason: strings.TrimSpace(rec.BanReason)}
	}
	if se := s.handleSilentDuringLogin(rec, key, password, cancelSilent); se != nil {
		return nil, "", se
	}
	if !checkPasswordHash(rec.PasswordHash, key, password) {
		return nil, "", errx.ErrWrongPassword
	}
	u := &userRecord{
		ID:           rec.UserID,
		Email:        rec.Email,
		Phone:        rec.Phone,
		PasswordHash: rec.PasswordHash,
		Provider:     rec.Provider,
		DisplayName:  rec.DisplayName,
		CurGold:      rec.CurGold,
		TotalGold:    rec.TotalGold,
		CurToken:     rec.CurToken,
		TotalToken:   rec.TotalToken,
	}
	u.AdRewardsDisabled = rec.AdRewardsDisabled != 0
	if inviteCode, ice := s.playerRepo.GetInviteCodeByAccountID(ctx, accountID); ice == nil {
		u.InviteCode = strings.TrimSpace(inviteCode)
	}
	directAccountID, directNickname, secondAccountID, secondNickname, ie := s.playerRepo.GetInviterInfoByAccountID(ctx, accountID)
	if ie == nil {
		u.InvitedUserID = strings.TrimSpace(directAccountID)
		u.InvitedUserID2 = strings.TrimSpace(secondAccountID)
		u.InvitedUserName = strings.TrimSpace(directNickname)
		u.InvitedUserName2 = strings.TrimSpace(secondNickname)
	}
	tok, te := s.issueToken(u.ID)
	return u, tok, te
}

func (s *AuthService) RegisterByCode(account, code, password, displayName, inviteCode string, deviceID, clientIP string, confirmDeactivateOldAccount bool) (*userRecord, string, error) {
	traceID := "reg-code-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	logger.DebugTrace(traceID, logger.TopicAuth,
		"[register-by-code] begin account=%q invite_code=%q device_id=%q confirm_deactivate=%t",
		strings.TrimSpace(account), strings.TrimSpace(inviteCode), strings.TrimSpace(deviceID), confirmDeactivateOldAccount)
	kind, key, err := normAccountID(account)
	if err != nil {
		logger.Warn(logger.TopicAuth, "[register-by-code] account normalize failed trace=%s account=%q err=%v", traceID, strings.TrimSpace(account), err)
		return nil, "", err
	}
	if len(password) < 6 {
		logger.Warn(logger.TopicAuth, "[register-by-code] password too short trace=%s account=%q", traceID, strings.TrimSpace(account))
		return nil, "", fmt.Errorf("password must be at least 6 chars")
	}
	codeTrim := strings.TrimSpace(code)
	s.mu.Lock()
	st, ok := s.smsState[key]
	if !ok || st.Code == "" || time.Now().After(st.Expire) {
		s.mu.Unlock()
		return nil, "", fmt.Errorf("verification code expired, send again")
	}
	if st.Code != codeTrim {
		s.mu.Unlock()
		return nil, "", fmt.Errorf("wrong verification code")
	}
	logger.DebugTrace(traceID, logger.TopicAuth, "[register-by-code] verification code accepted kind=%q key=%q", kind, key)
	s.mu.Unlock()

	ctx := context.Background()
	accountID := kind + "_" + key
	logger.DebugTrace(traceID, logger.TopicAuth, "[register-by-code] db path account_id=%q", accountID)
	existing, ge := s.playerRepo.GetByAccountID(ctx, accountID)
	if ge == nil && existing != nil {
		if se := s.validateAccountCanLogin(existing); se != nil {
			logger.Warn(logger.TopicAuth, "[register-by-code] existing account blocked trace=%s account_id=%q err=%v", traceID, accountID, se)
			return nil, "", se
		}
		logger.Warn(logger.TopicAuth, "[register-by-code] account already registered trace=%s account_id=%q", traceID, accountID)
		return nil, "", fmt.Errorf("%s already registered", kind)
	}
	if ge != nil && !store.IsNotFound(ge) {
		logger.Error(logger.TopicAuth, "[register-by-code] lookup existing failed trace=%s account_id=%q err=%v", traceID, accountID, ge)
		return nil, "", fmt.Errorf("db register lookup failed: %v", ge)
	}

	pwHash, pe := hashPasswordPBKDF2Style(key, password)
	if pe != nil {
		return nil, "", pe
	}
	u := &userRecord{
		ID:           accountID,
		PasswordHash: pwHash,
		Provider:     kind,
		DisplayName:  ensureDisplayName(displayName),
	}
	rec := &store.AuthUserRecord{
		UserID:       accountID,
		AccountID:    accountID,
		AccountType:  kind,
		AccountValue: key,
		PasswordHash: pwHash,
		Provider:     kind,
		DisplayName:  u.DisplayName,
	}
	inviteToken := strings.TrimSpace(inviteCode)
	if inviteToken != "" {
		logger.DebugTrace(traceID, logger.TopicAuth, "[register-by-code] resolving invite code=%q", inviteToken)
		inviterAccountID, ie := s.playerRepo.ResolveAccountIDByInviteCode(ctx, inviteToken)
		if ie != nil {
			logger.Warn(logger.TopicAuth, "[register-by-code] resolve invite code failed trace=%s code=%q err=%v", traceID, inviteToken, ie)
			return nil, "", fmt.Errorf("resolve inviteCode failed: %v", ie)
		}
		if strings.TrimSpace(inviterAccountID) == "" {
			logger.Warn(logger.TopicAuth, "[register-by-code] invite code not found trace=%s code=%q", traceID, inviteToken)
			return nil, "", fmt.Errorf("invalid inviteCode")
		}
		if ive := s.ensureInviterActiveForRegistration(ctx, inviterAccountID); ive != nil {
			logger.Warn(logger.TopicAuth, "[register-by-code] inviter not allowed trace=%s token=%q inviter=%q err=%v", traceID, inviteToken, strings.TrimSpace(inviterAccountID), ive)
			return nil, "", ive
		}
		rec.InvitedUserID = strings.TrimSpace(inviterAccountID)
		logger.DebugTrace(traceID, logger.TopicAuth, "[register-by-code] invite token resolved inviter_account_id=%q", rec.InvitedUserID)
	}
	if kind == "email" {
		u.Email = key
		rec.Email = key
	} else {
		u.Phone = key
		rec.Phone = key
	}
	disabled, de := s.computeAdRewardsDisabledNewRegistration(ctx, deviceID, clientIP)
	if de != nil {
		logger.Error(logger.TopicAuth, "[register-by-code] compute ad policy failed trace=%s account_id=%q err=%v", traceID, accountID, de)
		return nil, "", de
	}
	rec.DeviceID = truncateDeviceID(deviceID)
	rec.RegistrationIP = strings.TrimSpace(clientIP)
	logger.DebugTrace(traceID, logger.TopicAuth, "[register-by-code] before device policy account_id=%q device_id=%q client_ip=%q", accountID, rec.DeviceID, rec.RegistrationIP)
	if de := s.enforceDevicePolicyOnRegister(ctx, accountID, rec.DeviceID, confirmDeactivateOldAccount); de != nil {
		logger.Warn(logger.TopicAuth, "[register-by-code] device policy blocked trace=%s account_id=%q device_id=%q err=%v", traceID, accountID, rec.DeviceID, de)
		return nil, "", de
	}
	// Consume SMS only after device policy passes, so a 1420 conflict response does not
	// burn the code before the client retries with confirmDeactivateOldAccount=true.
	s.mu.Lock()
	st2, ok2 := s.smsState[key]
	if !ok2 || st2.Code == "" || time.Now().After(st2.Expire) {
		s.mu.Unlock()
		return nil, "", fmt.Errorf("verification code expired, send again")
	}
	if st2.Code != codeTrim {
		s.mu.Unlock()
		return nil, "", fmt.Errorf("wrong verification code")
	}
	st2.Code = ""
	s.mu.Unlock()

	if disabled {
		rec.AdRewardsDisabled = 1
	}
	if ce := s.playerRepo.CreateByAccountID(ctx, rec); ce != nil {
		logger.Error(logger.TopicAuth, "[register-by-code] create account failed trace=%s account_id=%q err=%v", traceID, accountID, ce)
		return nil, "", fmt.Errorf("db register create failed: %v", ce)
	}
	if ume := s.playerRepo.UpsertDeviceAccountMap(ctx, rec.DeviceID, accountID); ume != nil {
		_ = s.playerRepo.DeleteAuthAccountCascade(ctx, accountID)
		logger.Error(logger.TopicAuth, "[register-by-code] upsert device map failed trace=%s account_id=%q device_id=%q err=%v", traceID, accountID, rec.DeviceID, ume)
		return nil, "", fmt.Errorf("db upsert device map failed: %v", ume)
	}
	logger.Info(logger.TopicAuth, "[register-by-code] account created and device map upserted trace=%s account_id=%q device_id=%q", traceID, accountID, rec.DeviceID)
	u.AdRewardsDisabled = disabled
	if rec.InvitedUserID != "" {
		if me := s.playerRepo.AddInviteMember(ctx, rec.InvitedUserID, accountID); me != nil {
			logger.Error(logger.TopicAuth, "[register-by-code] add invite member failed trace=%s inviter=%q invitee=%q err=%v", traceID, rec.InvitedUserID, accountID, me)
			return nil, "", fmt.Errorf("db insert invite member failed: %v", me)
		}
		logger.Info(logger.TopicAuth, "[register-by-code] invite member linked trace=%s inviter=%q invitee=%q", traceID, rec.InvitedUserID, accountID)
		s.notifyTaskInvite(ctx, rec.InvitedUserID)
	}
	if err := s.assignInviteCodeForNewAccount(ctx, accountID, u); err != nil {
		logger.Error(logger.TopicAuth, "[register-by-code] assign invite code failed trace=%s account_id=%q err=%v", traceID, accountID, err)
		return nil, "", err
	}
	directAccountID, directNickname, secondAccountID, secondNickname, ie := s.playerRepo.GetInviterInfoByAccountID(ctx, accountID)
	if ie == nil {
		u.InvitedUserID = strings.TrimSpace(directAccountID)
		u.InvitedUserID2 = strings.TrimSpace(secondAccountID)
		u.InvitedUserName = strings.TrimSpace(directNickname)
		u.InvitedUserName2 = strings.TrimSpace(secondNickname)
	}
	tok, te := s.issueToken(accountID)
	if te != nil {
		logger.Error(logger.TopicAuth, "[register-by-code] issue token failed trace=%s account_id=%q err=%v", traceID, accountID, te)
		return nil, "", te
	}
	logger.Info(logger.TopicAuth, "[register-by-code] success trace=%s account_id=%q", traceID, accountID)
	return u, tok, te
}

func ensureDisplayName(name string) string {
	n := strings.TrimSpace(name)
	if n != "" {
		return n
	}
	// Default nickname style: meaningful English game-like name.
	// Pattern examples: BraveFalcon27, MysticRanger73
	prefixes := []string{
		"Delta", "Echo", "Foxtrot", "Ghost", "Shadow", "Viper", "Raven", "Falcon",
		"Night", "Steel", "Silent", "Recon", "Arctic", "Urban", "Strike", "Rapid",
		"Alpha", "Bravo", "Tango", "Nomad", "Saber", "Patriot", "Sentinel", "Aegis",
	}
	themes := []string{
		"Operator", "Sniper", "Raider", "Hunter", "Guardian", "Lancer", "Ranger", "Watcher",
		"Commander", "Vanguard", "Striker", "Breacher", "Scout", "Patrol", "Sentinel", "Taskforce",
		"Squad", "Recon", "Outrider", "Warden", "Aviator", "Navigator", "Spear", "Marauder",
	}
	p := prefixes[mrand.Intn(len(prefixes))]
	t := themes[mrand.Intn(len(themes))]
	suffix := 10 + mrand.Intn(90) // two digits, game-like but readable
	return fmt.Sprintf("%s%s%d", p, t, suffix)
}

func (s *AuthService) issueToken(userID string) (string, error) {
	exp := time.Now().Add(720 * time.Hour).Unix()
	payload := userID + "|" + strconv.FormatInt(exp, 10)
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))
	return "v1." + base64URLEncode([]byte(payload)) + "." + sig, nil
}

// VerifyToken returns user id or error.
func (s *AuthService) VerifyToken(token string) (string, error) {
	traceID := "authv-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	logger.DebugTrace(traceID, logger.TopicAuth, "[verify-token] begin token_len=%d", len(strings.TrimSpace(token)))
	parts := strings.Split(token, ".")
	if len(parts) != 3 || parts[0] != "v1" {
		logger.DebugTrace(traceID, logger.TopicAuth, "[verify-token] invalid token format parts=%d prefix=%s", len(parts), func() string {
			if len(parts) > 0 {
				return parts[0]
			}
			return ""
		}())
		return "", fmt.Errorf("invalid token")
	}
	raw, err := base64URLDecode(parts[1])
	if err != nil {
		logger.DebugTrace(traceID, logger.TopicAuth, "[verify-token] decode payload failed err=%v", err)
		return "", err
	}
	chunk := string(raw)
	i := strings.LastIndexByte(chunk, '|')
	if i <= 0 {
		logger.DebugTrace(traceID, logger.TopicAuth, "[verify-token] payload separator not found")
		return "", fmt.Errorf("invalid token payload")
	}
	userID := chunk[:i]
	expStr := chunk[i+1:]
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil {
		logger.DebugTrace(traceID, logger.TopicAuth, "[verify-token] parse exp failed expRaw=%q err=%v", expStr, err)
		return "", err
	}
	if time.Now().Unix() > exp {
		logger.DebugTrace(traceID, logger.TopicAuth, "[verify-token] token expired user_id=%s exp=%d now=%d", userID, exp, time.Now().Unix())
		return "", fmt.Errorf("token expired")
	}
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(chunk))
	if hex.EncodeToString(mac.Sum(nil)) != parts[2] {
		logger.DebugTrace(traceID, logger.TopicAuth, "[verify-token] bad signature user_id=%s", userID)
		return "", fmt.Errorf("invalid token signature")
	}
	candidates := buildTokenLookupAccountIDs(userID)
	logger.DebugTrace(traceID, logger.TopicAuth, "[verify-token] db lookup candidates=%v", candidates)
	for _, accountID := range candidates {
		rec, e := s.playerRepo.GetByAccountID(context.Background(), accountID)
		if e == nil && rec != nil {
			if rec.Status == accountStatusBanned {
				logger.DebugTrace(traceID, logger.TopicAuth, "[verify-token] banned account_id=%s", accountID)
				return "", &AccountBannedError{Reason: strings.TrimSpace(rec.BanReason)}
			}
			logger.DebugTrace(traceID, logger.TopicAuth, "[verify-token] db lookup hit account_id=%s", accountID)
			return accountID, nil
		}
		if e != nil && !store.IsNotFound(e) {
			logger.DebugTrace(traceID, logger.TopicAuth, "[verify-token] db lookup error account_id=%s err=%v", accountID, e)
			return "", fmt.Errorf("token verify db lookup failed: %v", e)
		}
		logger.DebugTrace(traceID, logger.TopicAuth, "[verify-token] db lookup miss account_id=%s", accountID)
	}
	logger.DebugTrace(traceID, logger.TopicAuth, "[verify-token] resolved failed final user_id=%s", userID)
	return "", fmt.Errorf("user no longer exists")
}

func buildTokenLookupAccountIDs(userID string) []string {
	id := strings.TrimSpace(userID)
	if id == "" {
		return nil
	}
	out := make([]string, 0, 3)
	seen := make(map[string]struct{}, 3)
	add := func(v string) {
		v = strings.TrimSpace(v)
		if v == "" {
			return
		}
		if _, ok := seen[v]; ok {
			return
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	add(id)
	kind, key, err := normAccountID(id)
	if err == nil && kind != "" && key != "" {
		add(kind + "_" + key)
	}
	// Legacy token compatibility: plain email/phone used as user id in old builds.
	if strings.Contains(id, "@") {
		add("email_" + strings.ToLower(strings.TrimSpace(id)))
	}
	if isLikelyPhoneAccountValue(id) {
		add("phone_" + normPhone(id))
	}
	return out
}

func isLikelyPhoneAccountValue(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	if strings.HasPrefix(s, "+") {
		s = s[1:]
	}
	if len(s) < 6 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func (s *AuthService) GetInviterInfoByAccountID(accountID string) (directAccountID string, directNickname string, secondAccountID string, secondNickname string, err error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return "", "", "", "", fmt.Errorf("account id is empty")
	}
	return s.playerRepo.GetInviterInfoByAccountID(context.Background(), accountID)
}

func (s *AuthService) GetInviteCodeByAccountID(accountID string) (string, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return "", fmt.Errorf("account id is empty")
	}
	return s.playerRepo.GetInviteCodeByAccountID(context.Background(), accountID)
}

// EnsureInviteCodeForAccount 保证 auth_invite_codes 中存在该 account_id 的非空邀请码（读或分配）；用于登录/注册回包兜底。
func (s *AuthService) EnsureInviteCodeForAccount(ctx context.Context, accountID string) (string, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return "", fmt.Errorf("account id is empty")
	}
	code, err := s.playerRepo.GetInviteCodeByAccountID(ctx, accountID)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(code) != "" {
		return strings.TrimSpace(code), nil
	}
	return s.playerRepo.AllocateInviteCodeOnRegister(ctx, accountID)
}

// TryLoadAuthUserRecord 供 HTTP 层在写登录/注册回包时与 /auth/me 同源拼装用户块；库里无此账号时返回 nil, nil。
func (s *AuthService) TryLoadAuthUserRecord(ctx context.Context, accountID string) (*store.AuthUserRecord, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return nil, nil
	}
	rec, e := s.playerRepo.GetByAccountID(ctx, accountID)
	if e != nil {
		if store.IsNotFound(e) {
			return nil, nil
		}
		return nil, e
	}
	return rec, nil
}

func (s *AuthService) ListInviteMembersByAccountID(accountID string) ([]store.InviteMemberRecord, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return []store.InviteMemberRecord{}, fmt.Errorf("account id is empty")
	}
	return s.playerRepo.ListInviteMembersByAccountID(context.Background(), accountID)
}

// AttachEconomy wires v7.1 gold ledger and welfare rank sync (call once after NewAuthService).
func (s *AuthService) AttachEconomy(ledger *GoldLedgerService, rank *WelfareRankSync, redis GoldRedisStore) {
	s.goldLedger = ledger
	s.welfareRank = rank
	s.goldRedis = redis
}

// AttachTaskInviteHook records valid daily invite for welfare tasks.
func (s *AuthService) AttachTaskInviteHook(fn func(ctx context.Context, inviterAccountID string)) {
	s.taskInviteHook = fn
}

func (s *AuthService) notifyTaskInvite(ctx context.Context, inviterAccountID string) {
	if s.taskInviteHook != nil && strings.TrimSpace(inviterAccountID) != "" {
		s.taskInviteHook(ctx, inviterAccountID)
	}
}

// PlayerRepository exposes the auth persistence layer for economy bootstrap.
func (s *AuthService) PlayerRepository() store.PlayerRepository {
	return s.playerRepo
}

// ExchangeGoldForToken 已废弃（v7.1）；金币→Token 仅月末批处理。
func (s *AuthService) ExchangeGoldForToken(accountID string) (tokenDelta, curGold, curToken, totalGold, totalToken float64, err error) {
	return 0, 0, 0, 0, 0, fmt.Errorf("welfare exchange deprecated: use monthly settlement and redeem-token-gift")
}

// RedeemTokenForGift 兑换 curtoken 为礼品（§4）。
func (s *AuthService) RedeemTokenForGift(accountID string) (redeemAmount float64, after store.EconomyBalances, err error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return 0, store.EconomyBalances{}, fmt.Errorf("account id is empty")
	}
	ctx := context.Background()
	if s.goldRedis != nil {
		ok, le := s.goldRedis.TryRedeemGiftLock(ctx, accountID)
		if le != nil {
			return 0, store.EconomyBalances{}, le
		}
		if !ok {
			return 0, store.EconomyBalances{}, fmt.Errorf("redeem in progress, please retry")
		}
	}
	redeemAmount, after, err = s.playerRepo.RedeemTokenForGift(ctx, accountID)
	if err != nil {
		return 0, store.EconomyBalances{}, err
	}
	if s.welfareRank != nil {
		s.welfareRank.Notify(ctx, accountID, WelfareChangedCurToken|WelfareChangedTotalToken, after)
	}
	return redeemAmount, after, nil
}

func (s *AuthService) GetAccountMetricsByAccountID(accountID string) (curGold, totalGold, curToken, totalToken float64, err error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return 0, 0, 0, 0, fmt.Errorf("account id is empty")
	}
	rec, e := s.playerRepo.GetByAccountID(context.Background(), accountID)
	if e != nil {
		if store.IsNotFound(e) {
			return 0, 0, 0, 0, nil
		}
		return 0, 0, 0, 0, e
	}
	if rec == nil {
		return 0, 0, 0, 0, nil
	}
	return rec.CurGold, rec.TotalGold, rec.CurToken, rec.TotalToken, nil
}

// GmPatchAccountMetrics updates persisted metrics; curgold changes go through GoldLedger Set when attached.
func (s *AuthService) GmPatchAccountMetrics(accountID string, curGold, totalGold, curToken, totalToken, curDir, totalDir, curSec, totalSec float64) (*store.AuthUserRecord, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return nil, fmt.Errorf("account id is empty")
	}
	ctx := context.Background()
	if s.goldLedger != nil {
		bal, err := s.playerRepo.GetEconomyBalances(ctx, accountID)
		if err != nil && !store.IsNotFound(err) {
			return nil, err
		}
		if err == nil && bal.CurGold != curGold {
			if _, err := s.goldLedger.ApplyGold(ctx, accountID, GoldOpSet, curGold, GoldApplyOpts{BizType: "gm", SkipDailyCap: true}); err != nil {
				logger.Warn(logger.TopicAuth, "[gm/metrics] ledger set failed account=%s err=%v; fallback to direct metrics patch", accountID, err)
			}
		}
	}
	if err := s.playerRepo.UpdateAccountMetricsByAccountID(ctx, accountID, curGold, totalGold, curToken, totalToken, curDir, totalDir, curSec, totalSec); err != nil {
		return nil, err
	}
	rec, err := s.playerRepo.GetByAccountID(context.Background(), accountID)
	if err != nil {
		return nil, err
	}
	if s.welfareRank != nil {
		ch := WelfareChangedCurGold | WelfareChangedTotalGold | WelfareChangedCurToken | WelfareChangedTotalToken
		s.welfareRank.Notify(ctx, accountID, ch, store.EconomyBalances{
			CurGold:    rec.CurGold,
			TotalGold:  rec.TotalGold,
			CurToken:   rec.CurToken,
			TotalToken: rec.TotalToken,
		})
	}
	return rec, nil
}

// GmFastForwardDeviceSilentByHours moves the latest account's silent-until backward by given hours on the same device.
// This is used for testing silent timeout flows.
func (s *AuthService) GmFastForwardDeviceSilentByHours(deviceID string, hours int) (*store.AuthUserRecord, *time.Time, *time.Time, error) {
	deviceID = truncateDeviceID(deviceID)
	if deviceID == "" {
		return nil, nil, nil, fmt.Errorf("device id is empty")
	}
	if hours <= 0 {
		hours = 6
	}
	ctx := context.Background()
	rec, err := s.playerRepo.FindLatestAccountByDeviceID(ctx, deviceID)
	if err != nil {
		return nil, nil, nil, err
	}
	if rec == nil {
		return nil, nil, nil, store.ErrNotFound
	}
	if rec.DeviceSilentUntil == nil {
		return rec, nil, nil, nil
	}
	before := rec.DeviceSilentUntil.UTC()
	after := before.Add(-time.Duration(hours) * time.Hour)
	s.mu.Lock()
	s.gmSilentRestoreByAccount[rec.AccountID] = before
	s.mu.Unlock()
	if err := s.playerRepo.SetDeviceSilentUntilByAccountID(ctx, rec.AccountID, after); err != nil {
		return nil, nil, nil, err
	}
	updated, err := s.playerRepo.GetByAccountID(ctx, rec.AccountID)
	if err != nil {
		return nil, nil, nil, err
	}
	return updated, &before, &after, nil
}

func (s *AuthService) GmRestoreDeviceSilent(deviceID string) (*store.AuthUserRecord, *time.Time, error) {
	deviceID = truncateDeviceID(deviceID)
	if deviceID == "" {
		return nil, nil, fmt.Errorf("device id is empty")
	}
	ctx := context.Background()
	rec, err := s.playerRepo.FindLatestAccountByDeviceID(ctx, deviceID)
	if err != nil {
		return nil, nil, err
	}
	if rec == nil {
		return nil, nil, store.ErrNotFound
	}
	s.mu.Lock()
	restoreAt, ok := s.gmSilentRestoreByAccount[rec.AccountID]
	s.mu.Unlock()
	if !ok {
		return rec, nil, nil
	}
	if err := s.playerRepo.SetDeviceSilentUntilByAccountID(ctx, rec.AccountID, restoreAt); err != nil {
		return nil, nil, err
	}
	updated, err := s.playerRepo.GetByAccountID(ctx, rec.AccountID)
	if err != nil {
		return nil, nil, err
	}
	s.mu.Lock()
	delete(s.gmSilentRestoreByAccount, rec.AccountID)
	s.mu.Unlock()
	t := restoreAt.UTC()
	return updated, &t, nil
}

func parseIntEnv(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func parseFloatEnv(key string, def float64) float64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return f
}

func truncateDeviceID(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	const max = 191
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	return string(r[:max])
}

func (s *AuthService) computeAdRewardsDisabledNewRegistration(ctx context.Context, deviceID, registrationIP string) (bool, error) {
	deviceID = truncateDeviceID(deviceID)
	ip := strings.TrimSpace(registrationIP)
	devLim := parseIntEnv("REG_ACCOUNTS_PER_DEVICE_PER_DAY", 2)
	ipLim := parseIntEnv("REG_ACCOUNTS_PER_IP_PER_DAY", 5)
	hasDev := deviceID != ""
	evalIP := ip != "" && !httpx.IsLoopback(ip)
	var devCnt, ipCnt int
	var err error
	if hasDev && devLim > 0 {
		devCnt, err = s.playerRepo.CountAccountsRegisteredTodayByDeviceID(ctx, deviceID)
		if err != nil {
			return false, err
		}
	}
	if evalIP && ipLim > 0 {
		ipCnt, err = s.playerRepo.CountAccountsRegisteredTodayByRegistrationIP(ctx, ip)
		if err != nil {
			return false, err
		}
	}
	return regpolicy.RegistrationShouldDisableAdRewards(devCnt, ipCnt, devLim, ipLim, hasDev, evalIP), nil
}

// DecideAdRewardsDisabledForSignup 供集成测试 / 运维脚本：按当前库统计判断「下一笔」注册是否会被打上不发广告奖励标记。
func (s *AuthService) DecideAdRewardsDisabledForSignup(ctx context.Context, deviceID, registrationIP string) (bool, error) {
	return s.computeAdRewardsDisabledNewRegistration(ctx, deviceID, registrationIP)
}

// ResolveUserForMe 按 MySQL auth_accounts 解析（与 VerifyToken / buildTokenLookupAccountIDs 对齐）。
func (s *AuthService) ResolveUserForMe(tokenSubject string) *PublicUser {
	ctx := context.Background()
	for _, aid := range buildTokenLookupAccountIDs(tokenSubject) {
		rec, e := s.playerRepo.GetByAccountID(ctx, aid)
		if e != nil || rec == nil {
			continue
		}
		inviteCode, _ := s.GetInviteCodeByAccountID(rec.AccountID)
		d1, n1, d2, n2, _ := s.GetInviterInfoByAccountID(rec.AccountID)
		return &PublicUser{
			ID:                rec.UserID,
			Email:             rec.Email,
			Phone:             rec.Phone,
			DisplayName:       rec.DisplayName,
			Provider:          rec.Provider,
			InviteCode:        strings.TrimSpace(inviteCode),
			CurGold:           rec.CurGold,
			TotalGold:         rec.TotalGold,
			CurToken:          rec.CurToken,
			TotalToken:        rec.TotalToken,
			InvitedUserID:     strings.TrimSpace(d1),
			InvitedUserID2:    strings.TrimSpace(d2),
			InvitedUserName:   strings.TrimSpace(n1),
			InvitedUserName2:  strings.TrimSpace(n2),
			AdRewardsDisabled: rec.AdRewardsDisabled != 0,
		}
	}
	return nil
}

// AdsRewardedStart 创建单次观看会话（watchId），需在过期前调用 complete 核销。
func (s *AuthService) AdsRewardedStart(accountID, slot, clientIP string) (watchID string, todayCount int, totalCount int, expiresInSec int, err error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return "", 0, 0, 0, fmt.Errorf("account id is empty")
	}
	if e := s.adsGate.CheckSlotValid(slot); e != nil {
		return "", 0, 0, 0, e
	}
	if e := s.adsGate.AllowStart(accountID, clientIP); e != nil {
		return "", 0, 0, 0, e
	}
	ctx := context.Background()
	if recAd, er := s.playerRepo.GetByAccountID(ctx, accountID); er != nil && !store.IsNotFound(er) {
		return "", 0, 0, 0, er
	} else if recAd != nil && recAd.AdRewardsDisabled != 0 {
		return "", 0, 0, 0, store.ErrAdRewardsDisabledAccount
	}
	dailyCap := parseIntEnv("AD_DAILY_COMPLETION_CAP", 0)
	if dailyCap > 0 {
		done, e := s.playerRepo.CountAdCompletionsToday(ctx, accountID)
		if e != nil {
			return "", 0, 0, 0, e
		}
		if done >= dailyCap {
			return "", 0, 0, 0, store.ErrAdDailyCapExceeded
		}
	}
	maxPen := parseIntEnv("AD_MAX_PENDING_WATCHES_ACCOUNT", 3)
	if maxPen > 0 {
		pend, e := s.playerRepo.CountPendingAdWatchSessions(ctx, accountID)
		if e != nil {
			return "", 0, 0, 0, e
		}
		if pend >= maxPen {
			return "", 0, 0, 0, antifraud.ErrTooManyPendingSessions
		}
	}
	expiresInSec = parseIntEnv("AD_WATCH_TTL_SEC", 600)
	if expiresInSec < 60 {
		expiresInSec = 60
	}
	ttl := time.Duration(expiresInSec) * time.Second
	wid, today, tot, e := s.playerRepo.CreateAdWatchSession(ctx, accountID, slot, ttl)
	if e != nil {
		return "", 0, 0, 0, e
	}
	return wid, today, tot, expiresInSec, nil
}

// AdsRewardedComplete 核销 watchId，仅发放金币（v7.1：广告不奖励 Token）。
func (s *AuthService) AdsRewardedComplete(accountID, watchID, slot, clientIP string) (todayCount int, totalCount int, curGold, totalGold, curToken, totalToken, grantedGold, dailyCapRemaining float64, err error) {
	accountID = strings.TrimSpace(accountID)
	watchID = strings.TrimSpace(watchID)
	if accountID == "" || watchID == "" {
		return 0, 0, 0, 0, 0, 0, 0, 0, fmt.Errorf("account id or watch id is empty")
	}
	if e := s.adsGate.AllowComplete(accountID, clientIP); e != nil {
		return 0, 0, 0, 0, 0, 0, 0, 0, e
	}
	ctx := context.Background()
	if recAd, er := s.playerRepo.GetByAccountID(ctx, accountID); er != nil && !store.IsNotFound(er) {
		return 0, 0, 0, 0, 0, 0, 0, 0, er
	} else if recAd != nil && recAd.AdRewardsDisabled != 0 {
		return 0, 0, 0, 0, 0, 0, 0, 0, store.ErrAdRewardsDisabledAccount
	}
	rg := parseFloatEnv("AD_REWARD_GOLD", 1)
	minWatch := parseIntEnv("AD_MIN_WATCH_SEC", 18)
	if minWatch < 0 {
		minWatch = 0
	}
	dailyCap := parseIntEnv("AD_DAILY_COMPLETION_CAP", 0)
	todayCount, totalCount, _, _, _, _, err = s.playerRepo.CompleteAdWatchAndGrant(ctx, accountID, watchID, slot, rg, 0, minWatch, dailyCap)
	if err != nil {
		return 0, 0, 0, 0, 0, 0, 0, 0, err
	}
	if s.goldLedger != nil && rg > 0 {
		res, ae := s.goldLedger.ApplyGold(ctx, accountID, GoldOpAdd, rg, GoldApplyOpts{BizType: "ad", BizNo: watchID})
		if ae != nil && !errors.Is(ae, ErrGoldDailyCapExceeded) {
			return todayCount, totalCount, 0, 0, 0, 0, 0, 0, ae
		}
		grantedGold = res.GrantedDelta
		dailyCapRemaining = res.DailyCapRemaining
	}
	bal, err := s.playerRepo.GetEconomyBalances(ctx, accountID)
	if err != nil {
		return todayCount, totalCount, 0, 0, 0, 0, grantedGold, dailyCapRemaining, err
	}
	return todayCount, totalCount, bal.CurGold, bal.TotalGold, bal.CurToken, bal.TotalToken, grantedGold, dailyCapRemaining, nil
}

func base64URLEncode(b []byte) string {
	// std encoding raw url without padding
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	// use hex for simplicity in sample (payload small)
	return hex.EncodeToString(b)
}

func base64URLDecode(s string) ([]byte, error) {
	return hex.DecodeString(s)
}

// --- Google tokeninfo ---

type googleTokenInfo struct {
	Sub      string `json:"sub"`
	Email    string `json:"email"`
	EmailVrf string `json:"email_verified"`
	Name     string `json:"name"`
	Aud      string `json:"aud"`
	Iss      string `json:"iss"`
	Exp      string `json:"exp"`
}

func VerifyGoogleIDToken(idToken string) (sub, email, name string, err error) {
	if idToken == "" {
		return "", "", "", fmt.Errorf("empty id_token")
	}
	url := "https://oauth2.googleapis.com/tokeninfo?id_token=" + strings.TrimSpace(idToken)
	resp, e := http.Get(url)
	if e != nil {
		return "", "", "", e
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", "", "", fmt.Errorf("tokeninfo: %s: %s", resp.Status, string(b))
	}
	var info googleTokenInfo
	if err = json.Unmarshal(b, &info); err != nil {
		return "", "", "", err
	}
	if info.Sub == "" {
		return "", "", "", fmt.Errorf("invalid id_token: no sub")
	}
	return info.Sub, info.Email, info.Name, nil
}

// --- Facebook graph ---

type fbMe struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// 使用盐 + 多轮 SHA256（无外部依赖，生产请换 bcrypt/argon2）。
func hashPasswordPBKDF2Style(email, password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	h := derivePasswordHash(salt, email, password)
	return base64.StdEncoding.EncodeToString(salt) + "$" + base64.StdEncoding.EncodeToString(h), nil
}

func derivePasswordHash(salt []byte, email, password string) []byte {
	x := append(append(append([]byte{}, salt...), []byte(strings.ToLower(email))...), []byte(password)...)
	out := sha256.Sum256(x)
	for i := 0; i < 80000; i++ {
		out = sha256.Sum256(out[:])
	}
	return out[:]
}

func checkPasswordHash(stored, email, password string) bool {
	parts := strings.Split(stored, "$")
	if len(parts) != 2 {
		return false
	}
	salt, err1 := base64.StdEncoding.DecodeString(parts[0])
	want, err2 := base64.StdEncoding.DecodeString(parts[1])
	if err1 != nil || err2 != nil || len(want) != 32 {
		return false
	}
	got := derivePasswordHash(salt, email, password)
	return subtle.ConstantTimeCompare(got, want) == 1
}

func VerifyFacebookAccessToken(accessToken string) (id, email, name string, err error) {
	if accessToken == "" {
		return "", "", "", fmt.Errorf("empty access_token")
	}
	url := "https://graph.facebook.com/me?fields=id,name,email&access_token=" + strings.TrimSpace(accessToken)
	resp, e := http.Get(url)
	if e != nil {
		return "", "", "", e
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", "", "", fmt.Errorf("graph: %s: %s", resp.Status, string(b))
	}
	var me fbMe
	if err = json.Unmarshal(b, &me); err != nil {
		return "", "", "", err
	}
	if me.ID == "" {
		return "", "", "", fmt.Errorf("invalid facebook response")
	}
	return me.ID, me.Email, me.Name, nil
}

// DeleteAccountSelf 玩家自助销号：从 DB 物理删除 auth_accounts（及 CASCADE 子表与邀请人侧 invite_members）。
func (s *AuthService) DeleteAccountSelf(accountID string) error {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return fmt.Errorf("empty account id")
	}
	ctx := context.Background()
	if err := s.playerRepo.DeleteAuthAccountCascade(ctx, accountID); err != nil {
		if store.IsNotFound(err) {
			return fmt.Errorf("account not found")
		}
		return err
	}
	return nil
}
