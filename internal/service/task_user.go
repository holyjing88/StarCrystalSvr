package service

import (
	"context"
	"strings"
	"time"
)

// TaskUserContext read-only user facts for task metrics.
type TaskUserContext struct {
	AccountAgeDays  int
	AccountIdleDays int
	HasInviter      bool
	ProfileComplete bool
	GuestUpgraded   bool
	InviteTotal     int
}

func (s *TaskService) loadUserContext(ctx context.Context, accountID string, today int) (TaskUserContext, error) {
	var uc TaskUserContext
	firstYmd, err := s.store.GetOrInitFirstSeenYmd(ctx, accountID, today)
	if err != nil {
		return uc, err
	}
	uc.AccountAgeDays = calendarDaysBetween(firstYmd, today)
	if s.repo != nil {
		rec, err := s.repo.GetByAccountID(ctx, accountID)
		if err != nil {
			return uc, err
		}
		uc.HasInviter = strings.TrimSpace(rec.InvitedUserID) != ""
		uc.ProfileComplete = strings.TrimSpace(rec.DisplayName) != "" && !strings.EqualFold(rec.DisplayName, "Guest")
		uc.GuestUpgraded = rec.AccountType != "guest" && (strings.TrimSpace(rec.Phone) != "" || strings.TrimSpace(rec.Email) != "")
		if members, me := s.repo.ListInviteMembersByAccountID(ctx, accountID); me == nil {
			uc.InviteTotal = len(members)
		}
	}
	prev, _ := s.store.TouchLastSeenYmd(ctx, accountID, today)
	if prev > 0 && today > prev {
		uc.AccountIdleDays = calendarDaysBetween(prev, today)
	}
	return uc, nil
}

func isWeekendInGoldTZ(tz string) bool {
	wd := GoldNow(tz).Weekday()
	return wd == time.Saturday || wd == time.Sunday
}

func taskDefVisible(def TaskDef, uc TaskUserContext, tz string) bool {
	if def.Category == TaskCategoryNewbie && uc.AccountAgeDays > def.MaxAccountAgeDays {
		return false
	}
	if def.WeekendOnly && !isWeekendInGoldTZ(tz) {
		return false
	}
	return true
}
