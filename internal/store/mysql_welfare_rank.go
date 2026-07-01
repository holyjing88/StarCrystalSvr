package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// WelfareRankCandidate one row for welfare leaderboard fallback (MySQL source of truth).
type WelfareRankCandidate struct {
	AccountID string
	Score     float64
}

func welfareBoardScoreExpr(board string) (expr string, err error) {
	switch strings.TrimSpace(strings.ToLower(board)) {
	case "welfare_gold_cur":
		return "COALESCE(curgold,0)+COALESCE(cur_downline_l1_contrib,0)+COALESCE(cur_downline_l2_contrib,0)", nil
	case "welfare_gold_total":
		return "COALESCE(totalgold,0)+COALESCE(total_downline_l1_contrib,0)+COALESCE(total_downline_l2_contrib,0)", nil
	case "welfare_down_contrib_cur":
		return "COALESCE(cur_downline_l1_contrib,0)+COALESCE(cur_downline_l2_contrib,0)", nil
	case "welfare_down_contrib_total":
		return "COALESCE(total_downline_l1_contrib,0)+COALESCE(total_downline_l2_contrib,0)", nil
	case "welfare_up_contrib_cur":
		return "COALESCE(cur_direct_inviter_share,0)+COALESCE(cur_second_inviter_share,0)", nil
	case "welfare_up_contrib_total":
		return "COALESCE(total_direct_inviter_share,0)+COALESCE(total_second_inviter_share,0)", nil
	case "welfare_token_cur":
		return "COALESCE(curtoken,0)", nil
	case "welfare_token_total":
		return "COALESCE(totaltoken,0)", nil
	default:
		return "", fmt.Errorf("unknown welfare board: %s", board)
	}
}

func clampWelfareListLimit(limit int) int {
	if limit <= 0 {
		return 50
	}
	if limit > 500 {
		return 500
	}
	return limit
}

// ListTopWelfareBoard reads top accounts by computed score when Redis board is cold/empty.
func (s *MySQLPlayerRepository) ListTopWelfareBoard(ctx context.Context, board string, limit int) ([]WelfareRankCandidate, error) {
	expr, err := welfareBoardScoreExpr(board)
	if err != nil {
		return nil, err
	}
	limit = clampWelfareListLimit(limit)
	q := fmt.Sprintf(`
SELECT account_id, (%s) AS score
FROM auth_accounts
WHERE deleted_at IS NULL AND COALESCE(status, 1) <> 2 AND (%s) > 0
ORDER BY score DESC, account_id ASC
LIMIT ?`, expr, expr)

	var out []WelfareRankCandidate
	err = s.withRetry(ctx, func(db *sql.DB) error {
		rows, e := db.QueryContext(ctx, q, limit)
		if e != nil {
			return e
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			var sc float64
			if e := rows.Scan(&id, &sc); e != nil {
				return e
			}
			id = strings.TrimSpace(id)
			if id == "" || sc <= 0 {
				continue
			}
			out = append(out, WelfareRankCandidate{AccountID: id, Score: sc})
		}
		return rows.Err()
	})
	return out, err
}
