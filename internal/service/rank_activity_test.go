package service

import (
	"context"
	"testing"
)

func TestReportActivityPlay_AccumulatesByAccountID(t *testing.T) {
	games := NewGameService()
	svc := NewRankService(games, RankRedisConfig{})
	ctx := context.Background()
	aid := "guest_activity_test"

	w1, score1, err := svc.ReportActivityPlay(ctx, aid, 30)
	if err != nil {
		t.Fatal(err)
	}
	if score1 < 30 {
		t.Fatalf("score1=%d want >=30", score1)
	}

	_, score2, err := svc.ReportActivityPlay(ctx, aid, 20)
	if err != nil {
		t.Fatal(err)
	}
	if score2 < 50 {
		t.Fatalf("score2=%d want >=50", score2)
	}

	week, rows, err := svc.ListActivity(ctx, w1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if week == "" {
		t.Fatal("empty week")
	}
	found := false
	for _, row := range rows {
		if row.AccountID == aid && row.PlayCount >= 50 {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("account not on board: %+v", rows)
	}

	rank, score, onBoard, err := svc.MemberActivityRank(ctx, w1, aid)
	if err != nil {
		t.Fatal(err)
	}
	if !onBoard || rank < 1 || score < 50 {
		t.Fatalf("member rank: rank=%d score=%d onBoard=%v", rank, score, onBoard)
	}
}
