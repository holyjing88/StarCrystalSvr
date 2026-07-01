package service

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"

	providertypes "starcrystal/server/internal/provider"
	"starcrystal/server/internal/store"
)

// oauthAccountID builds a stable auth_accounts.account_id from provider + OAuth subject.
func oauthAccountID(provider, sub string) string {
	p := strings.ToLower(strings.TrimSpace(provider))
	sub = strings.TrimSpace(sub)
	if sub == "" {
		sub = "unknown"
	}
	var b strings.Builder
	for _, r := range strings.ToLower(sub) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		}
	}
	safe := b.String()
	if safe == "" {
		safe = hex.EncodeToString([]byte(sub))
		if len(safe) > 64 {
			safe = safe[:64]
		}
	}
	return p + "_" + safe
}

// LoginOrLinkOAuth loads or creates a social account row in auth_accounts (password unused).
func (s *AuthService) LoginOrLinkOAuth(provider, sub, email, displayName string) (*userRecord, string, error) {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider != providertypes.Google && provider != providertypes.Facebook {
		return nil, "", fmt.Errorf("unsupported provider")
	}
	sub = strings.TrimSpace(sub)
	if sub == "" {
		return nil, "", fmt.Errorf("missing social subject id")
	}
	ctx := context.Background()
	accountID := oauthAccountID(provider, sub)
	rec, e := s.playerRepo.GetByAccountID(ctx, accountID)
	if e != nil && !store.IsNotFound(e) {
		return nil, "", fmt.Errorf("db oauth lookup failed: %v", e)
	}
	if rec != nil && e == nil {
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

	emailNorm := strings.ToLower(strings.TrimSpace(email))
	disp := ensureDisplayName(displayName)
	newRec := &store.AuthUserRecord{
		UserID:       accountID,
		AccountID:    accountID,
		AccountType:  provider,
		AccountValue: sub,
		PasswordHash: "oauth_nologin",
		Provider:     provider,
		DisplayName:  disp,
		Email:        emailNorm,
	}
	if ce := s.playerRepo.CreateByAccountID(ctx, newRec); ce != nil {
		return nil, "", fmt.Errorf("db oauth create failed: %v", ce)
	}
	u := &userRecord{
		ID:          accountID,
		Email:       emailNorm,
		Provider:    provider,
		DisplayName: disp,
	}
	if err := s.assignInviteCodeForNewAccount(ctx, accountID, u); err != nil {
		return nil, "", err
	}
	tok, te := s.issueToken(accountID)
	return u, tok, te
}
