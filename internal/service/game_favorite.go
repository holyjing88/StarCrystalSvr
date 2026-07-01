package service

import "context"

// GameFavoriteService user game favorites (per account).
type GameFavoriteService struct {
	store GameFavoriteStore
}

func NewGameFavoriteService(store GameFavoriteStore) *GameFavoriteService {
	if store == nil {
		store = NewMemoryGameFavoriteStore()
	}
	return &GameFavoriteService{store: store}
}

func (s *GameFavoriteService) ListGameIDs(ctx context.Context, accountID string) ([]string, error) {
	return s.store.List(ctx, accountID)
}

func (s *GameFavoriteService) Add(ctx context.Context, accountID, gameID string) error {
	return s.store.Add(ctx, accountID, gameID)
}

func (s *GameFavoriteService) Remove(ctx context.Context, accountID, gameID string) error {
	return s.store.Remove(ctx, accountID, gameID)
}

func (s *GameFavoriteService) IsFavorite(ctx context.Context, accountID, gameID string) (bool, error) {
	return s.store.IsFavorite(ctx, accountID, gameID)
}

func (s *GameFavoriteService) FavoriteSet(ctx context.Context, accountID string) (map[string]struct{}, error) {
	ids, err := s.ListGameIDs(ctx, accountID)
	if err != nil {
		return nil, err
	}
	set := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}
	return set, nil
}
