package store

import (
	"context"
	"database/sql"
)

type InviteShareLayer struct {
	Layer                              int
	BeneficiaryID                      string
	RawShare, PaidShare, PlatformShare float64
}

type InviteShareEarnParams struct {
	EarnerAccountID, EarnerNickname, GrantKind string
	BaseGold                                   float64
	BizType, BizNo                             string
	Layers                                     []InviteShareLayer
}

func (s *MySQLPlayerRepository) ApplyInviteShareEarn(ctx context.Context, p InviteShareEarnParams) error {
	if p.BaseGold <= 0 {
		return nil
	}
	return s.withRetry(ctx, func(db *sql.DB) error {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer func() { _ = tx.Rollback() }()
		for _, layer := range p.Layers {
			if err := insertContribLogTx(ctx, tx, p, layer); err != nil {
				return err
			}
		}
		var dPaid, sPaid float64
		for _, layer := range p.Layers {
			switch layer.Layer {
			case 1:
				dPaid = layer.PaidShare
			case 2:
				sPaid = layer.PaidShare
			}
		}
		if dPaid != 0 || sPaid != 0 {
			if _, err := tx.ExecContext(ctx, `
UPDATE auth_accounts SET
  cur_direct_inviter_share = cur_direct_inviter_share + ?,
  total_direct_inviter_share = total_direct_inviter_share + ?,
  cur_second_inviter_share = cur_second_inviter_share + ?,
  total_second_inviter_share = total_second_inviter_share + ?,
  updated_at = CURRENT_TIMESTAMP
WHERE account_id = ? AND deleted_at IS NULL`, dPaid, dPaid, sPaid, sPaid, p.EarnerAccountID); err != nil {
				return err
			}
		}
		for _, layer := range p.Layers {
			if layer.PaidShare <= 0 || layer.BeneficiaryID == "" || layer.BeneficiaryID == "platform" {
				continue
			}
			switch layer.Layer {
			case 1:
				_, err = tx.ExecContext(ctx, `
UPDATE auth_accounts SET cur_downline_l1_contrib = cur_downline_l1_contrib + ?,
  total_downline_l1_contrib = total_downline_l1_contrib + ?,
  notify_pending_since = CASE
    WHEN notify_pending_since IS NULL
      AND (cur_downline_l1_contrib + ?) > COALESCE(notify_watermark_downline_l1, 0)
    THEN CURRENT_TIMESTAMP ELSE notify_pending_since END,
  updated_at = CURRENT_TIMESTAMP
WHERE account_id = ? AND deleted_at IS NULL`, layer.PaidShare, layer.PaidShare, layer.PaidShare, layer.BeneficiaryID)
			case 2:
				_, err = tx.ExecContext(ctx, `
UPDATE auth_accounts SET cur_downline_l2_contrib = cur_downline_l2_contrib + ?,
  total_downline_l2_contrib = total_downline_l2_contrib + ?,
  notify_pending_since = CASE
    WHEN notify_pending_since IS NULL
      AND (cur_downline_l2_contrib + ?) > COALESCE(notify_watermark_downline_l2, 0)
    THEN CURRENT_TIMESTAMP ELSE notify_pending_since END,
  updated_at = CURRENT_TIMESTAMP
WHERE account_id = ? AND deleted_at IS NULL`, layer.PaidShare, layer.PaidShare, layer.PaidShare, layer.BeneficiaryID)
			}
			if err != nil {
				return err
			}
		}
		return tx.Commit()
	})
}

func insertContribLogTx(ctx context.Context, tx *sql.Tx, p InviteShareEarnParams, layer InviteShareLayer) error {
	ben := layer.BeneficiaryID
	if ben == "" {
		ben = "platform"
	}
	_, err := tx.ExecContext(ctx, `
INSERT INTO auth_invite_contrib_log (
  earner_account_id, beneficiary_account_id, layer, grant_kind,
  base_gold, raw_share, paid_share, platform_share, biz_type, biz_no, earner_nickname
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.EarnerAccountID, ben, layer.Layer, p.GrantKind, p.BaseGold,
		layer.RawShare, layer.PaidShare, layer.PlatformShare,
		nullStr(p.BizType), nullStr(p.BizNo), nullStr(p.EarnerNickname))
	return err
}

func nullStr(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}
