package service

import (
	"context"
	"testing"
)

// TestGameFavoriteService_AddListRemove 内存 Store，不依赖 MySQL。对齐客户端 FAV-I-* / FAV-S-001。
func TestGameFavoriteService_AddListRemove(t *testing.T) {
	svc := NewGameFavoriteService(nil)
	ctx := context.Background()
	const accountID = "fav_svc_acc"
	const gameID = "g_fav_unit_1"

	if err := svc.Add(ctx, accountID, gameID); err != nil {
		t.Fatalf("add: %v", err)
	}
	fav, err := svc.IsFavorite(ctx, accountID, gameID)
	if err != nil || !fav {
		t.Fatalf("is favorite: fav=%v err=%v", fav, err)
	}

	ids, err := svc.ListGameIDs(ctx, accountID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	found := false
	for _, id := range ids {
		if id == gameID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("gameId not in list: %v", ids)
	}

	set, err := svc.FavoriteSet(ctx, accountID)
	if err != nil || set[gameID] != struct{}{} {
		t.Fatalf("favorite set: %+v err=%v", set, err)
	}

	if err := svc.Remove(ctx, accountID, gameID); err != nil {
		t.Fatalf("remove: %v", err)
	}
	fav, err = svc.IsFavorite(ctx, accountID, gameID)
	if err != nil || fav {
		t.Fatalf("after remove: fav=%v err=%v", fav, err)
	}
}

func TestGameFavoriteService_Add_EmptyGameID(t *testing.T) {
	svc := NewGameFavoriteService(nil)
	err := svc.Add(context.Background(), "acc", "  ")
	if err != ErrGameFavoriteEmptyGameID {
		t.Fatalf("expected ErrGameFavoriteEmptyGameID, got %v", err)
	}
}
