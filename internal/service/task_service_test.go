package service

import (
	"context"
	"testing"
)

func TestTaskService_Play60sProgress(t *testing.T) {
	svc := NewTaskService(NewMemoryTaskStore(), nil, nil)
	ctx := context.Background()
	acc := "guest_play60"

	_ = svc.OnRankActivity(ctx, acc, "g1", 65)
	data, err := svc.GetWelfare(ctx, acc)
	if err != nil {
		t.Fatal(err)
	}
	var play *WelfareTaskItem
	for i := range data.Tasks {
		if data.Tasks[i].TaskID == "play_daily_60s" {
			play = &data.Tasks[i]
			break
		}
	}
	if play == nil || play.Status != TaskStatusClaimable {
		t.Fatalf("play_daily_60s want claimable got %+v", play)
	}
}

func TestTaskService_Signin7dClaimable(t *testing.T) {
	svc := NewTaskService(NewMemoryTaskStore(), nil, nil)
	ctx := context.Background()
	data, err := svc.GetWelfare(ctx, "guest_s7")
	if err != nil {
		t.Fatal(err)
	}
	if !data.Signin7d.CanClaim {
		t.Fatalf("signin7d want canClaim")
	}
	for i := range data.Tasks {
		if data.Tasks[i].TaskID == "signin_7d" && data.Tasks[i].Status != TaskStatusClaimable {
			t.Fatalf("signin_7d status=%s", data.Tasks[i].Status)
		}
	}
}

func TestTaskService_DailyFreeClaimable(t *testing.T) {
	svc := NewTaskService(NewMemoryTaskStore(), nil, nil)
	ctx := context.Background()
	data, err := svc.GetWelfare(ctx, "guest_free")
	if err != nil {
		t.Fatal(err)
	}
	for i := range data.Tasks {
		if data.Tasks[i].TaskID == "daily_free_claim" && data.Tasks[i].Status != TaskStatusClaimable {
			t.Fatalf("daily_free_claim status=%s", data.Tasks[i].Status)
		}
	}
}
