package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"starcrystal/server/internal/logger"
	providertypes "starcrystal/server/internal/provider"
	"starcrystal/server/internal/store"
)

func newGuestKey() (string, error) {
	buf := make([]byte, 18)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func isGuestUnverifiedAccount(rec *store.AuthUserRecord) bool {
	if rec == nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(rec.AccountType), providertypes.Guest)
}

func stableAccountID(rec *store.AuthUserRecord) string {
	if rec == nil {
		return ""
	}
	if id := strings.TrimSpace(rec.AccountID); id != "" {
		return id
	}
	return strings.TrimSpace(rec.UserID)
}

func guestKeyFromRecord(rec *store.AuthUserRecord) string {
	if rec == nil {
		return ""
	}
	if v := strings.TrimSpace(rec.AccountValue); v != "" {
		return strings.ToLower(v)
	}
	id := strings.TrimSpace(rec.AccountID)
	if strings.HasPrefix(strings.ToLower(id), "guest_") {
		return strings.ToLower(strings.TrimSpace(id[len("guest_"):]))
	}
	return ""
}

// tryClearGuestDeviceSilentForRegister clears guest device_silent_until when cancelSilent is used on POST /auth/guest.
// Possession: matching guest_key (request key vs account_value), or deviceAuthorized when the row was chosen by deviceId.
func (s *AuthService) tryClearGuestDeviceSilentForRegister(ctx context.Context, rec *store.AuthUserRecord, requestGuestKey string, deviceAuthorized bool) error {
	if rec == nil {
		return fmt.Errorf("guest silent cancel: missing record")
	}
	if rec.DeviceSilentUntil == nil {
		return nil
	}
	if !time.Now().Before(rec.DeviceSilentUntil.UTC()) {
		return nil
	}
	if !strings.EqualFold(strings.TrimSpace(rec.AccountType), providertypes.Guest) {
		return fmt.Errorf("guest silent cancel: not a guest account")
	}
	reqKey := strings.ToLower(strings.TrimSpace(requestGuestKey))
	accKey := strings.ToLower(strings.TrimSpace(rec.AccountValue))
	keyMatch := reqKey != "" && accKey == reqKey
	if !keyMatch && !deviceAuthorized {
		return fmt.Errorf("guest silent cancel: not authorized")
	}
	return s.playerRepo.ClearDeviceSilentByAccountID(ctx, stableAccountID(rec))
}

func (s *AuthService) clearExpiredSilentIfNeeded(ctx context.Context, rec *store.AuthUserRecord) error {
	if rec == nil || rec.DeviceSilentUntil == nil {
		return nil
	}
	until := rec.DeviceSilentUntil.UTC()
	if time.Now().Before(until) {
		return nil
	}
	if strings.TrimSpace(rec.AccountID) != "" {
		if err := s.playerRepo.ClearDeviceSilentByAccountID(ctx, rec.AccountID); err != nil {
			return err
		}
	}
	rec.DeviceSilentUntil = nil
	return nil
}

func (s *AuthService) buildGuestSessionFromRecord(ctx context.Context, rec *store.AuthUserRecord, fallbackKey string) (*userRecord, string, string, error) {
	if rec == nil {
		return nil, "", "", fmt.Errorf("guest record is nil")
	}
	key := strings.TrimSpace(guestKeyFromRecord(rec))
	if key == "" {
		key = strings.ToLower(strings.TrimSpace(fallbackKey))
	}
	if key == "" {
		key = strings.ToLower(strings.TrimSpace(rec.AccountID))
	}
	u := &userRecord{
		ID:                strings.TrimSpace(rec.UserID),
		Email:             strings.TrimSpace(rec.Email),
		Phone:             strings.TrimSpace(rec.Phone),
		Provider:          strings.TrimSpace(rec.Provider),
		DisplayName:       strings.TrimSpace(rec.DisplayName),
		InvitedUserID:     strings.TrimSpace(rec.InvitedUserID),
		CurGold:           rec.CurGold,
		TotalGold:         rec.TotalGold,
		CurToken:          rec.CurToken,
		TotalToken:        rec.TotalToken,
		AdRewardsDisabled: rec.AdRewardsDisabled != 0,
	}
	if strings.TrimSpace(u.ID) == "" {
		u.ID = strings.TrimSpace(rec.AccountID)
	}
	if u.Provider == "" {
		u.Provider = providertypes.Guest
	}
	if u.DisplayName == "" {
		u.DisplayName = ensureDisplayName("Guest")
	}
	lookupAID := stableAccountID(rec)
	if lookupAID == "" {
		return nil, "", "", fmt.Errorf("guest record missing account id")
	}
	inviteCode, ice := s.playerRepo.GetInviteCodeByAccountID(ctx, lookupAID)
	if ice != nil {
		return nil, "", "", fmt.Errorf("get invite code for guest session failed: %w", ice)
	}
	u.InviteCode = strings.TrimSpace(inviteCode)
	if u.InviteCode == "" {
		inviteCode, ice = s.playerRepo.AllocateInviteCodeOnRegister(ctx, lookupAID)
		if ice != nil {
			return nil, "", "", fmt.Errorf("allocate invite code failed: %w", ice)
		}
		u.InviteCode = strings.TrimSpace(inviteCode)
	}
	directAccountID, directNickname, secondAccountID, secondNickname, ie := s.playerRepo.GetInviterInfoByAccountID(ctx, lookupAID)
	if ie == nil {
		u.InvitedUserID = strings.TrimSpace(directAccountID)
		u.InvitedUserID2 = strings.TrimSpace(secondAccountID)
		u.InvitedUserName = strings.TrimSpace(directNickname)
		u.InvitedUserName2 = strings.TrimSpace(secondNickname)
	}
	tok, te := s.issueToken(lookupAID)
	return u, tok, key, te
}

func (s *AuthService) bindInviteRelationForGuestIfNeeded(ctx context.Context, traceID string, accountID string, inviteCode string) error {
	inviteToken := strings.TrimSpace(inviteCode)
	if inviteToken == "" {
		return nil
	}
	inviterAccountID, err := s.playerRepo.ResolveAccountIDByInviteCode(ctx, inviteToken)
	if err != nil {
		logger.Warn(logger.TopicAuth, "[register-guest] resolve invite token failed on guest auto-login trace=%s token=%q err=%v", traceID, inviteToken, err)
		return fmt.Errorf("resolve inviteCode failed: %v", err)
	}
	inviterAccountID = strings.TrimSpace(inviterAccountID)
	if inviterAccountID == "" {
		logger.Warn(logger.TopicAuth, "[register-guest] invite code not found on guest auto-login trace=%s code=%q", traceID, inviteToken)
		return fmt.Errorf("invalid inviteCode")
	}
	if ive := s.ensureInviterActiveForRegistration(ctx, inviterAccountID); ive != nil {
		logger.Warn(logger.TopicAuth, "[register-guest] inviter not allowed on guest auto-login trace=%s token=%q inviter=%q err=%v", traceID, inviteToken, inviterAccountID, ive)
		return ive
	}
	if err := s.playerRepo.AddInviteMember(ctx, inviterAccountID, strings.TrimSpace(accountID)); err != nil {
		logger.Error(logger.TopicAuth, "[register-guest] add invite member on guest auto-login failed trace=%s inviter=%q account_id=%q err=%v", traceID, inviterAccountID, accountID, err)
		return fmt.Errorf("db insert invite member failed: %v", err)
	}
	logger.Info(logger.TopicAuth, "[register-guest] invite member linked on guest auto-login trace=%s inviter=%q account_id=%q", traceID, inviterAccountID, accountID)
	return nil
}

// RegisterGuest creates a new guest account and applies the same register-time
// invite/device checks as normal account registration.
func (s *AuthService) RegisterGuest(guestKey, inviteCode, deviceID, clientIP string, confirmDeactivateOldAccount bool, cancelSilent bool) (*userRecord, string, string, error) {
	traceID := "guest-reg-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	logger.DebugTrace(traceID, logger.TopicAuth,
		"[register-guest] begin guest_key_len=%d invited_user_id=%q device_id=%q confirm_deactivate=%t cancel_silent=%t",
		len(strings.TrimSpace(guestKey)), strings.TrimSpace(inviteCode), strings.TrimSpace(deviceID), confirmDeactivateOldAccount, cancelSilent)
	key := strings.ToLower(strings.TrimSpace(guestKey))
	if key == "" {
		var err error
		key, err = newGuestKey()
		if err != nil {
			logger.Error(logger.TopicAuth, "[register-guest] generate guest key failed trace=%s err=%v", traceID, err)
			return nil, "", "", err
		}
		logger.DebugTrace(traceID, logger.TopicAuth, "[register-guest] generated guest key")
	}
	accountID := "guest_" + key
	logger.DebugTrace(traceID, logger.TopicAuth, "[register-guest] normalized account_id=%q", accountID)
	ctx := context.Background()
	rec, ge := s.playerRepo.GetByAccountID(ctx, accountID)
	if ge == nil && rec != nil {
		logger.DebugTrace(traceID, logger.TopicAuth, "[register-guest] account already exists account_id=%q", accountID)
		if !isGuestUnverifiedAccount(rec) {
			logger.Warn(logger.TopicAuth, "[register-guest] existing guest key already upgraded trace=%s account_id=%q account_type=%q provider=%q", traceID, accountID, rec.AccountType, rec.Provider)
			return nil, "", "", fmt.Errorf("guest account already upgraded")
		}
		if ce := s.clearExpiredSilentIfNeeded(ctx, rec); ce != nil {
			logger.Error(logger.TopicAuth, "[register-guest] clear expired silent failed trace=%s account_id=%q err=%v", traceID, accountID, ce)
			return nil, "", "", ce
		}
		if se := s.validateAccountCanLogin(rec); se != nil {
			var silent *AccountSilentError
			if cancelSilent && errors.As(se, &silent) {
				if ce := s.tryClearGuestDeviceSilentForRegister(ctx, rec, key, false); ce != nil {
					logger.Error(logger.TopicAuth, "[register-guest] cancel silent failed trace=%s account_id=%q err=%v", traceID, accountID, ce)
					return nil, "", "", ce
				}
				rec, ge = s.playerRepo.GetByAccountID(ctx, accountID)
				if ge != nil {
					logger.Error(logger.TopicAuth, "[register-guest] reload after cancel silent failed trace=%s account_id=%q err=%v", traceID, accountID, ge)
					return nil, "", "", fmt.Errorf("db guest reload failed: %v", ge)
				}
				if se = s.validateAccountCanLogin(rec); se != nil {
					logger.Warn(logger.TopicAuth, "[register-guest] existing account still blocked trace=%s account_id=%q err=%v", traceID, accountID, se)
					return nil, "", "", se
				}
				logger.Info(logger.TopicAuth, "[register-guest] silent cleared by cancel_silent trace=%s account_id=%q", traceID, accountID)
			} else {
				logger.Warn(logger.TopicAuth, "[register-guest] existing account blocked trace=%s account_id=%q err=%v", traceID, accountID, se)
				return nil, "", "", se
			}
		}
		u, tok, outKey, err := s.buildGuestSessionFromRecord(ctx, rec, key)
		if err != nil {
			logger.Error(logger.TopicAuth, "[register-guest] existing guest login failed trace=%s account_id=%q err=%v", traceID, accountID, err)
			return nil, "", "", err
		}
		if be := s.bindInviteRelationForGuestIfNeeded(ctx, traceID, rec.AccountID, inviteCode); be != nil {
			return nil, "", "", be
		}
		logger.Info(logger.TopicAuth, "[register-guest] existing guest auto-login by guest_key trace=%s account_id=%q", traceID, accountID)
		return u, tok, outKey, nil
	}
	if ge != nil && !store.IsNotFound(ge) {
		logger.Error(logger.TopicAuth, "[register-guest] lookup failed trace=%s account_id=%q err=%v", traceID, accountID, ge)
		return nil, "", "", fmt.Errorf("db guest register lookup failed: %v", ge)
	}
	normalizedDeviceID := truncateDeviceID(deviceID)
	if normalizedDeviceID != "" {
		guestByDevice, de := s.playerRepo.FindOldestActiveGuestAccountByDeviceID(ctx, normalizedDeviceID)
		if de != nil {
			logger.Error(logger.TopicAuth, "[register-guest] oldest guest by device lookup failed trace=%s device_id=%q err=%v", traceID, normalizedDeviceID, de)
			return nil, "", "", de
		}
		if guestByDevice != nil {
			if ce := s.clearExpiredSilentIfNeeded(ctx, guestByDevice); ce != nil {
				logger.Error(logger.TopicAuth, "[register-guest] clear expired silent failed trace=%s account_id=%q err=%v", traceID, stableAccountID(guestByDevice), ce)
				return nil, "", "", ce
			}
			if se := s.validateAccountCanLogin(guestByDevice); se != nil {
				var silent *AccountSilentError
				if cancelSilent && errors.As(se, &silent) {
					devAuth := normalizedDeviceID != "" && truncateDeviceID(strings.TrimSpace(guestByDevice.DeviceID)) == normalizedDeviceID
					if ce := s.tryClearGuestDeviceSilentForRegister(ctx, guestByDevice, key, devAuth); ce != nil {
						logger.Error(logger.TopicAuth, "[register-guest] cancel silent (device guest) failed trace=%s account_id=%q err=%v", traceID, stableAccountID(guestByDevice), ce)
						return nil, "", "", ce
					}
					gid := stableAccountID(guestByDevice)
					guestByDevice, de = s.playerRepo.GetByAccountID(ctx, gid)
					if de != nil {
						logger.Error(logger.TopicAuth, "[register-guest] reload device guest after cancel silent failed trace=%s account_id=%q err=%v", traceID, gid, de)
						return nil, "", "", fmt.Errorf("db guest reload failed: %v", de)
					}
					if se = s.validateAccountCanLogin(guestByDevice); se != nil {
						logger.Warn(logger.TopicAuth, "[register-guest] device guest still blocked trace=%s account_id=%q err=%v", traceID, gid, se)
						return nil, "", "", se
					}
					logger.Info(logger.TopicAuth, "[register-guest] silent cleared by cancel_silent (device guest) trace=%s account_id=%q", traceID, gid)
				} else {
					logger.Warn(logger.TopicAuth, "[register-guest] device guest account blocked trace=%s account_id=%q err=%v", traceID, stableAccountID(guestByDevice), se)
					return nil, "", "", se
				}
			}
			u, tok, outKey, err := s.buildGuestSessionFromRecord(ctx, guestByDevice, guestKeyFromRecord(guestByDevice))
			if err != nil {
				logger.Error(logger.TopicAuth, "[register-guest] device guest auto-login failed trace=%s account_id=%q err=%v", traceID, stableAccountID(guestByDevice), err)
				return nil, "", "", err
			}
			if be := s.bindInviteRelationForGuestIfNeeded(ctx, traceID, stableAccountID(guestByDevice), inviteCode); be != nil {
				return nil, "", "", be
			}
			logger.Info(logger.TopicAuth, "[register-guest] existing guest auto-login by device trace=%s account_id=%q device_id=%q", traceID, stableAccountID(guestByDevice), normalizedDeviceID)
			return u, tok, outKey, nil
		}
	}

	u := &userRecord{
		ID:          accountID,
		Provider:    providertypes.Guest,
		DisplayName: ensureDisplayName("Guest"),
	}
	newRec := &store.AuthUserRecord{
		UserID:       accountID,
		AccountID:    accountID,
		AccountType:  providertypes.Guest,
		AccountValue: key,
		PasswordHash: "guest_nologin",
		Provider:     providertypes.Guest,
		DisplayName:  u.DisplayName,
	}
	inviteToken := strings.TrimSpace(inviteCode)
	if inviteToken != "" {
		logger.DebugTrace(traceID, logger.TopicAuth, "[register-guest] resolving invite code len=%d", len(inviteToken))
		inviterAccountID, ie := s.playerRepo.ResolveAccountIDByInviteCode(ctx, inviteToken)
		if ie != nil {
			logger.Warn(logger.TopicAuth, "[register-guest] resolve invite code failed trace=%s code=%q err=%v", traceID, inviteToken, ie)
			return nil, "", "", fmt.Errorf("resolve inviteCode failed: %v", ie)
		}
		if strings.TrimSpace(inviterAccountID) == "" {
			logger.Warn(logger.TopicAuth, "[register-guest] invite code not found trace=%s code=%q", traceID, inviteToken)
			return nil, "", "", fmt.Errorf("invalid inviteCode")
		}
		if ive := s.ensureInviterActiveForRegistration(ctx, inviterAccountID); ive != nil {
			logger.Warn(logger.TopicAuth, "[register-guest] inviter not allowed trace=%s token=%q inviter=%q err=%v", traceID, inviteToken, strings.TrimSpace(inviterAccountID), ive)
			return nil, "", "", ive
		}
		newRec.InvitedUserID = strings.TrimSpace(inviterAccountID)
		logger.DebugTrace(traceID, logger.TopicAuth, "[register-guest] invite resolved inviter_account_id=%q", newRec.InvitedUserID)
	}
	disabled, de := s.computeAdRewardsDisabledNewRegistration(ctx, deviceID, clientIP)
	if de != nil {
		logger.Error(logger.TopicAuth, "[register-guest] compute ad policy failed trace=%s err=%v", traceID, de)
		return nil, "", "", de
	}
	newRec.DeviceID = normalizedDeviceID
	newRec.RegistrationIP = strings.TrimSpace(clientIP)
	logger.DebugTrace(traceID, logger.TopicAuth, "[register-guest] applying device policy account_id=%q device_id=%q", accountID, newRec.DeviceID)
	if de := s.enforceDevicePolicyOnRegister(ctx, accountID, newRec.DeviceID, confirmDeactivateOldAccount); de != nil {
		logger.Warn(logger.TopicAuth, "[register-guest] device policy blocked trace=%s account_id=%q device_id=%q err=%v", traceID, accountID, newRec.DeviceID, de)
		return nil, "", "", de
	}
	if disabled {
		newRec.AdRewardsDisabled = 1
	}
	if ce := s.playerRepo.CreateByAccountID(ctx, newRec); ce != nil {
		logger.Error(logger.TopicAuth, "[register-guest] create account failed trace=%s account_id=%q err=%v", traceID, accountID, ce)
		return nil, "", "", fmt.Errorf("db guest register create failed: %v", ce)
	}
	if ume := s.playerRepo.UpsertDeviceAccountMap(ctx, newRec.DeviceID, accountID); ume != nil {
		_ = s.playerRepo.DeleteAuthAccountCascade(ctx, accountID)
		logger.Error(logger.TopicAuth, "[register-guest] upsert device map failed trace=%s account_id=%q device_id=%q err=%v", traceID, accountID, newRec.DeviceID, ume)
		return nil, "", "", fmt.Errorf("db upsert device map failed: %v", ume)
	}
	logger.DebugTrace(traceID, logger.TopicAuth, "[register-guest] account created and device map upserted account_id=%q", accountID)
	u.AdRewardsDisabled = disabled
	if newRec.InvitedUserID != "" {
		if me := s.playerRepo.AddInviteMember(ctx, newRec.InvitedUserID, accountID); me != nil {
			logger.Error(logger.TopicAuth, "[register-guest] add invite member failed trace=%s inviter=%q account_id=%q err=%v", traceID, newRec.InvitedUserID, accountID, me)
			return nil, "", "", fmt.Errorf("db insert invite member failed: %v", me)
		}
		logger.DebugTrace(traceID, logger.TopicAuth, "[register-guest] invite member linked inviter=%q account_id=%q", newRec.InvitedUserID, accountID)
	}
	if err := s.assignInviteCodeForNewAccount(ctx, accountID, u); err != nil {
		logger.Error(logger.TopicAuth, "[register-guest] assign invite code failed trace=%s account_id=%q err=%v", traceID, accountID, err)
		return nil, "", "", err
	}
	logger.DebugTrace(traceID, logger.TopicAuth, "[register-guest] invite code assigned account_id=%q invite_code=%q", accountID, strings.TrimSpace(u.InviteCode))
	directAccountID, directNickname, secondAccountID, secondNickname, ie := s.playerRepo.GetInviterInfoByAccountID(ctx, accountID)
	if ie == nil {
		u.InvitedUserID = strings.TrimSpace(directAccountID)
		u.InvitedUserID2 = strings.TrimSpace(secondAccountID)
		u.InvitedUserName = strings.TrimSpace(directNickname)
		u.InvitedUserName2 = strings.TrimSpace(secondNickname)
		logger.DebugTrace(traceID, logger.TopicAuth, "[register-guest] inviter chain loaded account_id=%q direct=%q second=%q", accountID, u.InvitedUserID, u.InvitedUserID2)
	}
	tok, te := s.issueToken(accountID)
	if te != nil {
		logger.Error(logger.TopicAuth, "[register-guest] issue token failed trace=%s account_id=%q err=%v", traceID, accountID, te)
		return nil, "", "", te
	}
	logger.Info(logger.TopicAuth, "[register-guest] success trace=%s account_id=%q guest_key_len=%d device_id=%q", traceID, accountID, len(key), newRec.DeviceID)
	return u, tok, key, te
}

// GuestLogin 创建或恢复访客账号（持久化于 MySQL）。
func (s *AuthService) GuestLogin(guestKey, _, _ string) (*userRecord, string, string, error) {
	return s.guestLoginWithStore(guestKey)
}

func (s *AuthService) guestLoginWithStore(guestKey string) (*userRecord, string, string, error) {
	key := strings.ToLower(strings.TrimSpace(guestKey))
	if key == "" {
		var err error
		key, err = newGuestKey()
		if err != nil {
			return nil, "", "", err
		}
	}
	accountID := "guest_" + key
	ctx := context.Background()
	rec, e := s.playerRepo.GetByAccountID(ctx, accountID)
	if e != nil && !store.IsNotFound(e) {
		return nil, "", "", fmt.Errorf("db guest lookup failed: %v", e)
	}
	if rec == nil || store.IsNotFound(e) {
		newRec := &store.AuthUserRecord{
			UserID:       accountID,
			AccountID:    accountID,
			AccountType:  providertypes.Guest,
			AccountValue: key,
			PasswordHash: "guest_nologin",
			Provider:     providertypes.Guest,
			DisplayName:  ensureDisplayName("Guest"),
		}
		if ce := s.playerRepo.CreateByAccountID(ctx, newRec); ce != nil {
			return nil, "", "", fmt.Errorf("db guest create failed: %v", ce)
		}
		rec, e = s.playerRepo.GetByAccountID(ctx, accountID)
		if e != nil {
			return nil, "", "", fmt.Errorf("db guest reload failed: %v", e)
		}
	}
	if ce := s.clearExpiredSilentIfNeeded(ctx, rec); ce != nil {
		return nil, "", "", ce
	}
	if se := s.validateAccountCanLogin(rec); se != nil {
		return nil, "", "", se
	}
	u := &userRecord{
		ID:          rec.UserID,
		Provider:    rec.Provider,
		DisplayName: rec.DisplayName,
		CurGold:     rec.CurGold,
		TotalGold:   rec.TotalGold,
		CurToken:    rec.CurToken,
		TotalToken:  rec.TotalToken,
	}
	if inviteCode, ice := s.playerRepo.GetInviteCodeByAccountID(ctx, accountID); ice == nil {
		u.InviteCode = strings.TrimSpace(inviteCode)
	}
	if strings.TrimSpace(u.InviteCode) == "" {
		if ae := s.assignInviteCodeForNewAccount(ctx, accountID, u); ae != nil {
			return nil, "", "", ae
		}
	}
	tok, te := s.issueToken(accountID)
	return u, tok, key, te
}

func (s *AuthService) GuestVerify(accountID, account, code string) (*userRecord, string, error) {
	traceID := "guest-verify-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return nil, "", fmt.Errorf("missing account id")
	}
	kind, key, err := normAccountID(strings.TrimSpace(account))
	if err != nil {
		return nil, "", err
	}
	if kind != providertypes.Email && kind != providertypes.Phone {
		return nil, "", fmt.Errorf("guest verify account must be email or phone")
	}
	verifyCode := strings.TrimSpace(code)
	if verifyCode == "" {
		return nil, "", fmt.Errorf("verification code is required")
	}

	s.mu.Lock()
	st, ok := s.smsState[key]
	if !ok || st.Code == "" || time.Now().After(st.Expire) {
		s.mu.Unlock()
		return nil, "", fmt.Errorf("verification code expired, send again")
	}
	if st.Code != verifyCode {
		s.mu.Unlock()
		return nil, "", fmt.Errorf("wrong verification code")
	}
	st.Code = ""
	s.mu.Unlock()

	ctx := context.Background()
	rec, ge := s.playerRepo.GetByAccountID(ctx, accountID)
	if ge != nil {
		if store.IsNotFound(ge) {
			return nil, "", fmt.Errorf("guest account not found")
		}
		return nil, "", fmt.Errorf("db load guest account failed: %v", ge)
	}
	if !isGuestUnverifiedAccount(rec) {
		return nil, "", fmt.Errorf("current account is not guest")
	}
	email := ""
	phone := ""
	if kind == providertypes.Email {
		email = key
	} else {
		phone = key
	}
	if ue := s.playerRepo.UpdateGuestVerifiedContactByAccountID(ctx, accountID, kind, key, email, phone, kind); ue != nil {
		if store.IsNotFound(ue) {
			return nil, "", fmt.Errorf("guest account not found")
		}
		return nil, "", fmt.Errorf("db update guest verify failed: %v", ue)
	}
	updated, ge2 := s.playerRepo.GetByAccountID(ctx, accountID)
	if ge2 != nil {
		if store.IsNotFound(ge2) {
			return nil, "", fmt.Errorf("guest account not found")
		}
		return nil, "", fmt.Errorf("db reload guest verify failed: %v", ge2)
	}
	u := &userRecord{
		ID:                strings.TrimSpace(updated.UserID),
		Email:             strings.TrimSpace(updated.Email),
		Phone:             strings.TrimSpace(updated.Phone),
		Provider:          strings.TrimSpace(updated.Provider),
		DisplayName:       strings.TrimSpace(updated.DisplayName),
		CurGold:           updated.CurGold,
		TotalGold:         updated.TotalGold,
		CurToken:          updated.CurToken,
		TotalToken:        updated.TotalToken,
		InvitedUserID:     strings.TrimSpace(updated.InvitedUserID),
		AdRewardsDisabled: updated.AdRewardsDisabled != 0,
	}
	if u.ID == "" {
		u.ID = accountID
	}
	if inviteCode, ice := s.playerRepo.GetInviteCodeByAccountID(ctx, accountID); ice == nil {
		u.InviteCode = strings.TrimSpace(inviteCode)
	}
	if d1, n1, d2, n2, ie := s.playerRepo.GetInviterInfoByAccountID(ctx, accountID); ie == nil {
		u.InvitedUserID = strings.TrimSpace(d1)
		u.InvitedUserName = strings.TrimSpace(n1)
		u.InvitedUserID2 = strings.TrimSpace(d2)
		u.InvitedUserName2 = strings.TrimSpace(n2)
	}
	tok, te := s.issueToken(accountID)
	if te != nil {
		logger.Error(logger.TopicAuth, "[guest-verify] issue token failed trace=%s account_id=%q err=%v", traceID, accountID, te)
		return nil, "", te
	}
	return u, tok, nil
}
