package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// AckInviteNotifyWatermarks sets notify_watermark_downline_* to current cur_downline_*.
func (s *MySQLPlayerRepository) AckInviteNotifyWatermarks(ctx context.Context, accountID string) error {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return fmt.Errorf("empty account id")
	}
	return s.withRetry(ctx, func(db *sql.DB) error {
		_, err := db.ExecContext(ctx, `
UPDATE auth_accounts SET
  notify_watermark_downline_l1 = cur_downline_l1_contrib,
  notify_watermark_downline_l2 = cur_downline_l2_contrib,
  notify_pending_since = NULL,
  updated_at = CURRENT_TIMESTAMP
WHERE account_id = ? AND deleted_at IS NULL`, accountID)
		return err
	})
}

// EnsureNotifyPendingSince stamps notify_pending_since when user has unacked pending contrib.
func (s *MySQLPlayerRepository) EnsureNotifyPendingSince(ctx context.Context, accountID string) error {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return fmt.Errorf("empty account id")
	}
	return s.withRetry(ctx, func(db *sql.DB) error {
		_, err := db.ExecContext(ctx, `
UPDATE auth_accounts SET notify_pending_since = CURRENT_TIMESTAMP
WHERE account_id = ? AND deleted_at IS NULL
  AND notify_pending_since IS NULL
  AND (
    cur_downline_l1_contrib > COALESCE(notify_watermark_downline_l1, 0)
    OR cur_downline_l2_contrib > COALESCE(notify_watermark_downline_l2, 0)
  )`, accountID)
		return err
	})
}

// SumPlatformShareForMonth sums platform_share in auth_invite_contrib_log for yyyymm.
func (s *MySQLPlayerRepository) SumPlatformShareForMonth(ctx context.Context, yyyymm string) (float64, error) {
	yyyymm = strings.TrimSpace(yyyymm)
	if len(yyyymm) != 6 {
		return 0, fmt.Errorf("invalid yyyymm")
	}
	var sum sql.NullFloat64
	err := s.withRetry(ctx, func(db *sql.DB) error {
		return db.QueryRowContext(ctx, `
SELECT COALESCE(SUM(platform_share), 0)
FROM auth_invite_contrib_log
WHERE DATE_FORMAT(created_at, '%Y%m') = ?`, yyyymm).Scan(&sum)
	})
	if err != nil {
		return 0, err
	}
	if sum.Valid {
		return sum.Float64, nil
	}
	return 0, nil
}
