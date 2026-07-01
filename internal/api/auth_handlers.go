package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"starcrystal/server/internal/errx"
	"starcrystal/server/internal/httpx"
	"starcrystal/server/internal/logger"
	providertypes "starcrystal/server/internal/provider"
	"starcrystal/server/internal/service"
	"starcrystal/server/internal/store"
)

// mapAuthLoginFailure 将登录相关错误映射为明确业务码（避免「账号还是密码错了」含糊不清）。
// legacyCode：未能分类时回退（如历史接口 1402/1412/1414）。
func mapAuthLoginFailure(err error, legacyCode int) (httpStatus int, businessCode int, message string) {
	var banned *service.AccountBannedError
	if errors.As(err, &banned) {
		return http.StatusForbidden, 1422, banned.Error()
	}
	var silent *service.AccountSilentError
	if errors.As(err, &silent) {
		return http.StatusForbidden, 1421, silent.Error()
	}
	switch {
	case errors.Is(err, errx.ErrAccountNotFound):
		return http.StatusUnauthorized, 1415, errx.ErrAccountNotFound.Error()
	case errors.Is(err, errx.ErrWrongPassword):
		return http.StatusUnauthorized, 1416, errx.ErrWrongPassword.Error()
	case errors.Is(err, errx.ErrPasswordLoginUnavailable):
		return http.StatusUnauthorized, 1417, errx.ErrPasswordLoginUnavailable.Error()
	case errors.Is(err, errx.ErrInvalidEmailFormat), errors.Is(err, errx.ErrInvalidPhoneFormat):
		return http.StatusBadRequest, 1400, err.Error()
	default:
		if strings.Contains(err.Error(), "db login failed") {
			return http.StatusInternalServerError, 1500, "登录服务暂时不可用，请稍后重试"
		}
		return http.StatusUnauthorized, legacyCode, err.Error()
	}
}

func (s *Server) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeJSON(w, http.StatusMethodNotAllowed, Response{Code: 1405, Message: "method not allowed"})
		return
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	tok := strings.TrimPrefix(auth, "Bearer ")
	if tok == "" {
		tok = strings.TrimSpace(r.URL.Query().Get("accessToken"))
	}
	if tok == "" {
		s.writeJSON(w, http.StatusUnauthorized, Response{Code: 1406, Message: "missing bearer token"})
		return
	}
	userID, err := s.authService.VerifyToken(tok)
	if err != nil {
		s.writeJSON(w, http.StatusUnauthorized, Response{Code: 1407, Message: err.Error()})
		return
	}
	pu := s.authService.ResolveUserForMe(userID)
	if pu == nil {
		s.writeJSON(w, http.StatusUnauthorized, Response{Code: 1408, Message: "user not found"})
		return
	}
	fallback := buildAuthUserFieldsWithNames(pu.ID, pu.Email, pu.Phone, pu.DisplayName, pu.Provider, pu.InviteCode, pu.CurGold, pu.TotalGold, pu.CurToken, pu.TotalToken, pu.InvitedUserID, pu.InvitedUserID2, pu.InvitedUserName, pu.InvitedUserName2)
	fallback.AdRewardsDisabled = pu.AdRewardsDisabled
	aux := s.authUserFromSessionResponse(userID, fallback)
	if s.economy != nil && s.economy.InviteNotify != nil && s.economy.InviteNotify.Enabled() {
		if rec, err := s.authService.TryLoadAuthUserRecord(r.Context(), userID); err == nil && rec != nil {
			pending := s.economy.InviteNotify.ComputePending(rec)
			aux.PendingDownlineL1Contrib = pending.PendingL1
			aux.PendingDownlineL2Contrib = pending.PendingL2
		}
	}
	s.writeJSON(w, http.StatusOK, Response{
		Code:    0,
		Message: "ok",
		Data:    aux,
	})
}

type oauthReq struct {
	Provider    string `json:"provider"`    // "google" | "facebook"
	IDToken     string `json:"idToken"`     // Google
	AccessToken string `json:"accessToken"` // Facebook
	DisplayName string `json:"displayName"`
}

type authData struct {
	AccessToken string   `json:"accessToken"`
	ExpiresIn   int64    `json:"expiresIn"`
	User        authUser `json:"user"`
	GuestKey    string   `json:"guestKey,omitempty"`
}

type authUser struct {
	UserID                   string  `json:"userId"`
	AccountID                string  `json:"accountId"`
	Email                    string  `json:"email"`
	Phone                    string  `json:"phone"`
	DisplayName              string  `json:"displayName"`
	Provider                 string  `json:"provider"`
	InviteCode               string  `json:"inviteCode"`
	CurGold                  float64 `json:"curgold,omitempty"`
	TotalGold                float64 `json:"totalgold,omitempty"`
	CurToken                 float64 `json:"curtoken,omitempty"`
	TotalToken               float64 `json:"totaltoken,omitempty"`
	CurDirectInviterShare    float64 `json:"cur_direct_inviter_share,omitempty"`
	TotalDirectInviterShare  float64 `json:"total_direct_inviter_share,omitempty"`
	CurSecondInviterShare    float64 `json:"cur_second_inviter_share,omitempty"`
	TotalSecondInviterShare  float64 `json:"total_second_inviter_share,omitempty"`
	CurDownlineL1Contrib     float64 `json:"curDownlineL1Contrib,omitempty"`
	TotalDownlineL1Contrib   float64 `json:"totalDownlineL1Contrib,omitempty"`
	CurDownlineL2Contrib     float64 `json:"curDownlineL2Contrib,omitempty"`
	TotalDownlineL2Contrib   float64 `json:"totalDownlineL2Contrib,omitempty"`
	PendingDownlineL1Contrib float64 `json:"pendingDownlineL1Contrib,omitempty"`
	PendingDownlineL2Contrib float64 `json:"pendingDownlineL2Contrib,omitempty"`
	InvitedUserID            string  `json:"inviteduserid,omitempty"`
	InvitedUserID2           string  `json:"inviteduserid2,omitempty"`
	InvitedUserName          string  `json:"invitedusername,omitempty"`
	InvitedUserName2         string  `json:"invitedusername2,omitempty"`
	AdRewardsDisabled        bool    `json:"adRewardsDisabled,omitempty"`
}

func buildAuthUserFields(userID, email, phone, displayName, provider, inviteCode string, curGold, totalGold, curToken, totalToken float64, invitedUserID, invitedUserID2 string) authUser {
	return authUser{
		UserID:         userID,
		AccountID:      userID,
		Email:          email,
		Phone:          phone,
		DisplayName:    displayName,
		Provider:       provider,
		InviteCode:     inviteCode,
		CurGold:        curGold,
		TotalGold:      totalGold,
		CurToken:       curToken,
		TotalToken:     totalToken,
		InvitedUserID:  invitedUserID,
		InvitedUserID2: invitedUserID2,
	}
}

func buildAuthUserFieldsWithNames(userID, email, phone, displayName, provider, inviteCode string, curGold, totalGold, curToken, totalToken float64, invitedUserID, invitedUserID2, invitedUserName, invitedUserName2 string) authUser {
	u := buildAuthUserFields(userID, email, phone, displayName, provider, inviteCode, curGold, totalGold, curToken, totalToken, invitedUserID, invitedUserID2)
	u.InvitedUserName = invitedUserName
	u.InvitedUserName2 = invitedUserName2
	return u
}

func (s *Server) authUserFromAccountRecord(accountID string, rec *store.AuthUserRecord) authUser {
	if rec == nil {
		return authUser{}
	}
	lookupID := strings.TrimSpace(accountID)
	if lookupID == "" {
		lookupID = strings.TrimSpace(rec.AccountID)
		if lookupID == "" {
			lookupID = strings.TrimSpace(rec.UserID)
		}
	}
	inviteCode, _ := s.authService.GetInviteCodeByAccountID(lookupID)
	if strings.TrimSpace(inviteCode) == "" {
		for _, alt := range []string{strings.TrimSpace(rec.AccountID), strings.TrimSpace(rec.UserID)} {
			if alt != "" && alt != lookupID {
				if ic, _ := s.authService.GetInviteCodeByAccountID(alt); strings.TrimSpace(ic) != "" {
					inviteCode = ic
					lookupID = alt
					break
				}
			}
		}
	}
	d1, n1, d2, n2, _ := s.authService.GetInviterInfoByAccountID(lookupID)
	u := buildAuthUserFieldsWithNames(rec.UserID, rec.Email, rec.Phone, rec.DisplayName, rec.Provider, inviteCode,
		rec.CurGold, rec.TotalGold, rec.CurToken, rec.TotalToken, d1, d2, n1, n2)
	u.CurDirectInviterShare = rec.CurDirectInviterShare
	u.TotalDirectInviterShare = rec.TotalDirectInviterShare
	u.CurSecondInviterShare = rec.CurSecondInviterShare
	u.TotalSecondInviterShare = rec.TotalSecondInviterShare
	u.CurDownlineL1Contrib = rec.CurDownlineL1Contrib
	u.TotalDownlineL1Contrib = rec.TotalDownlineL1Contrib
	u.CurDownlineL2Contrib = rec.CurDownlineL2Contrib
	u.TotalDownlineL2Contrib = rec.TotalDownlineL2Contrib
	u.AdRewardsDisabled = rec.AdRewardsDisabled != 0
	return u
}

// authUserFromSessionResponse 配置了 MySQL 时按 account_id 读 auth_accounts 并由 authUserFromAccountRecord 组装
// （邀请码仅从 auth_invite_codes 读取、上级昵称、分成字段等与 /auth/me、gm/metrics 一致）；否则使用 fallback。
func (s *Server) authUserFromSessionResponse(accountID string, fallback authUser) authUser {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return fallback
	}
	ctx := context.Background()
	rec, err := s.authService.TryLoadAuthUserRecord(ctx, accountID)
	if err != nil || rec == nil {
		fb := fallback
		if strings.TrimSpace(fb.InviteCode) == "" {
			if code, e2 := s.authService.EnsureInviteCodeForAccount(ctx, accountID); e2 == nil && strings.TrimSpace(code) != "" {
				fb.InviteCode = strings.TrimSpace(code)
			}
		}
		return fb
	}
	out := s.authUserFromAccountRecord(accountID, rec)
	// 读本行后若联系人列为空（偶发读写时序或服务端逻辑遗漏），沿用 handler 传入的 fallback，避免丢失刚校验写入的邮箱/手机。
	if out.Email == "" && strings.TrimSpace(fallback.Email) != "" {
		out.Email = fallback.Email
	}
	if out.Phone == "" && strings.TrimSpace(fallback.Phone) != "" {
		out.Phone = fallback.Phone
	}
	if strings.TrimSpace(out.Provider) == "" && strings.TrimSpace(fallback.Provider) != "" {
		out.Provider = fallback.Provider
	}
	if strings.TrimSpace(out.AccountID) == "" && strings.TrimSpace(fallback.AccountID) != "" {
		out.AccountID = fallback.AccountID
	} else if strings.TrimSpace(out.AccountID) == "" {
		out.AccountID = strings.TrimSpace(out.UserID)
	}
	if strings.TrimSpace(out.InviteCode) == "" && strings.TrimSpace(fallback.InviteCode) != "" {
		out.InviteCode = strings.TrimSpace(fallback.InviteCode)
	}
	if strings.TrimSpace(out.InviteCode) == "" {
		if code, e2 := s.authService.EnsureInviteCodeForAccount(ctx, accountID); e2 == nil && strings.TrimSpace(code) != "" {
			out.InviteCode = strings.TrimSpace(code)
		}
	}
	return out
}

type gmMetricsReq struct {
	CurGold                 float64 `json:"curgold"`
	TotalGold               float64 `json:"totalgold"`
	CurToken                float64 `json:"curtoken"`
	TotalToken              float64 `json:"totaltoken"`
	CurDirectInviterShare   float64 `json:"cur_direct_inviter_share"`
	TotalDirectInviterShare float64 `json:"total_direct_inviter_share"`
	CurSecondInviterShare   float64 `json:"cur_second_inviter_share"`
	TotalSecondInviterShare float64 `json:"total_second_inviter_share"`
}

type gmFastForwardSilentReq struct {
	DeviceID string `json:"deviceId"`
	Hours    int    `json:"hours"`
}

type gmFastForwardSilentData struct {
	AccountID         string `json:"accountId"`
	BeforeSilentUntil string `json:"beforeSilentUntil,omitempty"`
	AfterSilentUntil  string `json:"afterSilentUntil,omitempty"`
	Applied           bool   `json:"applied"`
}

type gmRestoreSilentData struct {
	AccountID     string `json:"accountId"`
	RestoredUntil string `json:"restoredUntil,omitempty"`
	Restored      bool   `json:"restored"`
}

func (s *Server) handleAuthGmMetrics(w http.ResponseWriter, r *http.Request) {
	traceID := "gm-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	logger.DebugTrace(traceID, logger.TopicAPI, "[gm/metrics] enter method=%s path=%s", r.Method, r.URL.Path)
	if r.Method != http.MethodPost {
		logger.DebugTrace(traceID, logger.TopicAPI, "[gm/metrics] reject method=%s", r.Method)
		s.writeJSON(w, http.StatusMethodNotAllowed, Response{Code: 1405, Message: "method not allowed"})
		return
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	tok := strings.TrimPrefix(auth, "Bearer ")
	if tok == "" {
		tok = strings.TrimSpace(r.URL.Query().Get("accessToken"))
	}
	logger.DebugTrace(traceID, logger.TopicAPI, "[gm/metrics] auth extracted token_len=%d source=%s", len(tok), func() string {
		if strings.TrimSpace(r.Header.Get("Authorization")) != "" {
			return "header"
		}
		if strings.TrimSpace(r.URL.Query().Get("accessToken")) != "" {
			return "query"
		}
		return "none"
	}())
	if tok == "" {
		logger.DebugTrace(traceID, logger.TopicAPI, "[gm/metrics] auth missing token")
		s.writeJSON(w, http.StatusUnauthorized, Response{Code: 1406, Message: "missing bearer token"})
		return
	}
	accountID, err := s.authService.VerifyToken(tok)
	if err != nil {
		logger.DebugTrace(traceID, logger.TopicAPI, "[gm/metrics] verify token failed err=%v", err)
		s.writeJSON(w, http.StatusUnauthorized, Response{Code: 1407, Message: err.Error()})
		return
	}
	logger.DebugTrace(traceID, logger.TopicAPI, "[gm/metrics] verify token ok account_id=%s", accountID)
	var body gmMetricsReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		logger.DebugTrace(traceID, logger.TopicAPI, "[gm/metrics] decode body failed err=%v", err)
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "invalid json: " + err.Error()})
		return
	}
	logger.DebugTrace(traceID, logger.TopicAPI, "[gm/metrics] decoded payload curgold=%.2f totalgold=%.2f curtoken=%.2f totaltoken=%.2f cur_direct=%.2f total_direct=%.2f cur_second=%.2f total_second=%.2f",
		body.CurGold, body.TotalGold, body.CurToken, body.TotalToken, body.CurDirectInviterShare, body.TotalDirectInviterShare, body.CurSecondInviterShare, body.TotalSecondInviterShare)
	rec, err := s.authService.GmPatchAccountMetrics(accountID, body.CurGold, body.TotalGold, body.CurToken, body.TotalToken,
		body.CurDirectInviterShare, body.TotalDirectInviterShare, body.CurSecondInviterShare, body.TotalSecondInviterShare)
	if err != nil {
		logger.DebugTrace(traceID, logger.TopicAPI, "[gm/metrics] patch failed err=%v", err)
		if store.IsNotFound(err) {
			s.writeJSON(w, http.StatusNotFound, Response{Code: 1418, Message: "account not found"})
			return
		}
		msg := err.Error()
		if strings.Contains(msg, "database not configured") || strings.Contains(msg, "mysql dsn is empty") {
			s.writeJSON(w, http.StatusServiceUnavailable, Response{Code: 1501, Message: msg})
			return
		}
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1414, Message: msg})
		return
	}
	logger.DebugTrace(traceID, logger.TopicAPI, "[gm/metrics] patch success db account_id=%s curgold=%.2f totalgold=%.2f curtoken=%.2f totaltoken=%.2f",
		rec.AccountID, rec.CurGold, rec.TotalGold, rec.CurToken, rec.TotalToken)
	user := s.authUserFromAccountRecord(accountID, rec)
	logger.DebugTrace(traceID, logger.TopicAPI, "[gm/metrics] response user_id=%s invited1=%s invited2=%s inviteCode_len=%d",
		user.UserID, user.InvitedUserID, user.InvitedUserID2, len(user.InviteCode))
	s.writeJSON(w, http.StatusOK, Response{Code: 0, Message: "ok", Data: user})
}

func (s *Server) handleAuthGmFastForwardSilent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, http.StatusMethodNotAllowed, Response{Code: 1405, Message: "method not allowed"})
		return
	}
	var body gmFastForwardSilentReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "invalid json: " + err.Error()})
		return
	}
	rec, before, after, err := s.authService.GmFastForwardDeviceSilentByHours(body.DeviceID, body.Hours)
	if err != nil {
		if store.IsNotFound(err) {
			s.writeJSON(w, http.StatusNotFound, Response{Code: 1418, Message: "account not found"})
			return
		}
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1414, Message: err.Error()})
		return
	}
	data := gmFastForwardSilentData{
		AccountID: strings.TrimSpace(rec.AccountID),
		Applied:   before != nil && after != nil,
	}
	if before != nil {
		data.BeforeSilentUntil = before.UTC().Format(time.RFC3339)
	}
	if after != nil {
		data.AfterSilentUntil = after.UTC().Format(time.RFC3339)
	}
	s.writeJSON(w, http.StatusOK, Response{Code: 0, Message: "ok", Data: data})
}

func (s *Server) handleAuthGmRestoreSilent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, http.StatusMethodNotAllowed, Response{Code: 1405, Message: "method not allowed"})
		return
	}
	var body gmFastForwardSilentReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "invalid json: " + err.Error()})
		return
	}
	rec, restored, err := s.authService.GmRestoreDeviceSilent(body.DeviceID)
	if err != nil {
		if store.IsNotFound(err) {
			s.writeJSON(w, http.StatusNotFound, Response{Code: 1418, Message: "account not found"})
			return
		}
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1414, Message: err.Error()})
		return
	}
	data := gmRestoreSilentData{
		AccountID: strings.TrimSpace(rec.AccountID),
		Restored:  restored != nil,
	}
	if restored != nil {
		data.RestoredUntil = restored.UTC().Format(time.RFC3339)
	}
	s.writeJSON(w, http.StatusOK, Response{Code: 0, Message: "ok", Data: data})
}

func (s *Server) handleAuthOAuth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, http.StatusMethodNotAllowed, Response{Code: 1405, Message: "method not allowed"})
		return
	}
	var body oauthReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "invalid json: " + err.Error()})
		return
	}
	p := strings.ToLower(strings.TrimSpace(body.Provider))
	var sub, email, name string
	var err error
	switch p {
	case providertypes.Google:
		sub, email, name, err = service.VerifyGoogleIDToken(body.IDToken)
		if err != nil {
			s.writeJSON(w, http.StatusUnauthorized, Response{Code: 1403, Message: "google: " + err.Error()})
			return
		}
	case providertypes.Facebook:
		sub, email, name, err = service.VerifyFacebookAccessToken(body.AccessToken)
		if err != nil {
			s.writeJSON(w, http.StatusUnauthorized, Response{Code: 1404, Message: "facebook: " + err.Error()})
			return
		}
	default:
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "provider must be google or facebook"})
		return
	}
	if body.DisplayName != "" {
		name = body.DisplayName
	}
	u, tok, err := s.authService.LoginOrLinkOAuth(p, sub, email, name)
	if err != nil {
		s.writeJSON(w, http.StatusInternalServerError, Response{Code: 2002, Message: err.Error()})
		return
	}
	fb := authUser{UserID: u.ID, AccountID: u.ID, Email: u.Email, Phone: u.Phone, DisplayName: u.DisplayName, Provider: u.Provider, InviteCode: u.InviteCode, CurGold: u.CurGold, TotalGold: u.TotalGold, CurToken: u.CurToken, TotalToken: u.TotalToken, AdRewardsDisabled: u.AdRewardsDisabled}
	s.writeJSON(w, http.StatusOK, Response{Code: 0, Message: "ok", Data: authData{
		AccessToken: tok,
		ExpiresIn:   720 * 3600,
		User:        s.authUserFromSessionResponse(u.ID, fb),
	}})
}

// sendVerificationCodeReq 统一：注册验证码 + 找回密码发码。@json channel: phone | email（可选，与 account 推断不一致时 400）。
// @json purpose: register | password_reset；兼容旧 sms/send（无 purpose 视为 register）与仅含 email（无 account）的旧找回密码请求。
type sendVerificationCodeReq struct {
	Purpose string `json:"purpose"`
	Channel string `json:"channel"`
	Account string `json:"account"`
	Phone   string `json:"phone"`
	Email   string `json:"email"`
}

type registerByCodeReq struct {
	Account                     string `json:"account"`
	Code                        string `json:"code"`
	Password                    string `json:"password"`
	DisplayName                 string `json:"displayName"`
	InviteCode                  string `json:"inviteCode"`
	DeviceID                    string `json:"deviceId,omitempty"`
	ConfirmDeactivateOldAccount bool   `json:"confirmDeactivateOldAccount,omitempty"`
}

type registerConflictData struct {
	ExistingAccountID  string `json:"existingAccountId,omitempty"`
	ExistingUserName   string `json:"existingUserName,omitempty"`
	NeedConfirmation   bool   `json:"needConfirmation,omitempty"`
	RemainingSilentSec int64  `json:"remainingSilentSec,omitempty"`
	SilentSeconds      int64  `json:"silentSeconds,omitempty"`
	SilentHours        int    `json:"silentHours,omitempty"`
}

type loginByAccountReq struct {
	Account      string `json:"account"`
	Password     string `json:"password"`
	CancelSilent bool   `json:"cancelSilent,omitempty"`
}

type loginSilentData struct {
	AccountID          string `json:"accountId,omitempty"`
	RemainingSilentSec int64  `json:"remainingSilentSec,omitempty"`
	CanCancel          bool   `json:"canCancel,omitempty"`
}

type passwordResetConfirmReq struct {
	Account     string `json:"account"`
	Email       string `json:"email"`
	Code        string `json:"code"`
	NewPassword string `json:"newPassword"`
	DeviceID    string `json:"deviceId,omitempty"`
}

type guestReq struct {
	GuestKey                    string `json:"guestKey"`
	InviteCode                  string `json:"inviteCode"`
	DeviceID                    string `json:"deviceId,omitempty"`
	Fingerprint                 string `json:"fingerprint,omitempty"`
	ConfirmDeactivateOldAccount bool   `json:"confirmDeactivateOldAccount,omitempty"`
	CancelSilent                bool   `json:"cancelSilent,omitempty"`
}

type guestVerifyReq struct {
	Account     string `json:"account"`
	Code        string `json:"code"`
	DeviceID    string `json:"deviceId,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
}

type profileNameReq struct {
	AccountID   string `json:"accountId"`
	DisplayName string `json:"displayName"`
}

type sendSmsData struct {
	CooldownSec   int    `json:"cooldownSec"`
	DevVerifyCode string `json:"devVerifyCode,omitempty"`
}

type myTeamData struct {
	InvitedUserID          string           `json:"inviteduserid,omitempty"`
	InvitedUserID2         string           `json:"inviteduserid2,omitempty"`
	InvitedUserIDNickname  string           `json:"inviteduseridNickname,omitempty"`
	InvitedUserID2Nickname string           `json:"inviteduserid2Nickname,omitempty"`
	InviteCode             string           `json:"inviteCode,omitempty"`
	CurGold                float64          `json:"curgold"`
	TotalGold              float64          `json:"totalgold"`
	CurToken               float64          `json:"curtoken"`
	TotalToken             float64          `json:"totaltoken"`
	TeamLevel1TotalMembers int              `json:"teamLevel1TotalMembers"`
	TeamLevel2TotalMembers int              `json:"teamLevel2TotalMembers"`
	TeamLevel1TotalShare   float64          `json:"teamLevel1TotalShare"`
	TeamLevel2TotalShare   float64          `json:"teamLevel2TotalShare"`
	Members                []teamMemberData `json:"members"`
}

type teamMemberData struct {
	AccountID   string  `json:"accountId"`
	Nickname    string  `json:"nickname"`
	DisplayName string  `json:"displayName"`
	Email       string  `json:"email"`
	Phone       string  `json:"phone"`
	Provider    string  `json:"provider"`
	CreatedAt   string  `json:"createdAt"`
	CurGold     float64 `json:"curgold"`
	TotalGold   float64 `json:"totalgold"`
	CurToken    float64 `json:"curtoken"`
	TotalToken  float64 `json:"totaltoken"`
}

func resolveSendVerificationTarget(body *sendVerificationCodeReq) (account string, purpose string) {
	account = strings.TrimSpace(body.Account)
	if account == "" {
		account = strings.TrimSpace(body.Phone)
	}
	purpose = strings.ToLower(strings.TrimSpace(body.Purpose))
	ch := strings.ToLower(strings.TrimSpace(body.Channel))
	emailOnly := strings.TrimSpace(body.Email)
	if purpose == "" && account == "" && emailOnly != "" {
		return emailOnly, "password_reset"
	}
	if purpose == "" {
		purpose = "register"
	}
	if ch != "" {
		body.Channel = ch
	}
	return account, purpose
}

func (s *Server) handleAuthSendVerificationCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, http.StatusMethodNotAllowed, Response{Code: 1405, Message: "method not allowed"})
		return
	}
	var body sendVerificationCodeReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "invalid json: " + err.Error()})
		return
	}
	account, purpose := resolveSendVerificationTarget(&body)
	if account == "" {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1410, Message: "account is required"})
		return
	}
	if purpose != "register" && purpose != "password_reset" && purpose != "guest_verify" {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "purpose must be register, password_reset or guest_verify"})
		return
	}
	if ch := strings.TrimSpace(body.Channel); ch != "" {
		kind, err := service.InferAccountKind(account)
		if err != nil {
			if errors.Is(err, errx.ErrInvalidEmailFormat) || errors.Is(err, errx.ErrInvalidPhoneFormat) {
				s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: err.Error()})
				return
			}
			s.writeJSON(w, http.StatusBadRequest, Response{Code: 1410, Message: err.Error()})
			return
		}
		if ch != kind {
			s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "channel does not match account"})
			return
		}
	}

	var cool int
	var devCode string
	var err error
	switch purpose {
	case "register":
		cool, devCode, err = s.authService.SendVerifyCode(account)
	case "password_reset":
		cool, devCode, err = s.authService.PasswordResetSend(account)
	case "guest_verify":
		cool, devCode, err = s.authService.SendVerifyCode(account)
	}
	if err != nil {
		if errors.Is(err, errx.ErrPasswordResetAccountNotFound) {
			s.writeJSON(w, http.StatusBadRequest, Response{Code: 1415, Message: err.Error()})
			return
		}
		if errors.Is(err, errx.ErrInvalidEmailFormat) || errors.Is(err, errx.ErrInvalidPhoneFormat) {
			s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: err.Error()})
			return
		}
		if strings.HasPrefix(err.Error(), "send too often") {
			s.writeJSON(w, http.StatusTooManyRequests, Response{Code: 1409, Message: err.Error(), Data: sendSmsData{CooldownSec: cool}})
			return
		}
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1410, Message: err.Error()})
		return
	}
	s.writeJSON(w, http.StatusOK, Response{Code: 0, Message: "ok", Data: sendSmsData{CooldownSec: cool, DevVerifyCode: devCode}})
}

func (s *Server) handleAuthPasswordResetConfirm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, http.StatusMethodNotAllowed, Response{Code: 1405, Message: "method not allowed"})
		return
	}
	var body passwordResetConfirmReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "invalid json: " + err.Error()})
		return
	}
	acct := strings.TrimSpace(body.Account)
	if acct == "" {
		acct = body.Email
	}
	u, tok, err := s.authService.PasswordResetConfirm(acct, body.Code, body.NewPassword, body.DeviceID, httpx.ClientIP(r))
	if err != nil {
		var banned *service.AccountBannedError
		if errors.As(err, &banned) {
			s.writeJSON(w, http.StatusForbidden, Response{Code: 1422, Message: banned.Error()})
			return
		}
		var silent *service.AccountSilentError
		if errors.As(err, &silent) {
			s.writeJSON(w, http.StatusForbidden, Response{Code: 1421, Message: silent.Error()})
			return
		}
		if errors.Is(err, errx.ErrPasswordResetAccountNotFound) {
			s.writeJSON(w, http.StatusBadRequest, Response{Code: 1415, Message: err.Error()})
			return
		}
		if errors.Is(err, errx.ErrInvalidEmailFormat) || errors.Is(err, errx.ErrInvalidPhoneFormat) {
			s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: err.Error()})
			return
		}
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1411, Message: err.Error()})
		return
	}
	if u == nil {
		s.writeJSON(w, http.StatusInternalServerError, Response{Code: 1411, Message: "password reset succeeded but user payload is missing"})
		return
	}
	base := withAdFlag(buildAuthUserFieldsWithNames(u.ID, u.Email, u.Phone, u.DisplayName, u.Provider, u.InviteCode, u.CurGold, u.TotalGold, u.CurToken, u.TotalToken, u.InvitedUserID, u.InvitedUserID2, u.InvitedUserName, u.InvitedUserName2), u.AdRewardsDisabled)
	s.writeJSON(w, http.StatusOK, Response{Code: 0, Message: "ok", Data: authData{
		AccessToken: tok,
		ExpiresIn:   720 * 3600,
		User:        s.authUserFromSessionResponse(u.ID, base),
	}})
}

func withAdFlag(u authUser, adOff bool) authUser {
	u.AdRewardsDisabled = adOff
	return u
}

func (s *Server) handleAuthMyTeam(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeJSON(w, http.StatusMethodNotAllowed, Response{Code: 1405, Message: "method not allowed"})
		return
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	tok := strings.TrimPrefix(auth, "Bearer ")
	if tok == "" {
		tok = strings.TrimSpace(r.URL.Query().Get("accessToken"))
	}
	if tok == "" {
		s.writeJSON(w, http.StatusUnauthorized, Response{Code: 1406, Message: "missing bearer token"})
		return
	}
	accountID, err := s.authService.VerifyToken(tok)
	if err != nil {
		s.writeJSON(w, http.StatusUnauthorized, Response{Code: 1407, Message: err.Error()})
		return
	}
	directAccountID, directNickname, secondAccountID, secondNickname, ie := s.authService.GetInviterInfoByAccountID(accountID)
	if ie != nil {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1414, Message: ie.Error()})
		return
	}
	inviteCode, ice := s.authService.GetInviteCodeByAccountID(accountID)
	if ice != nil {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1414, Message: ice.Error()})
		return
	}
	curGold, totalGold, curToken, totalToken, ge := s.authService.GetAccountMetricsByAccountID(accountID)
	if ge != nil {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1414, Message: ge.Error()})
		return
	}
	rawMembers, me := s.authService.ListInviteMembersByAccountID(accountID)
	if me != nil {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1414, Message: me.Error()})
		return
	}
	level1Count := len(rawMembers)
	level2Count := 0
	level1Share := 0.0
	level2Share := 0.0
	members := make([]teamMemberData, 0, len(rawMembers))
	seen := make(map[string]struct{}, len(rawMembers)*2)
	for _, m := range rawMembers {
		level1Share += m.TotalDirectInviterShare
		id := strings.TrimSpace(m.AccountID)
		if id != "" {
			if _, ok := seen[id]; !ok {
				members = append(members, teamMemberData{
					AccountID:   m.AccountID,
					Nickname:    m.Nickname,
					DisplayName: m.DisplayName,
					Email:       m.Email,
					Phone:       m.Phone,
					Provider:    m.Provider,
					CreatedAt:   m.CreatedAt,
					CurGold:     m.CurGold,
					TotalGold:   m.TotalGold,
					CurToken:    m.CurToken,
					TotalToken:  m.TotalToken,
				})
				seen[id] = struct{}{}
			}
		}
		level2Members, le := s.authService.ListInviteMembersByAccountID(m.AccountID)
		if le == nil {
			level2Count += len(level2Members)
			for _, l2 := range level2Members {
				level2Share += l2.TotalSecondInviterShare
				if level1Count < 500 && len(members) < 500 {
					id := strings.TrimSpace(l2.AccountID)
					if id != "" {
						if _, ok := seen[id]; !ok {
							members = append(members, teamMemberData{
								AccountID:   l2.AccountID,
								Nickname:    l2.Nickname,
								DisplayName: l2.DisplayName,
								Email:       l2.Email,
								Phone:       l2.Phone,
								Provider:    l2.Provider,
								CreatedAt:   l2.CreatedAt,
								CurGold:     l2.CurGold,
								TotalGold:   l2.TotalGold,
								CurToken:    l2.CurToken,
								TotalToken:  l2.TotalToken,
							})
							seen[id] = struct{}{}
						}
					}
				}
			}
		}
	}
	s.writeJSON(w, http.StatusOK, Response{
		Code:    0,
		Message: "ok",
		Data: myTeamData{
			InvitedUserID:          strings.TrimSpace(directAccountID),
			InvitedUserID2:         strings.TrimSpace(secondAccountID),
			InvitedUserIDNickname:  strings.TrimSpace(directNickname),
			InvitedUserID2Nickname: strings.TrimSpace(secondNickname),
			InviteCode:             strings.TrimSpace(inviteCode),
			CurGold:                curGold,
			TotalGold:              totalGold,
			CurToken:               curToken,
			TotalToken:             totalToken,
			TeamLevel1TotalMembers: level1Count,
			TeamLevel2TotalMembers: level2Count,
			TeamLevel1TotalShare:   level1Share,
			TeamLevel2TotalShare:   level2Share,
			Members:                members,
		},
	})
}

func (s *Server) handleAuthRegisterByCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, http.StatusMethodNotAllowed, Response{Code: 1405, Message: "method not allowed"})
		return
	}
	var body registerByCodeReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "invalid json: " + err.Error()})
		return
	}
	u, tok, err := s.authService.RegisterByCode(
		body.Account,
		body.Code,
		body.Password,
		body.DisplayName,
		body.InviteCode,
		body.DeviceID,
		httpx.ClientIP(r),
		body.ConfirmDeactivateOldAccount,
	)
	if err != nil {
		var conflict *service.DeviceAccountConflictError
		if errors.As(err, &conflict) {
			s.writeJSON(w, http.StatusConflict, Response{
				Code:    1420,
				Message: conflict.Error(),
				Data: registerConflictData{
					ExistingAccountID:  conflict.ExistingAccountID,
					ExistingUserName:   conflict.ExistingUserName,
					NeedConfirmation:   conflict.NeedConfirmation,
					RemainingSilentSec: conflict.RemainingSec,
					SilentSeconds:      conflict.SilentSeconds,
					SilentHours:        int((conflict.SilentSeconds + 3599) / 3600),
				},
			})
			return
		}
		var silent *service.AccountSilentError
		if errors.As(err, &silent) {
			s.writeJSON(w, http.StatusForbidden, Response{Code: 1421, Message: silent.Error()})
			return
		}
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1413, Message: err.Error()})
		return
	}
	rb := authUser{
		UserID: u.ID, AccountID: u.ID, Email: u.Email, Phone: u.Phone, DisplayName: u.DisplayName, Provider: u.Provider, InviteCode: u.InviteCode,
		CurGold: u.CurGold, TotalGold: u.TotalGold, CurToken: u.CurToken, TotalToken: u.TotalToken,
		InvitedUserID: u.InvitedUserID, InvitedUserID2: u.InvitedUserID2,
		InvitedUserName: u.InvitedUserName, InvitedUserName2: u.InvitedUserName2,
		AdRewardsDisabled: u.AdRewardsDisabled,
	}
	s.writeJSON(w, http.StatusOK, Response{Code: 0, Message: "ok", Data: authData{
		AccessToken: tok,
		ExpiresIn:   720 * 3600,
		User:        s.authUserFromSessionResponse(u.ID, rb),
	}})
}

func (s *Server) handleAuthLoginByAccount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, http.StatusMethodNotAllowed, Response{Code: 1405, Message: "method not allowed"})
		return
	}
	var body loginByAccountReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "invalid json: " + err.Error()})
		return
	}
	u, tok, err := s.authService.LoginByAccount(body.Account, body.Password, body.CancelSilent)
	if err != nil {
		var silent *service.AccountSilentError
		if errors.As(err, &silent) {
			s.writeJSON(w, http.StatusForbidden, Response{
				Code:    1421,
				Message: silent.Error(),
				Data: loginSilentData{
					AccountID:          strings.TrimSpace(silent.AccountID),
					RemainingSilentSec: silent.RemainingSec,
					CanCancel:          silent.CanCancel,
				},
			})
			return
		}
		st, co, msg := mapAuthLoginFailure(err, 1414)
		s.writeJSON(w, st, Response{Code: co, Message: msg})
		return
	}
	lb := withAdFlag(buildAuthUserFieldsWithNames(u.ID, u.Email, u.Phone, u.DisplayName, u.Provider, u.InviteCode, u.CurGold, u.TotalGold, u.CurToken, u.TotalToken, u.InvitedUserID, u.InvitedUserID2, u.InvitedUserName, u.InvitedUserName2), u.AdRewardsDisabled)
	s.writeJSON(w, http.StatusOK, Response{Code: 0, Message: "ok", Data: authData{
		AccessToken: tok,
		ExpiresIn:   720 * 3600,
		User:        s.authUserFromSessionResponse(u.ID, lb),
	}})
}

func (s *Server) handleAuthGuest(w http.ResponseWriter, r *http.Request) {
	traceID := "guest-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	if r.Method != http.MethodPost {
		s.writeJSON(w, http.StatusMethodNotAllowed, Response{Code: 1405, Message: "method not allowed"})
		return
	}
	var body guestReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		logger.Warn(logger.TopicAuth, "[auth/guest] decode request failed trace=%s err=%v", traceID, err)
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "invalid json: " + err.Error()})
		return
	}
	u, tok, gk, err := s.authService.RegisterGuest(
		body.GuestKey,
		body.InviteCode,
		body.DeviceID,
		httpx.ClientIP(r),
		body.ConfirmDeactivateOldAccount,
		body.CancelSilent,
	)
	if err != nil {
		logger.Error(logger.TopicAuth, "[auth/guest] register guest failed trace=%s guest_key_len=%d device_id=%q invite_code=%q err=%v",
			traceID, len(strings.TrimSpace(body.GuestKey)), strings.TrimSpace(body.DeviceID), strings.TrimSpace(body.InviteCode), err)
		var conflict *service.DeviceAccountConflictError
		if errors.As(err, &conflict) {
			s.writeJSON(w, http.StatusConflict, Response{
				Code:    1420,
				Message: conflict.Error(),
				Data: registerConflictData{
					ExistingAccountID:  conflict.ExistingAccountID,
					ExistingUserName:   conflict.ExistingUserName,
					NeedConfirmation:   conflict.NeedConfirmation,
					RemainingSilentSec: conflict.RemainingSec,
					SilentSeconds:      conflict.SilentSeconds,
					SilentHours:        int((conflict.SilentSeconds + 3599) / 3600),
				},
			})
			return
		}
		var silent *service.AccountSilentError
		if errors.As(err, &silent) {
			s.writeJSON(w, http.StatusForbidden, Response{
				Code:    1421,
				Message: silent.Error(),
				Data: loginSilentData{
					AccountID:          strings.TrimSpace(silent.AccountID),
					RemainingSilentSec: silent.RemainingSec,
					CanCancel:          silent.CanCancel,
				},
			})
			return
		}
		if strings.Contains(strings.ToLower(err.Error()), "column 'account_id' in field list is ambiguous") {
			s.writeJSON(w, http.StatusInternalServerError, Response{
				Code:    1411,
				Message: "guest register failed: database query is ambiguous (account_id)",
			})
			return
		}
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1411, Message: err.Error()})
		return
	}
	gb := authUser{
		UserID: u.ID, AccountID: u.ID, Email: u.Email, Phone: u.Phone, DisplayName: u.DisplayName, Provider: u.Provider, InviteCode: u.InviteCode,
		CurGold: u.CurGold, TotalGold: u.TotalGold, CurToken: u.CurToken, TotalToken: u.TotalToken,
		InvitedUserID: u.InvitedUserID, InvitedUserID2: u.InvitedUserID2,
		InvitedUserName: u.InvitedUserName, InvitedUserName2: u.InvitedUserName2,
		AdRewardsDisabled: u.AdRewardsDisabled,
	}
	s.writeJSON(w, http.StatusOK, Response{Code: 0, Message: "ok", Data: authData{
		AccessToken: tok,
		ExpiresIn:   720 * 3600,
		GuestKey:    gk,
		User:        s.authUserFromSessionResponse(u.ID, gb),
	}})
}

func (s *Server) handleAuthGuestVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, http.StatusMethodNotAllowed, Response{Code: 1405, Message: "method not allowed"})
		return
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	tok := strings.TrimPrefix(auth, "Bearer ")
	if tok == "" {
		tok = strings.TrimSpace(r.URL.Query().Get("accessToken"))
	}
	if tok == "" {
		s.writeJSON(w, http.StatusUnauthorized, Response{Code: 1406, Message: "missing bearer token"})
		return
	}
	accountID, err := s.authService.VerifyToken(tok)
	if err != nil {
		s.writeJSON(w, http.StatusUnauthorized, Response{Code: 1407, Message: err.Error()})
		return
	}
	var body guestVerifyReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "invalid json: " + err.Error()})
		return
	}
	u, newToken, err := s.authService.GuestVerify(accountID, body.Account, body.Code)
	if err != nil {
		if errors.Is(err, errx.ErrInvalidEmailFormat) || errors.Is(err, errx.ErrInvalidPhoneFormat) {
			s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: err.Error()})
			return
		}
		msg := strings.ToLower(strings.TrimSpace(err.Error()))
		if strings.Contains(msg, "wrong verification code") || strings.Contains(msg, "verification code expired") {
			s.writeJSON(w, http.StatusBadRequest, Response{Code: 1418, Message: err.Error()})
			return
		}
		if strings.Contains(msg, "not guest") {
			s.writeJSON(w, http.StatusBadRequest, Response{Code: 1423, Message: err.Error()})
			return
		}
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1411, Message: err.Error()})
		return
	}
	base := withAdFlag(buildAuthUserFieldsWithNames(u.ID, u.Email, u.Phone, u.DisplayName, u.Provider, u.InviteCode, u.CurGold, u.TotalGold, u.CurToken, u.TotalToken, u.InvitedUserID, u.InvitedUserID2, u.InvitedUserName, u.InvitedUserName2), u.AdRewardsDisabled)
	s.writeJSON(w, http.StatusOK, Response{Code: 0, Message: "ok", Data: authData{
		AccessToken: newToken,
		ExpiresIn:   720 * 3600,
		User:        s.authUserFromSessionResponse(u.ID, base),
	}})
}

// handleAuthAccountDelete 自助删除：物理删除 auth_accounts（FK CASCADE 清理设备映射、邀请码、广告会话等）。
func (s *Server) handleAuthAccountDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, http.StatusMethodNotAllowed, Response{Code: 1405, Message: "method not allowed"})
		return
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	tok := strings.TrimPrefix(auth, "Bearer ")
	if tok == "" {
		tok = strings.TrimSpace(r.URL.Query().Get("accessToken"))
	}
	if tok == "" {
		s.writeJSON(w, http.StatusUnauthorized, Response{Code: 1406, Message: "missing bearer token"})
		return
	}
	accountID, err := s.authService.VerifyToken(tok)
	if err != nil {
		var banned *service.AccountBannedError
		if errors.As(err, &banned) {
			s.writeJSON(w, http.StatusForbidden, Response{Code: 1422, Message: banned.Error()})
			return
		}
		s.writeJSON(w, http.StatusUnauthorized, Response{Code: 1407, Message: err.Error()})
		return
	}
	if err := s.authService.DeleteAccountSelf(accountID); err != nil {
		if strings.Contains(strings.ToLower(strings.TrimSpace(err.Error())), "not found") {
			s.writeJSON(w, http.StatusBadRequest, Response{Code: 1415, Message: err.Error()})
			return
		}
		s.writeJSON(w, http.StatusInternalServerError, Response{Code: 1411, Message: err.Error()})
		return
	}
	s.writeJSON(w, http.StatusOK, Response{Code: 0, Message: "ok"})
}

func (s *Server) handleAuthProfileName(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, http.StatusMethodNotAllowed, Response{Code: 1405, Message: "method not allowed"})
		return
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	tok := strings.TrimPrefix(auth, "Bearer ")
	if tok == "" {
		tok = strings.TrimSpace(r.URL.Query().Get("accessToken"))
	}
	if tok == "" {
		s.writeJSON(w, http.StatusUnauthorized, Response{Code: 1406, Message: "missing bearer token"})
		return
	}
	subject, err := s.authService.VerifyToken(tok)
	if err != nil {
		s.writeJSON(w, http.StatusUnauthorized, Response{Code: 1407, Message: err.Error()})
		return
	}
	var body profileNameReq
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: "invalid json: " + err.Error()})
		return
	}
	if err := s.authService.UpdateProfileDisplayName(subject, body.AccountID, body.DisplayName); err != nil {
		s.writeJSON(w, http.StatusBadRequest, Response{Code: 1419, Message: err.Error()})
		return
	}
	pu := s.authService.ResolveUserForMe(subject)
	if pu == nil {
		s.writeJSON(w, http.StatusUnauthorized, Response{Code: 1408, Message: "user not found"})
		return
	}
	inviteCode := strings.TrimSpace(pu.InviteCode)
	d1, n1, d2, n2, _ := s.authService.GetInviterInfoByAccountID(subject)
	aux := buildAuthUserFieldsWithNames(pu.ID, pu.Email, pu.Phone, pu.DisplayName, pu.Provider, inviteCode,
		pu.CurGold, pu.TotalGold, pu.CurToken, pu.TotalToken, d1, d2, n1, n2)
	aux.AdRewardsDisabled = pu.AdRewardsDisabled
	s.writeJSON(w, http.StatusOK, Response{Code: 0, Message: "ok", Data: aux})
}
