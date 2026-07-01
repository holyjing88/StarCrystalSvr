package api

import (
	"net/http"
	"testing"

	"starcrystal/server/internal/service"
)

func TestGameFavorite_AddListRemove(t *testing.T) {
	srv := httptestNewServer(t, NewServer(service.RankRedisConfig{}).Handler())

	st, env := doAPI(t, srv, http.MethodPost, "/api/v1/auth/guest", nil,
		`{"guestKey":"fav-test-guest","deviceId":"fav-test-device"}`)
	if st != http.StatusOK || env.Code != 0 {
		t.Fatalf("guest login status=%d code=%d", st, env.Code)
	}
	var guest struct {
		AccessToken string `json:"accessToken"`
	}
	decodeData(t, env.Data, &guest)
	if guest.AccessToken == "" {
		t.Fatal("empty access token")
	}

	hdr := http.Header{}
	hdr.Set("Authorization", "Bearer "+guest.AccessToken)
	gameID := "test_game_fav_1"

	st, env = doAPI(t, srv, http.MethodPost, "/api/v1/games/favorite", hdr, `{"gameId":"`+gameID+`"}`)
	if st != http.StatusOK || env.Code != 0 {
		t.Fatalf("add favorite status=%d code=%d msg=%q", st, env.Code, env.Message)
	}

	st, env = doAPI(t, srv, http.MethodGet, "/api/v1/games/favorites", hdr, "")
	if st != http.StatusOK || env.Code != 0 {
		t.Fatalf("list favorites status=%d code=%d", st, env.Code)
	}
	var list GameFavoritesListData
	decodeData(t, env.Data, &list)
	found := false
	for _, id := range list.GameIDs {
		if id == gameID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("gameId not in favorites: %+v", list.GameIDs)
	}

	st, env = doAPI(t, srv, http.MethodDelete, "/api/v1/games/favorite?gameId="+gameID, hdr, "")
	if st != http.StatusOK || env.Code != 0 {
		t.Fatalf("remove favorite status=%d code=%d", st, env.Code)
	}
}
