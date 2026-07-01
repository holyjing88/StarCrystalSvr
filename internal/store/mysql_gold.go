package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// EconomyBalances welfare + invite economy snapshot for rank/settlement.
type EconomyBalances struct {
	CurGold    float64
	TotalGold  float64
	CurToken   float64
	TotalToken float64

	CurDownlineL1Contrib   float64
	TotalDownlineL1Contrib float64
	CurDownlineL2Contrib   float64
	TotalDownlineL2Contrib float64

	CurDirectInviterShare   float64
	TotalDirectInviterShare float64
	CurSecondInviterShare   float64
	TotalSecondInviterShare float64
}

const economyBalanceSelect = `
SELECT COALESCE(curgold,0), COALESCE(totalgold,0), COALESCE(curtoken,0), COALESCE(totaltoken,0),
       COALESCE(cur_downline_l1_contrib,0), COALESCE(total_downline_l1_contrib,0),
       COALESCE(cur_downline_l2_contrib,0), COALESCE(total_downline_l2_contrib,0),
       COALESCE(cur_direct_inviter_share,0), COALESCE(total_direct_inviter_share,0),
       COALESCE(cur_second_inviter_share,0), COALESCE(total_second_inviter_share,0)
FROM auth_accounts WHERE account_id = ? AND deleted_at IS NULL`

func scanEconomyBalances(row scanner) (EconomyBalances, error) {
	var b EconomyBalances
	err := row.Scan(
		&b.CurGold, &b.TotalGold, &b.CurToken, &b.TotalToken,
		&b.CurDownlineL1Contrib, &b.TotalDownlineL1Contrib,
		&b.CurDownlineL2Contrib, &b.TotalDownlineL2Contrib,
		&b.CurDirectInviterShare, &b.TotalDirectInviterShare,
		&b.CurSecondInviterShare, &b.TotalSecondInviterShare,
	)
	return b, err
}

type scanner interface {
	Scan(dest ...any) error
}

// EffectiveCurGold v1.0.15 welfare gold cur board score.
func (b EconomyBalances) EffectiveCurGold() float64 {
	return b.CurGold + b.CurDownlineL1Contrib + b.CurDownlineL2Contrib
}

// EffectiveTotalGold v1.0.15 welfare gold total board score.
func (b EconomyBalances) EffectiveTotalGold() float64 {
	return b.TotalGold + b.TotalDownlineL1Contrib + b.TotalDownlineL2Contrib
}

func (b EconomyBalances) DownContribCur() float64 {
	return b.CurDownlineL1Contrib + b.CurDownlineL2Contrib
}

func (b EconomyBalances) DownContribTotal() float64 {
	return b.TotalDownlineL1Contrib + b.TotalDownlineL2Contrib
}

func (b EconomyBalances) UpContribCur() float64 {
	return b.CurDirectInviterShare + b.CurSecondInviterShare
}

func (b EconomyBalances) UpContribTotal() float64 {
	return b.TotalDirectInviterShare + b.TotalSecondInviterShare
}

// MonthlyGoldSpent v7.2 settlement numerator.
func (b EconomyBalances) MonthlyGoldSpent() float64 {
	return b.EffectiveCurGold()
}

// GetEconomyBalances reads economy + invite fields for an account.
func (s *MySQLPlayerRepository) GetEconomyBalances(ctx context.Context, accountID string) (EconomyBalances, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return EconomyBalances{}, fmt.Errorf("empty account id")
	}
	var b EconomyBalances
	err := s.withRetry(ctx, func(db *sql.DB) error {
		row := db.QueryRowContext(ctx, economyBalanceSelect+" LIMIT 1", accountID)
		var e error
		b, e = scanEconomyBalances(row)
		return e
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return EconomyBalances{}, ErrNotFound
		}
		return EconomyBalances{}, err
	}
	return b, nil
}

// ApplyCurGoldDelta updates curgold; negative delta also reduces totalgold (v7.2 pair).
func (s *MySQLPlayerRepository) ApplyCurGoldDelta(ctx context.Context, accountID string, delta float64) (before, after EconomyBalances, err error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return EconomyBalances{}, EconomyBalances{}, fmt.Errorf("empty account id")
	}
	err = s.withRetry(ctx, func(db *sql.DB) error {
		tx, e := db.BeginTx(ctx, nil)
		if e != nil {
			return e
		}
		defer func() { _ = tx.Rollback() }()

		row := tx.QueryRowContext(ctx, economyBalanceSelect+" FOR UPDATE", accountID)
		b, e := scanEconomyBalances(row)
		if e != nil {
			if errors.Is(e, sql.ErrNoRows) {
				return ErrNotFound
			}
			return e
		}
		before = b
		newCG := b.CurGold + delta
		if newCG < 0 {
			return fmt.Errorf("insufficient gold")
		}
		newTG := b.TotalGold
		if delta < 0 {
			newTG = b.TotalGold + delta
			if newTG < 0 {
				return fmt.Errorf("insufficient gold")
			}
		}
		res, e := tx.ExecContext(ctx, `
UPDATE auth_accounts SET curgold = ?, totalgold = ?, updated_at = CURRENT_TIMESTAMP
WHERE account_id = ? AND deleted_at IS NULL`, newCG, newTG, accountID)
		if e != nil {
			return e
		}
		aff, e := res.RowsAffected()
		if e != nil {
			return e
		}
		if aff <= 0 {
			return ErrNotFound
		}
		after = b
		after.CurGold = newCG
		after.TotalGold = newTG
		return tx.Commit()
	})
	return before, after, err
}

// SetCurGold sets curgold to target (v7.1 GM/idip Set).
func (s *MySQLPlayerRepository) SetCurGold(ctx context.Context, accountID string, target float64) (before, after EconomyBalances, err error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return EconomyBalances{}, EconomyBalances{}, fmt.Errorf("empty account id")
	}
	if target < 0 {
		return EconomyBalances{}, EconomyBalances{}, fmt.Errorf("invalid target gold")
	}
	err = s.withRetry(ctx, func(db *sql.DB) error {
		tx, e := db.BeginTx(ctx, nil)
		if e != nil {
			return e
		}
		defer func() { _ = tx.Rollback() }()

		row := tx.QueryRowContext(ctx, economyBalanceSelect+" FOR UPDATE", accountID)
		b, e := scanEconomyBalances(row)
		if e != nil {
			if errors.Is(e, sql.ErrNoRows) {
				return ErrNotFound
			}
			return e
		}
		before = b
		res, e := tx.ExecContext(ctx, `
UPDATE auth_accounts SET curgold = ?, updated_at = CURRENT_TIMESTAMP
WHERE account_id = ? AND deleted_at IS NULL`, target, accountID)
		if e != nil {
			return e
		}
		aff, e := res.RowsAffected()
		if e != nil {
			return e
		}
		if aff <= 0 {
			return ErrNotFound
		}
		after = b
		after.CurGold = target
		return tx.Commit()
	})
	return before, after, err
}

// ApplyCurTokenDelta updates only curtoken.
func (s *MySQLPlayerRepository) ApplyCurTokenDelta(ctx context.Context, accountID string, delta float64) (before, after EconomyBalances, err error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return EconomyBalances{}, EconomyBalances{}, fmt.Errorf("empty account id")
	}
	err = s.withRetry(ctx, func(db *sql.DB) error {
		tx, e := db.BeginTx(ctx, nil)
		if e != nil {
			return e
		}
		defer func() { _ = tx.Rollback() }()

		row := tx.QueryRowContext(ctx, economyBalanceSelect+" FOR UPDATE", accountID)
		b, e := scanEconomyBalances(row)
		if e != nil {
			if errors.Is(e, sql.ErrNoRows) {
				return ErrNotFound
			}
			return e
		}
		before = b
		newCT := b.CurToken + delta
		if newCT < 0 {
			return fmt.Errorf("insufficient token")
		}
		res, e := tx.ExecContext(ctx, `
UPDATE auth_accounts SET curtoken = ?, updated_at = CURRENT_TIMESTAMP
WHERE account_id = ? AND deleted_at IS NULL`, newCT, accountID)
		if e != nil {
			return e
		}
		aff, e := res.RowsAffected()
		if e != nil {
			return e
		}
		if aff <= 0 {
			return ErrNotFound
		}
		after = b
		after.CurToken = newCT
		return tx.Commit()
	})
	return before, after, err
}

// RedeemTokenForGift totaltoken += curtoken, curtoken = 0.
func (s *MySQLPlayerRepository) RedeemTokenForGift(ctx context.Context, accountID string) (redeemAmount float64, after EconomyBalances, err error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return 0, EconomyBalances{}, fmt.Errorf("empty account id")
	}
	err = s.withRetry(ctx, func(db *sql.DB) error {
		tx, e := db.BeginTx(ctx, nil)
		if e != nil {
			return e
		}
		defer func() { _ = tx.Rollback() }()

		row := tx.QueryRowContext(ctx, economyBalanceSelect+" FOR UPDATE", accountID)
		b, e := scanEconomyBalances(row)
		if e != nil {
			if errors.Is(e, sql.ErrNoRows) {
				return ErrNotFound
			}
			return e
		}
		if b.CurToken <= 0 {
			return fmt.Errorf("no token to redeem")
		}
		redeemAmount = b.CurToken
		newTT := b.TotalToken + redeemAmount
		res, e := tx.ExecContext(ctx, `
UPDATE auth_accounts SET curtoken = 0, totaltoken = ?, updated_at = CURRENT_TIMESTAMP
WHERE account_id = ? AND deleted_at IS NULL`, newTT, accountID)
		if e != nil {
			return e
		}
		aff, e := res.RowsAffected()
		if e != nil {
			return e
		}
		if aff <= 0 {
			return ErrNotFound
		}
		after = b
		after.CurToken = 0
		after.TotalToken = newTT
		return tx.Commit()
	})
	return redeemAmount, after, err
}

// ApplyMonthlyGoldSettlement v7.2: clear cur cycle fields, pair totalgold/totaltoken.
func (s *MySQLPlayerRepository) ApplyMonthlyGoldSettlement(ctx context.Context, accountID string, goldSpent, tokenDelta float64) (after EconomyBalances, err error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return EconomyBalances{}, fmt.Errorf("empty account id")
	}
	err = s.withRetry(ctx, func(db *sql.DB) error {
		tx, e := db.BeginTx(ctx, nil)
		if e != nil {
			return e
		}
		defer func() { _ = tx.Rollback() }()

		row := tx.QueryRowContext(ctx, economyBalanceSelect+`
  AND COALESCE(status,1) <> 2 FOR UPDATE`, accountID)
		b, e := scanEconomyBalances(row)
		if e != nil {
			if errors.Is(e, sql.ErrNoRows) {
				return ErrNotFound
			}
			return e
		}
		newTG := b.TotalGold + goldSpent
		newCT := b.CurToken + tokenDelta
		newTT := b.TotalToken + tokenDelta
		res, e := tx.ExecContext(ctx, `
UPDATE auth_accounts
SET curgold = 0,
    totalgold = ?,
    cur_downline_l1_contrib = 0,
    cur_downline_l2_contrib = 0,
    cur_direct_inviter_share = 0,
    cur_second_inviter_share = 0,
    notify_watermark_downline_l1 = 0,
    notify_watermark_downline_l2 = 0,
    curtoken = ?,
    totaltoken = ?,
    updated_at = CURRENT_TIMESTAMP
WHERE account_id = ? AND deleted_at IS NULL`, newTG, newCT, newTT, accountID)
		if e != nil {
			return e
		}
		aff, e := res.RowsAffected()
		if e != nil {
			return e
		}
		if aff <= 0 {
			return ErrNotFound
		}
		after = b
		after.CurGold = 0
		after.TotalGold = newTG
		after.CurDownlineL1Contrib = 0
		after.CurDownlineL2Contrib = 0
		after.CurDirectInviterShare = 0
		after.CurSecondInviterShare = 0
		after.CurToken = newCT
		after.TotalToken = newTT
		return tx.Commit()
	})
	return after, err
}

// ListAccountIDsForMonthlySettlement returns active non-banned account ids.
func (s *MySQLPlayerRepository) ListAccountIDsForMonthlySettlement(ctx context.Context) ([]string, error) {
	var ids []string
	err := s.withRetry(ctx, func(db *sql.DB) error {
		rows, e := db.QueryContext(ctx, `
SELECT account_id FROM auth_accounts
WHERE deleted_at IS NULL AND COALESCE(status,1) <> 2`)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			if e := rows.Scan(&id); e != nil {
				return e
			}
			ids = append(ids, id)
		}
		return rows.Err()
	})
	return ids, err
}

// InsertWelfareExchangeLog records one monthly settlement row.
func (s *MySQLPlayerRepository) InsertWelfareExchangeLog(ctx context.Context, accountID, yyyymm string, goldSpent, tokenDelta, rate, snapshot float64) error {
	accountID = strings.TrimSpace(accountID)
	yyyymm = strings.TrimSpace(yyyymm)
	if accountID == "" || yyyymm == "" {
		return fmt.Errorf("invalid exchange log args")
	}
	return s.withRetry(ctx, func(db *sql.DB) error {
		_, err := db.ExecContext(ctx, `
INSERT INTO welfare_exchange_log (account_id, yyyymm, gold_spent, token_delta, rate, server_delta_snapshot, rule_version)
VALUES (?, ?, ?, ?, ?, ?, 'v7.2')`, accountID, yyyymm, goldSpent, tokenDelta, rate, snapshot)
		return err
	})
}

func validYyyymm(yyyymm string) bool {
	if len(yyyymm) != 6 {
		return false
	}
	for _, c := range yyyymm {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// CountMonthlyTokenLeaderboard counts rows for yyyymm (capped at 1000 for IDIP).
func (s *MySQLPlayerRepository) CountMonthlyTokenLeaderboard(ctx context.Context, yyyymm string) (int, error) {
	yyyymm = strings.TrimSpace(yyyymm)
	if !validYyyymm(yyyymm) {
		return 0, fmt.Errorf("invalid yyyymm")
	}
	var n int
	err := s.withRetry(ctx, func(db *sql.DB) error {
		return db.QueryRowContext(ctx, `SELECT COUNT(*) FROM welfare_exchange_log WHERE yyyymm = ?`, yyyymm).Scan(&n)
	})
	if err != nil {
		return 0, err
	}
	if n > 1000 {
		n = 1000
	}
	return n, nil
}

// ListMonthlyTokenLeaderboard returns token_delta DESC page (IDIP-011).
func (s *MySQLPlayerRepository) ListMonthlyTokenLeaderboard(ctx context.Context, yyyymm string, offset, limit int) ([]MonthlyTokenLeaderboardRow, error) {
	yyyymm = strings.TrimSpace(yyyymm)
	if !validYyyymm(yyyymm) {
		return nil, fmt.Errorf("invalid yyyymm")
	}
	if offset < 0 {
		offset = 0
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}
	if offset >= 1000 {
		return nil, nil
	}
	if offset+limit > 1000 {
		limit = 1000 - offset
	}
	var out []MonthlyTokenLeaderboardRow
	err := s.withRetry(ctx, func(db *sql.DB) error {
		rows, e := db.QueryContext(ctx, `
SELECT l.account_id,
       COALESCE(NULLIF(TRIM(a.display_name), ''), NULLIF(TRIM(a.nickname), ''), ''),
       COALESCE(ic.invite_code, ''),
       l.gold_spent, l.token_delta, l.rate, l.server_delta_snapshot, l.created_at
FROM welfare_exchange_log l
INNER JOIN auth_accounts a ON a.account_id = l.account_id AND a.deleted_at IS NULL
LEFT JOIN auth_invite_codes ic ON ic.account_id = l.account_id
WHERE l.yyyymm = ?
ORDER BY l.token_delta DESC, l.account_id ASC
LIMIT ? OFFSET ?`, yyyymm, limit, offset)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			var row MonthlyTokenLeaderboardRow
			if e := rows.Scan(&row.AccountID, &row.DisplayName, &row.InviteCode,
				&row.GoldSpent, &row.TokenDelta, &row.Rate, &row.ServerDeltaSnapshot, &row.SettledAt); e != nil {
				return e
			}
			out = append(out, row)
		}
		return rows.Err()
	})
	return out, err
}

// ListWelfareExchangeLogByAccount returns recent settlement rows newest first.
func (s *MySQLPlayerRepository) ListWelfareExchangeLogByAccount(ctx context.Context, accountID string, limit int) ([]WelfareExchangeLogRow, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return nil, fmt.Errorf("account_id required")
	}
	if limit <= 0 {
		limit = 6
	}
	if limit > 24 {
		limit = 24
	}
	var out []WelfareExchangeLogRow
	err := s.withRetry(ctx, func(db *sql.DB) error {
		rows, e := db.QueryContext(ctx, `
SELECT yyyymm, gold_spent, token_delta, rate, server_delta_snapshot, created_at
FROM welfare_exchange_log
WHERE account_id = ?
ORDER BY yyyymm DESC, created_at DESC
LIMIT ?`, accountID, limit)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			var row WelfareExchangeLogRow
			if e := rows.Scan(&row.Yyyymm, &row.GoldSpent, &row.TokenDelta, &row.Rate, &row.ServerDeltaSnapshot, &row.SettledAt); e != nil {
				return e
			}
			out = append(out, row)
		}
		return rows.Err()
	})
	return out, err
}

// CountDirectInvitees counts auth_invite_members rows for inviter.
func (s *MySQLPlayerRepository) CountDirectInvitees(ctx context.Context, accountID string) (int, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return 0, nil
	}
	var n int
	err := s.withRetry(ctx, func(db *sql.DB) error {
		return db.QueryRowContext(ctx, `SELECT COUNT(*) FROM auth_invite_members WHERE accountid = ?`, accountID).Scan(&n)
	})
	return n, err
}
