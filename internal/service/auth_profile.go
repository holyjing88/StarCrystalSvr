package service

import (
	"context"
	"fmt"
	"strings"
)

// UpdateProfileDisplayName 验证 tokenSubject 后更新昵称（auth_accounts）。
func (s *AuthService) UpdateProfileDisplayName(tokenSubject, accountHint, displayName string) error {
	sub := strings.TrimSpace(tokenSubject)
	hint := strings.TrimSpace(accountHint)
	if sub == "" {
		return fmt.Errorf("empty subject")
	}
	if hint != "" && !strings.EqualFold(hint, sub) {
		return fmt.Errorf("account mismatch")
	}
	disp := ensureDisplayName(displayName)
	return s.playerRepo.UpdateDisplayNameByAccountID(context.Background(), sub, disp)
}
