package service

import (
	"context"
	"testing"

	"starcrystal/server/internal/store"
)

// WRK-002: 福利 cur 榜 effectiveCurGold = curgold + downline。
func TestWelfareRankSync_EffectiveCurGold(t *testing.T) {
	ctx := context.Background()
	mem := newMemoryWelfareRankStore()
	sync := NewWelfareRankSync(mem, nil)

	bal := store.EconomyBalances{CurGold: 100, CurDownlineL1Contrib: 10, CurDownlineL2Contrib: 5}
	sync.Notify(ctx, "acc_a", WelfareChangedCurGold, bal)
	rows, err := sync.ListBoard(ctx, BoardWelfareGoldCur, 10)
	if err != nil || len(rows) != 1 || rows[0].Score != 115 {
		t.Fatalf("effective cur gold: rows=%v err=%v", rows, err)
	}
}

// WRK-003: cur 榜归零移除成员。
func TestWelfareRankSync_CurBoardZeroRemovesMember(t *testing.T) {
	ctx := context.Background()
	mem := newMemoryWelfareRankStore()
	sync := NewWelfareRankSync(mem, nil)

	sync.Notify(ctx, "acc_a", WelfareChangedCurGold, store.EconomyBalances{CurGold: 100})
	sync.Notify(ctx, "acc_a", WelfareChangedCurGold, store.EconomyBalances{})
	rows, err := sync.ListBoard(ctx, BoardWelfareGoldCur, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("cur board want empty after zero, got %+v", rows)
	}
}

// WRK-004: total 榜归零仍保留历史 downline 分。
func TestWelfareRankSync_TotalBoardZeroKeepsMember(t *testing.T) {
	ctx := context.Background()
	mem := newMemoryWelfareRankStore()
	sync := NewWelfareRankSync(mem, nil)

	sync.Notify(ctx, "acc_b", WelfareChangedTotalGold, store.EconomyBalances{TotalGold: 50, TotalDownlineL1Contrib: 10})
	sync.Notify(ctx, "acc_b", WelfareChangedTotalGold, store.EconomyBalances{TotalGold: 0, TotalDownlineL1Contrib: 10})

	rows, err := sync.ListBoard(ctx, BoardWelfareGoldTotal, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].Score != 10 {
		t.Fatalf("total board should keep historical downline score, got %+v", rows)
	}
}

// WRK-005: 六榜 batch notify（gold/down/up cur+total）。
func TestWelfareRankSync_BatchNotifySixBoards(t *testing.T) {
	ctx := context.Background()
	mem := newMemoryWelfareRankStore()
	sync := NewWelfareRankSync(mem, nil)

	bal := store.EconomyBalances{
		CurGold: 100, TotalGold: 200,
		CurDownlineL1Contrib: 10, TotalDownlineL1Contrib: 20,
		CurDirectInviterShare: 5, TotalDirectInviterShare: 15,
	}
	sync.Notify(ctx, "acc_c", WelfareChangedCurGold|WelfareChangedTotalGold|WelfareChangedInviteFields, bal)

	checks := map[string]float64{
		BoardWelfareGoldCur:          110,
		BoardWelfareDownContribCur:   10,
		BoardWelfareUpContribCur:     5,
		BoardWelfareGoldTotal:        220,
		BoardWelfareDownContribTotal: 20,
		BoardWelfareUpContribTotal:   15,
	}
	for board, want := range checks {
		rows, err := sync.ListBoard(ctx, board, 10)
		if err != nil || len(rows) != 1 || rows[0].Score != want {
			t.Fatalf("board %s want %.0f got %+v err=%v", board, want, rows, err)
		}
	}
}

// WRK-006: token cur 榜归零移除。
func TestWelfareRankSync_TokenCurZeroRemoves(t *testing.T) {
	ctx := context.Background()
	mem := newMemoryWelfareRankStore()
	sync := NewWelfareRankSync(mem, nil)

	sync.Notify(ctx, "acc_d", WelfareChangedCurToken, store.EconomyBalances{CurToken: 10})
	sync.Notify(ctx, "acc_d", WelfareChangedCurToken, store.EconomyBalances{CurToken: 0})
	rows, _ := sync.ListBoard(ctx, BoardWelfareTokenCur, 10)
	if len(rows) != 0 {
		t.Fatalf("token cur want empty, got %+v", rows)
	}
}
