package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

func randomAdWatchID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *MySQLPlayerRepository) CreateAdWatchSession(ctx context.Context, accountID, slot string, ttl time.Duration) (watchID string, todayCount int, totalCount int, err error) {
	accountID = strings.TrimSpace(accountID)
	slot = strings.TrimSpace(slot)
	if accountID == "" || ttl <= 0 {
		return "", 0, 0, fmt.Errorf("invalid ad session args")
	}
	ttlSec := int(ttl.Round(time.Second) / time.Second)
	if ttlSec < 60 {
		ttlSec = 60
	}
	wid, err := randomAdWatchID()
	if err != nil {
		return "", 0, 0, err
	}
	err = s.withRetry(ctx, func(db *sql.DB) error {
		_, e := db.ExecContext(ctx, `
INSERT INTO auth_ad_watch_sessions (watch_id, account_id, slot, expires_at, consumed, created_at)
VALUES (?, ?, ?, DATE_ADD(UTC_TIMESTAMP(), INTERVAL ? SECOND), 0, UTC_TIMESTAMP())
`, wid, accountID, slot, ttlSec)
		return e
	})
	if err != nil {
		return "", 0, 0, err
	}

	err = s.withRetry(ctx, func(db *sql.DB) error {
		row := db.QueryRowContext(ctx, `
SELECT COUNT(*) FROM auth_ad_completions WHERE account_id = ? AND DATE(completed_at) = CURDATE()`,
			accountID)
		if e := row.Scan(&todayCount); e != nil {
			return e
		}
		row2 := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM auth_ad_completions WHERE account_id = ?`, accountID)
		return row2.Scan(&totalCount)
	})
	if err != nil {
		return "", 0, 0, err
	}
	return wid, todayCount, totalCount, nil
}

func (s *MySQLPlayerRepository) CountAdCompletionsToday(ctx context.Context, accountID string) (n int, err error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return 0, fmt.Errorf("invalid account")
	}
	err = s.withRetry(ctx, func(db *sql.DB) error {
		row := db.QueryRowContext(ctx, `
SELECT COUNT(*) FROM auth_ad_completions WHERE account_id = ? AND DATE(completed_at) = CURDATE()`,
			accountID)
		return row.Scan(&n)
	})
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (s *MySQLPlayerRepository) CountPendingAdWatchSessions(ctx context.Context, accountID string) (n int, err error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return 0, fmt.Errorf("invalid account")
	}
	err = s.withRetry(ctx, func(db *sql.DB) error {
		row := db.QueryRowContext(ctx, `
SELECT COUNT(*) FROM auth_ad_watch_sessions
WHERE account_id = ? AND consumed = 0 AND expires_at > UTC_TIMESTAMP()`, accountID)
		return row.Scan(&n)
	})
	if err != nil {
		return 0, err
	}
	return n, nil
}

func (s *MySQLPlayerRepository) CompleteAdWatchAndGrant(ctx context.Context, accountID, watchID, slot string, rewardGold, rewardToken float64, minWatchSec int, dailyCompletionCap int) (todayCount int, totalCount int, curGold, totalGold, curToken, totalToken float64, err error) {
	accountID = strings.TrimSpace(accountID)
	watchID = strings.TrimSpace(watchID)
	slot = strings.TrimSpace(slot)
	if accountID == "" || watchID == "" {
		return 0, 0, 0, 0, 0, 0, fmt.Errorf("invalid complete args")
	}

	err = s.withRetry(ctx, func(db *sql.DB) error {
		tx, e := db.BeginTx(ctx, nil)
		if e != nil {
			return e
		}
		defer func() { _ = tx.Rollback() }()

		rowA := tx.QueryRowContext(ctx, `SELECT account_id FROM auth_accounts WHERE account_id = ? AND deleted_at IS NULL LIMIT 1 FOR UPDATE`, accountID)
		var accCheck string
		if e := rowA.Scan(&accCheck); e != nil {
			if errors.Is(e, sql.ErrNoRows) {
				return fmt.Errorf("account not found")
			}
			return e
		}

		var consumed int
		var ageSec sql.NullInt64
		row := tx.QueryRowContext(ctx, `
SELECT consumed,
  TIMESTAMPDIFF(SECOND, created_at, UTC_TIMESTAMP())
FROM auth_ad_watch_sessions
WHERE watch_id = ? AND account_id = ? AND expires_at > UTC_TIMESTAMP() AND consumed = 0
LIMIT 1
FOR UPDATE`, watchID, accountID)
		if e := row.Scan(&consumed, &ageSec); e != nil {
			if errors.Is(e, sql.ErrNoRows) {
				return ErrNotFound
			}
			return e
		}

		if minWatchSec > 0 {
			if !ageSec.Valid || int(ageSec.Int64) < minWatchSec {
				return ErrAdCompletionTooSoon
			}
		}

		if dailyCompletionCap > 0 {
			var doneToday int
			rowCt := tx.QueryRowContext(ctx, `
SELECT COUNT(*) FROM auth_ad_completions
WHERE account_id = ? AND DATE(completed_at) = CURDATE()`, accountID)
			if e := rowCt.Scan(&doneToday); e != nil {
				return e
			}
			if doneToday >= dailyCompletionCap {
				return ErrAdDailyCapExceeded
			}
		}

		res, e := tx.ExecContext(ctx, `UPDATE auth_ad_watch_sessions SET consumed = 1 WHERE watch_id = ? AND account_id = ?`, watchID, accountID)
		if e != nil {
			return e
		}
		aff, e := res.RowsAffected()
		if e != nil {
			return e
		}
		if aff == 0 {
			return ErrNotFound
		}

		_, e = tx.ExecContext(ctx, `
INSERT INTO auth_ad_completions (account_id, watch_id, slot)
VALUES (?, ?, ?)
`, accountID, watchID, slot)
		if e != nil {
			return e
		}

		// v7.1：金币/Token 奖励由 GoldLedgerService.ApplyGold 在 service 层发放（此处仅核销会话）。
		_ = rewardGold
		_ = rewardToken

		return tx.Commit()
	})
	if err != nil {
		return 0, 0, 0, 0, 0, 0, err
	}

	err = s.withRetry(ctx, func(db *sql.DB) error {
		row := db.QueryRowContext(ctx, `
SELECT COUNT(*) FROM auth_ad_completions WHERE account_id = ? AND DATE(completed_at) = CURDATE()`,
			accountID)
		if e := row.Scan(&todayCount); e != nil {
			return e
		}
		row2 := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM auth_ad_completions WHERE account_id = ?`, accountID)
		if e := row2.Scan(&totalCount); e != nil {
			return e
		}
		row3 := db.QueryRowContext(ctx, `
SELECT COALESCE(curgold,0), COALESCE(totalgold,0), COALESCE(curtoken,0), COALESCE(totaltoken,0)
FROM auth_accounts WHERE account_id = ? AND deleted_at IS NULL LIMIT 1`, accountID)
		return row3.Scan(&curGold, &totalGold, &curToken, &totalToken)
	})
	if err != nil {
		return 0, 0, 0, 0, 0, 0, err
	}
	return todayCount, totalCount, curGold, totalGold, curToken, totalToken, nil
}
