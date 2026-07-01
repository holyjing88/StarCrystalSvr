package store

import (
	"strings"

	providertypes "starcrystal/server/internal/provider"
)

// RankListDisplayName returns a user-facing label for leaderboard rows (never raw account_id).
func RankListDisplayName(rec *AuthUserRecord, accountID string) string {
	if rec != nil {
		if dn := strings.TrimSpace(rec.DisplayName); dn != "" && !looksLikeAccountID(dn) {
			return dn
		}
		if nn := strings.TrimSpace(rec.Nickname); nn != "" && !looksLikeAccountID(nn) {
			return nn
		}
		if strings.EqualFold(strings.TrimSpace(rec.Provider), providertypes.Guest) ||
			strings.EqualFold(strings.TrimSpace(rec.AccountType), providertypes.Guest) {
			if dn := strings.TrimSpace(rec.DisplayName); dn != "" {
				return dn
			}
			return "Guest"
		}
		if h := phoneHintLabel(rec.Phone); h != "" {
			return h
		}
		if e := strings.TrimSpace(rec.Email); e != "" && strings.Contains(e, "@") {
			return maskEmailLocal(e)
		}
	}
	aid := strings.TrimSpace(accountID)
	if aid != "" && !looksLikeAccountID(aid) {
		return aid
	}
	return "Player"
}

func looksLikeAccountID(s string) bool {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return false
	}
	return strings.HasPrefix(s, "guest_") ||
		strings.HasPrefix(s, "phone_") ||
		strings.HasPrefix(s, "email_")
}

func phoneHintLabel(phone string) string {
	p := strings.TrimSpace(phone)
	if len(p) < 4 {
		return ""
	}
	return "…" + p[len(p)-4:]
}

func maskEmailLocal(email string) string {
	email = strings.TrimSpace(email)
	at := strings.Index(email, "@")
	if at <= 1 {
		return ""
	}
	return email[:1] + "…" + email[at:]
}
