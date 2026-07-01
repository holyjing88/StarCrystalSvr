package service

import (
	"context"
	"sync"
	"time"
)

// Signin7dState persisted sign-in chain (Asia/Shanghai calendar).
type Signin7dState struct {
	Chain         int
	LastClaimYmd  int
	RoundStartYmd int
}

// TaskProgressStore hot progress for welfare tasks.
type TaskProgressStore interface {
	GetActiveSecDay(ctx context.Context, ymd int, accountID string) (int64, error)
	IncrActiveSecDay(ctx context.Context, ymd int, accountID string, delta int64) (int64, error)
	AddGameSecDay(ctx context.Context, ymd int, accountID, gameID string, delta int64) (gameTotal int64, err error)
	CountDistinctGames30Day(ctx context.Context, ymd int, accountID string) (int, error)

	IncrAdsCompleteDay(ctx context.Context, ymd int, accountID string) (int64, error)
	GetAdsCompleteDay(ctx context.Context, ymd int, accountID string) (int64, error)

	IncrInviteValidDay(ctx context.Context, ymd int, inviterAccountID string) (int64, error)
	GetInviteValidDay(ctx context.Context, ymd int, inviterAccountID string) (int64, error)

	GetSignin7d(ctx context.Context, accountID string) (Signin7dState, error)
	SetSignin7d(ctx context.Context, accountID string, st Signin7dState) error

	IsClaimed(ctx context.Context, claimKey string) (bool, error)
	MarkClaimed(ctx context.Context, claimKey string) error

	SetAdProof(ctx context.Context, accountID string, until time.Time) error
	HasAdProof(ctx context.Context, accountID string, now time.Time) (bool, error)

	GetStreakActiveDays(ctx context.Context, accountID string) (int, error)
	RecordActiveDayForStreak(ctx context.Context, ymd int, accountID string) error

	GetActiveSecWeek(ctx context.Context, weekID, accountID string) (int64, error)
	IncrActiveSecWeek(ctx context.Context, weekID, accountID string, delta int64) (int64, error)

	IncrPlayOpensLifetime(ctx context.Context, accountID string) (int64, error)
	GetPlayOpensLifetime(ctx context.Context, accountID string) (int64, error)

	GetGameSecDay(ctx context.Context, ymd int, accountID, gameID string) (int64, error)

	IncrGoldEarnedDay(ctx context.Context, ymd int, accountID string, delta float64) (float64, error)
	GetGoldEarnedDay(ctx context.Context, ymd int, accountID string) (float64, error)
	IncrGoldEarnedLifetime(ctx context.Context, accountID string, delta float64) (float64, error)
	GetGoldEarnedLifetime(ctx context.Context, accountID string) (float64, error)

	MarkPageViewDay(ctx context.Context, ymd int, accountID, page string) error
	HasPageViewDay(ctx context.Context, ymd int, accountID, page string) (bool, error)

	IncrAdsCompleteWeek(ctx context.Context, weekID, accountID string) (int64, error)
	GetAdsCompleteWeek(ctx context.Context, weekID, accountID string) (int64, error)
	IncrAdsCompleteLifetime(ctx context.Context, accountID string) (int64, error)
	GetAdsCompleteLifetime(ctx context.Context, accountID string) (int64, error)

	IncrActiveSecLifetime(ctx context.Context, accountID string, delta int64) (int64, error)
	GetActiveSecLifetime(ctx context.Context, accountID string) (int64, error)

	GetStreakSigninDays(ctx context.Context, accountID string) (int, error)
	RecordSigninClaimForStreak(ctx context.Context, ymd int, accountID string) error

	GetSignin7dRounds(ctx context.Context, accountID string) (int, error)
	IncrSignin7dRounds(ctx context.Context, accountID string) (int, error)

	TouchLastSeenYmd(ctx context.Context, accountID string, ymd int) (prevYmd int, err error)
	GetOrInitFirstSeenYmd(ctx context.Context, accountID string, ymd int) (int, error)

	MarkShareSuccessDay(ctx context.Context, ymd int, accountID string) error
	HasShareSuccessDay(ctx context.Context, ymd int, accountID string) (bool, error)
}

type memoryTaskStore struct {
	mu                sync.Mutex
	activeSec         map[string]int64 // ymd:account
	gameSec           map[string]int64 // ymd:account:game
	games30           map[string]map[string]struct{}
	adsDay            map[string]int64
	inviteDay         map[string]int64
	signin            map[string]Signin7dState
	claimed           map[string]bool
	adProofUntil      map[string]time.Time
	streakYmd         map[string]int
	streakCount       map[string]int
	activeSecWeek     map[string]int64
	playOpensLife     map[string]int64
	goldEarnedDay     map[string]float64
	goldEarnedLife    map[string]float64
	pageViewDay       map[string]bool
	adsWeek           map[string]int64
	adsLife           map[string]int64
	activeSecLife     map[string]int64
	streakSigninYmd   map[string]int
	streakSigninCount map[string]int
	signinRounds      map[string]int
	lastSeenYmd       map[string]int
	firstSeenYmd      map[string]int
	shareDay          map[string]bool
}

func NewMemoryTaskStore() TaskProgressStore {
	return &memoryTaskStore{
		activeSec:         make(map[string]int64),
		gameSec:           make(map[string]int64),
		games30:           make(map[string]map[string]struct{}),
		adsDay:            make(map[string]int64),
		inviteDay:         make(map[string]int64),
		signin:            make(map[string]Signin7dState),
		claimed:           make(map[string]bool),
		adProofUntil:      make(map[string]time.Time),
		streakYmd:         make(map[string]int),
		streakCount:       make(map[string]int),
		activeSecWeek:     make(map[string]int64),
		playOpensLife:     make(map[string]int64),
		goldEarnedDay:     make(map[string]float64),
		goldEarnedLife:    make(map[string]float64),
		pageViewDay:       make(map[string]bool),
		adsWeek:           make(map[string]int64),
		adsLife:           make(map[string]int64),
		activeSecLife:     make(map[string]int64),
		streakSigninYmd:   make(map[string]int),
		streakSigninCount: make(map[string]int),
		signinRounds:      make(map[string]int),
		lastSeenYmd:       make(map[string]int),
		firstSeenYmd:      make(map[string]int),
		shareDay:          make(map[string]bool),
	}
}

func weekAccountKey(weekID, accountID string) string {
	return weekID + ":" + accountID
}

func dayAccountKey(ymd int, accountID string) string {
	return itoaYmd(ymd) + ":" + accountID
}

func itoaYmd(ymd int) string {
	if ymd <= 0 {
		return "0"
	}
	n := ymd
	var digits [8]byte
	i := 8
	for n > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}
	return string(digits[i:])
}

func (s *memoryTaskStore) GetActiveSecDay(_ context.Context, ymd int, accountID string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.activeSec[dayAccountKey(ymd, accountID)], nil
}

func (s *memoryTaskStore) IncrActiveSecDay(_ context.Context, ymd int, accountID string, delta int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := dayAccountKey(ymd, accountID)
	s.activeSec[k] += delta
	return s.activeSec[k], nil
}

func (s *memoryTaskStore) AddGameSecDay(_ context.Context, ymd int, accountID, gameID string, delta int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	gk := dayAccountKey(ymd, accountID) + ":" + gameID
	s.gameSec[gk] += delta
	total := s.gameSec[gk]
	if total >= 30 {
		dk := dayAccountKey(ymd, accountID)
		if s.games30[dk] == nil {
			s.games30[dk] = make(map[string]struct{})
		}
		s.games30[dk][gameID] = struct{}{}
	}
	return total, nil
}

func (s *memoryTaskStore) CountDistinctGames30Day(_ context.Context, ymd int, accountID string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.games30[dayAccountKey(ymd, accountID)]), nil
}

func (s *memoryTaskStore) IncrAdsCompleteDay(_ context.Context, ymd int, accountID string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := dayAccountKey(ymd, accountID)
	s.adsDay[k]++
	return s.adsDay[k], nil
}

func (s *memoryTaskStore) GetAdsCompleteDay(_ context.Context, ymd int, accountID string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.adsDay[dayAccountKey(ymd, accountID)], nil
}

func (s *memoryTaskStore) IncrInviteValidDay(_ context.Context, ymd int, inviterAccountID string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := dayAccountKey(ymd, inviterAccountID)
	s.inviteDay[k]++
	return s.inviteDay[k], nil
}

func (s *memoryTaskStore) GetInviteValidDay(_ context.Context, ymd int, inviterAccountID string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inviteDay[dayAccountKey(ymd, inviterAccountID)], nil
}

func (s *memoryTaskStore) GetSignin7d(_ context.Context, accountID string) (Signin7dState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.signin[accountID], nil
}

func (s *memoryTaskStore) SetSignin7d(_ context.Context, accountID string, st Signin7dState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.signin[accountID] = st
	return nil
}

func (s *memoryTaskStore) IsClaimed(_ context.Context, claimKey string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.claimed[claimKey], nil
}

func (s *memoryTaskStore) MarkClaimed(_ context.Context, claimKey string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.claimed[claimKey] = true
	return nil
}

func (s *memoryTaskStore) SetAdProof(_ context.Context, accountID string, until time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.adProofUntil[accountID] = until
	return nil
}

func (s *memoryTaskStore) HasAdProof(_ context.Context, accountID string, now time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	until, ok := s.adProofUntil[accountID]
	return ok && now.Before(until), nil
}

func (s *memoryTaskStore) GetStreakActiveDays(_ context.Context, accountID string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.streakCount[accountID], nil
}

func (s *memoryTaskStore) RecordActiveDayForStreak(_ context.Context, ymd int, accountID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	last := s.streakYmd[accountID]
	if last == ymd {
		return nil
	}
	gap := calendarDaysBetween(last, ymd)
	if last == 0 || gap == 1 {
		s.streakCount[accountID]++
	} else if gap > 1 {
		s.streakCount[accountID] = 1
	}
	s.streakYmd[accountID] = ymd
	return nil
}

func (s *memoryTaskStore) GetActiveSecWeek(_ context.Context, weekID, accountID string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.activeSecWeek[weekAccountKey(weekID, accountID)], nil
}

func (s *memoryTaskStore) IncrActiveSecWeek(_ context.Context, weekID, accountID string, delta int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := weekAccountKey(weekID, accountID)
	s.activeSecWeek[k] += delta
	return s.activeSecWeek[k], nil
}

func (s *memoryTaskStore) IncrPlayOpensLifetime(_ context.Context, accountID string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.playOpensLife[accountID]++
	return s.playOpensLife[accountID], nil
}

func (s *memoryTaskStore) GetPlayOpensLifetime(_ context.Context, accountID string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.playOpensLife[accountID], nil
}

func (s *memoryTaskStore) GetGameSecDay(_ context.Context, ymd int, accountID, gameID string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.gameSec[dayAccountKey(ymd, accountID)+":"+gameID], nil
}

func (s *memoryTaskStore) IncrGoldEarnedDay(_ context.Context, ymd int, accountID string, delta float64) (float64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := dayAccountKey(ymd, accountID)
	s.goldEarnedDay[k] += delta
	return s.goldEarnedDay[k], nil
}

func (s *memoryTaskStore) GetGoldEarnedDay(_ context.Context, ymd int, accountID string) (float64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.goldEarnedDay[dayAccountKey(ymd, accountID)], nil
}

func (s *memoryTaskStore) IncrGoldEarnedLifetime(_ context.Context, accountID string, delta float64) (float64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.goldEarnedLife[accountID] += delta
	return s.goldEarnedLife[accountID], nil
}

func (s *memoryTaskStore) GetGoldEarnedLifetime(_ context.Context, accountID string) (float64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.goldEarnedLife[accountID], nil
}

func (s *memoryTaskStore) MarkPageViewDay(_ context.Context, ymd int, accountID, page string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pageViewDay[dayAccountKey(ymd, accountID)+":"+page] = true
	return nil
}

func (s *memoryTaskStore) HasPageViewDay(_ context.Context, ymd int, accountID, page string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pageViewDay[dayAccountKey(ymd, accountID)+":"+page], nil
}

func (s *memoryTaskStore) IncrAdsCompleteWeek(_ context.Context, weekID, accountID string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	k := weekAccountKey(weekID, accountID)
	s.adsWeek[k]++
	return s.adsWeek[k], nil
}

func (s *memoryTaskStore) GetAdsCompleteWeek(_ context.Context, weekID, accountID string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.adsWeek[weekAccountKey(weekID, accountID)], nil
}

func (s *memoryTaskStore) IncrAdsCompleteLifetime(_ context.Context, accountID string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.adsLife[accountID]++
	return s.adsLife[accountID], nil
}

func (s *memoryTaskStore) GetAdsCompleteLifetime(_ context.Context, accountID string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.adsLife[accountID], nil
}

func (s *memoryTaskStore) IncrActiveSecLifetime(_ context.Context, accountID string, delta int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.activeSecLife[accountID] += delta
	return s.activeSecLife[accountID], nil
}

func (s *memoryTaskStore) GetActiveSecLifetime(_ context.Context, accountID string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.activeSecLife[accountID], nil
}

func (s *memoryTaskStore) GetStreakSigninDays(_ context.Context, accountID string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.streakSigninCount[accountID], nil
}

func (s *memoryTaskStore) RecordSigninClaimForStreak(_ context.Context, ymd int, accountID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	last := s.streakSigninYmd[accountID]
	if last == ymd {
		return nil
	}
	gap := calendarDaysBetween(last, ymd)
	if last == 0 || gap == 1 {
		s.streakSigninCount[accountID]++
	} else if gap > 1 {
		s.streakSigninCount[accountID] = 1
	}
	s.streakSigninYmd[accountID] = ymd
	return nil
}

func (s *memoryTaskStore) GetSignin7dRounds(_ context.Context, accountID string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.signinRounds[accountID], nil
}

func (s *memoryTaskStore) IncrSignin7dRounds(_ context.Context, accountID string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.signinRounds[accountID]++
	return s.signinRounds[accountID], nil
}

func (s *memoryTaskStore) TouchLastSeenYmd(_ context.Context, accountID string, ymd int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.lastSeenYmd[accountID]
	s.lastSeenYmd[accountID] = ymd
	return prev, nil
}

func (s *memoryTaskStore) MarkShareSuccessDay(_ context.Context, ymd int, accountID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.shareDay[dayAccountKey(ymd, accountID)] = true
	return nil
}

func (s *memoryTaskStore) HasShareSuccessDay(_ context.Context, ymd int, accountID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.shareDay[dayAccountKey(ymd, accountID)], nil
}

func (s *memoryTaskStore) GetOrInitFirstSeenYmd(_ context.Context, accountID string, ymd int) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if v, ok := s.firstSeenYmd[accountID]; ok && v > 0 {
		return v, nil
	}
	s.firstSeenYmd[accountID] = ymd
	return ymd, nil
}
