package service

import (
	"context"
	"strings"

	"starcrystal/server/internal/logger"
	"starcrystal/server/internal/store"
)

type WelfareFieldChange uint8

const (
	WelfareChangedCurGold WelfareFieldChange = 1 << iota
	WelfareChangedTotalGold
	WelfareChangedCurToken
	WelfareChangedTotalToken
	WelfareChangedInviteFields
)

type WelfareRankSync struct {
	store welfareRankStore
	repo  store.PlayerRepository
}

func NewWelfareRankSync(st welfareRankStore, repo store.PlayerRepository) *WelfareRankSync {
	return &WelfareRankSync{store: st, repo: repo}
}

func (s *WelfareRankSync) Notify(ctx context.Context, accountID string, changed WelfareFieldChange, bal store.EconomyBalances) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return
	}
	if s.repo != nil {
		if rec, err := s.repo.GetByAccountID(ctx, accountID); err == nil && rec != nil && rec.Status == 2 {
			return
		}
	}
	if (changed == 0 || changed&WelfareChangedInviteFields != 0) && s.repo != nil {
		var err error
		bal, err = s.repo.GetEconomyBalances(ctx, accountID)
		if err != nil {
			logger.Warn(logger.TopicAuth, "[welfare-rank] load balances failed account=%s err=%v", accountID, err)
			return
		}
		changed = WelfareChangedCurGold | WelfareChangedTotalGold | WelfareChangedCurToken | WelfareChangedTotalToken | WelfareChangedInviteFields
	}
	var cmds []welfareBoardScoreCmd
	if changed&(WelfareChangedCurGold|WelfareChangedInviteFields) != 0 {
		cmds = append(cmds,
			welfareBoardScoreCmd{BoardWelfareGoldCur, accountID, bal.EffectiveCurGold()},
			welfareBoardScoreCmd{BoardWelfareDownContribCur, accountID, bal.DownContribCur()},
			welfareBoardScoreCmd{BoardWelfareUpContribCur, accountID, bal.UpContribCur()},
		)
	}
	if changed&(WelfareChangedTotalGold|WelfareChangedInviteFields) != 0 {
		cmds = append(cmds,
			welfareBoardScoreCmd{BoardWelfareGoldTotal, accountID, bal.EffectiveTotalGold()},
			welfareBoardScoreCmd{BoardWelfareDownContribTotal, accountID, bal.DownContribTotal()},
			welfareBoardScoreCmd{BoardWelfareUpContribTotal, accountID, bal.UpContribTotal()},
		)
	}
	if changed&WelfareChangedCurToken != 0 {
		cmds = append(cmds, welfareBoardScoreCmd{BoardWelfareTokenCur, accountID, bal.CurToken})
	}
	if changed&WelfareChangedTotalToken != 0 {
		cmds = append(cmds, welfareBoardScoreCmd{BoardWelfareTokenTotal, accountID, bal.TotalToken})
	}
	s.applyBatch(ctx, cmds)
}

// BatchNotifyWelfareRanks reloads balances and updates all gold-related boards (Pipeline).
func (s *WelfareRankSync) BatchNotifyWelfareRanks(ctx context.Context, accountIDs []string) {
	if s.repo == nil || len(accountIDs) == 0 {
		return
	}
	seen := make(map[string]struct{}, len(accountIDs))
	var cmds []welfareBoardScoreCmd
	for _, id := range accountIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		if rec, err := s.repo.GetByAccountID(ctx, id); err == nil && rec != nil && rec.Status == 2 {
			continue
		}
		bal, err := s.repo.GetEconomyBalances(ctx, id)
		if err != nil {
			logger.Warn(logger.TopicAuth, "[welfare-rank] batch load failed account=%s err=%v", id, err)
			continue
		}
		cmds = append(cmds,
			welfareBoardScoreCmd{BoardWelfareGoldCur, id, bal.EffectiveCurGold()},
			welfareBoardScoreCmd{BoardWelfareGoldTotal, id, bal.EffectiveTotalGold()},
			welfareBoardScoreCmd{BoardWelfareDownContribCur, id, bal.DownContribCur()},
			welfareBoardScoreCmd{BoardWelfareDownContribTotal, id, bal.DownContribTotal()},
			welfareBoardScoreCmd{BoardWelfareUpContribCur, id, bal.UpContribCur()},
			welfareBoardScoreCmd{BoardWelfareUpContribTotal, id, bal.UpContribTotal()},
		)
	}
	s.applyBatch(ctx, cmds)
}

func (s *WelfareRankSync) applyBatch(ctx context.Context, cmds []welfareBoardScoreCmd) {
	if len(cmds) == 0 {
		return
	}
	if err := s.store.batchSetScores(ctx, cmds); err != nil {
		logger.Warn(logger.TopicAuth, "[welfare-rank] batchSetScores err=%v", err)
	}
}

func (s *WelfareRankSync) ListBoard(ctx context.Context, board string, limit int) ([]WelfareRankRow, error) {
	board = NormalizeWelfareBoard(board)
	rows, err := s.store.top(ctx, board, limit)
	if err != nil {
		return nil, err
	}
	if len(rows) > 0 {
		return rows, nil
	}
	return s.listBoardFromRepo(ctx, board, limit)
}

type welfareBoardLister interface {
	ListTopWelfareBoard(ctx context.Context, board string, limit int) ([]store.WelfareRankCandidate, error)
}

func (s *WelfareRankSync) listBoardFromRepo(ctx context.Context, board string, limit int) ([]WelfareRankRow, error) {
	lister, ok := s.repo.(welfareBoardLister)
	if !ok || lister == nil {
		return nil, nil
	}
	cands, err := lister.ListTopWelfareBoard(ctx, board, limit)
	if err != nil {
		return nil, err
	}
	out := make([]WelfareRankRow, 0, len(cands))
	var cmds []welfareBoardScoreCmd
	for _, c := range cands {
		out = append(out, WelfareRankRow{AccountID: c.AccountID, Score: c.Score})
		cmds = append(cmds, welfareBoardScoreCmd{board, c.AccountID, c.Score})
	}
	s.applyBatch(ctx, cmds)
	if len(out) > 0 {
		logger.Info(logger.TopicAuth, "[welfare-rank] list fallback from mysql board=%s rows=%d", board, len(out))
	}
	return out, nil
}

func welfareScoreFromBalances(board string, bal store.EconomyBalances) float64 {
	switch NormalizeWelfareBoard(board) {
	case BoardWelfareGoldCur:
		return bal.EffectiveCurGold()
	case BoardWelfareGoldTotal:
		return bal.EffectiveTotalGold()
	case BoardWelfareDownContribCur:
		return bal.DownContribCur()
	case BoardWelfareDownContribTotal:
		return bal.DownContribTotal()
	case BoardWelfareUpContribCur:
		return bal.UpContribCur()
	case BoardWelfareUpContribTotal:
		return bal.UpContribTotal()
	case BoardWelfareTokenCur:
		return bal.CurToken
	case BoardWelfareTokenTotal:
		return bal.TotalToken
	default:
		return 0
	}
}

func (s *WelfareRankSync) MemberRank(ctx context.Context, board, accountID string) (rank int64, score float64, onBoard bool, err error) {
	board = NormalizeWelfareBoard(board)
	rank, score, onBoard, err = s.store.memberRank(ctx, board, accountID)
	if err != nil || onBoard {
		return rank, score, onBoard, err
	}
	if s.repo == nil {
		return 0, 0, false, nil
	}
	bal, err := s.repo.GetEconomyBalances(ctx, accountID)
	if err != nil {
		if store.IsNotFound(err) {
			return 0, 0, false, nil
		}
		return 0, 0, false, err
	}
	sc := welfareScoreFromBalances(board, bal)
	if sc <= 0 {
		return 0, 0, false, nil
	}
	s.applyBatch(ctx, []welfareBoardScoreCmd{{board, accountID, sc}})
	rows, err := s.ListBoard(ctx, board, ClampRankListLimit(500))
	if err != nil {
		return 0, sc, true, nil
	}
	for i, row := range rows {
		if row.AccountID == accountID {
			return int64(i + 1), row.Score, true, nil
		}
	}
	return 0, sc, true, nil
}
