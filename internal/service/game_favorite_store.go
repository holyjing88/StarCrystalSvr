package service

import (
	"context"
	"errors"
	"strings"
	"sync"
)

var ErrGameFavoriteEmptyGameID = errors.New("gameId is required")

// GameFavoriteStore persists per-account favorite game ids.
type GameFavoriteStore interface {
	List(ctx context.Context, accountID string) ([]string, error)
	Add(ctx context.Context, accountID, gameID string) error
	Remove(ctx context.Context, accountID, gameID string) error
	IsFavorite(ctx context.Context, accountID, gameID string) (bool, error)
}

type memoryGameFavoriteStore struct {
	mu   sync.RWMutex
	data map[string]map[string]struct{}
}

func NewMemoryGameFavoriteStore() GameFavoriteStore {
	return &memoryGameFavoriteStore{data: make(map[string]map[string]struct{})}
}

func normalizeGameFavoriteID(gameID string) (string, error) {
	id := strings.TrimSpace(gameID)
	if id == "" {
		return "", ErrGameFavoriteEmptyGameID
	}
	return id, nil
}

func (s *memoryGameFavoriteStore) bucket(accountID string) map[string]struct{} {
	if s.data[accountID] == nil {
		s.data[accountID] = make(map[string]struct{})
	}
	return s.data[accountID]
}

func (s *memoryGameFavoriteStore) List(_ context.Context, accountID string) ([]string, error) {
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return nil, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	b := s.data[accountID]
	if len(b) == 0 {
		return []string{}, nil
	}
	out := make([]string, 0, len(b))
	for id := range b {
		out = append(out, id)
	}
	return out, nil
}

func (s *memoryGameFavoriteStore) Add(_ context.Context, accountID, gameID string) error {
	id, err := normalizeGameFavoriteID(gameID)
	if err != nil {
		return err
	}
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return errors.New("accountId is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bucket(accountID)[id] = struct{}{}
	return nil
}

func (s *memoryGameFavoriteStore) Remove(_ context.Context, accountID, gameID string) error {
	id, err := normalizeGameFavoriteID(gameID)
	if err != nil {
		return err
	}
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return errors.New("accountId is required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.bucket(accountID), id)
	return nil
}

func (s *memoryGameFavoriteStore) IsFavorite(_ context.Context, accountID, gameID string) (bool, error) {
	id, err := normalizeGameFavoriteID(gameID)
	if err != nil {
		return false, err
	}
	accountID = strings.TrimSpace(accountID)
	if accountID == "" {
		return false, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.data[accountID][id]
	return ok, nil
}
