package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"starcrystal/server/internal/config"
)

var (
	ErrIdipLoginFailed    = errors.New("idip login failed")
	ErrIdipLoginRateLimit = errors.New("idip login rate limited")
	ErrIdipSessionInvalid = errors.New("idip session invalid")
)

type IdipLoginResult struct {
	SessionToken           string    `json:"sessionToken"`
	ExpiresAt              time.Time `json:"expiresAt"`
	HeartbeatIntervalSec   int       `json:"heartbeatIntervalSec"`
	KickedUsername         string    `json:"-"`
}

type idipSessionSnapshot struct {
	Username          string    `json:"username"`
	ExpiresAt         time.Time `json:"expiresAt"`
	LastHeartbeatAt   time.Time `json:"lastHeartbeatAt"`
}

type idipSessionStore interface {
	getSession(ctx context.Context, token string) (idipSessionSnapshot, bool, error)
	setSession(ctx context.Context, token string, snap idipSessionSnapshot, ttl time.Duration) error
	deleteSession(ctx context.Context, token string) error
	getActiveToken(ctx context.Context, username string) (string, bool, error)
	setActiveToken(ctx context.Context, username, token string, ttl time.Duration) error
	deleteActiveToken(ctx context.Context, username string) error
	incrLoginFail(ctx context.Context, username, clientIP string, windowSec int) (int, error)
	clearLoginFail(ctx context.Context, username, clientIP string) error
}

type IdipSessionService struct {
	store idipSessionStore
	cfg   config.IdipConfig
}

func NewIdipSessionService(cfg config.IdipConfig, rankRedis RankRedisConfig) *IdipSessionService {
	normalizeIdipSessionCfg(&cfg)
	if addr := redisAddr(rankRedis); addr != "" {
		if st, err := newRedisIdipSessionStore(addr, rankRedis.Password, rankRedis.DB); err == nil {
			return &IdipSessionService{store: st, cfg: cfg}
		}
	}
	return &IdipSessionService{store: newMemoryIdipSessionStore(), cfg: cfg}
}

func NewIdipSessionServiceForTest(cfg config.IdipConfig) *IdipSessionService {
	normalizeIdipSessionCfg(&cfg)
	return &IdipSessionService{store: newMemoryIdipSessionStore(), cfg: cfg}
}

func normalizeIdipSessionCfg(cfg *config.IdipConfig) {
	if cfg.LoginFailMaxAttempts <= 0 {
		cfg.LoginFailMaxAttempts = 5
	}
	if cfg.LoginFailWindowSec <= 0 {
		cfg.LoginFailWindowSec = 60
	}
	if cfg.SessionTtlSec <= 0 {
		cfg.SessionTtlSec = 28800
	}
	if cfg.SessionHeartbeatIntervalSec <= 0 {
		cfg.SessionHeartbeatIntervalSec = 30
	}
	if cfg.SessionHeartbeatTimeoutSec <= 0 {
		cfg.SessionHeartbeatTimeoutSec = 30
	}
}

func redisAddr(file RankRedisConfig) string {
	if v := strings.TrimSpace(os.Getenv("REDIS_ADDR")); v != "" {
		return v
	}
	return strings.TrimSpace(file.Addr)
}

func (s *IdipSessionService) Config() config.IdipConfig { return s.cfg }

func (s *IdipSessionService) UpdateConfig(cfg config.IdipConfig) {
	normalizeIdipSessionCfg(&cfg)
	s.cfg = cfg
}

func (s *IdipSessionService) HeartbeatIntervalSec() int {
	return s.cfg.SessionHeartbeatIntervalSec
}

func (s *IdipSessionService) Login(ctx context.Context, username, password, clientIP string, verify func(config.IdipConfig, string, string) bool) (*IdipLoginResult, error) {
	cfg := s.cfg
	if verify != nil && !verify(cfg, username, password) {
		n, _ := s.store.incrLoginFail(ctx, username, clientIP, cfg.LoginFailWindowSec)
		if n >= cfg.LoginFailMaxAttempts {
			return nil, ErrIdipLoginRateLimit
		}
		return nil, ErrIdipLoginFailed
	}
	_ = s.store.clearLoginFail(ctx, username, clientIP)

	token, err := newSessionToken()
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	exp := now.Add(time.Duration(cfg.SessionTtlSec) * time.Second)
	snap := idipSessionSnapshot{Username: username, ExpiresAt: exp, LastHeartbeatAt: now}
	ttl := time.Duration(cfg.SessionTtlSec) * time.Second

	var kicked string
	if old, ok, _ := s.store.getActiveToken(ctx, username); ok && strings.TrimSpace(old) != "" && old != token {
		if oldSnap, ok2, _ := s.store.getSession(ctx, old); ok2 {
			kicked = oldSnap.Username
		}
		_ = s.store.deleteSession(ctx, old)
	}
	_ = s.store.setActiveToken(ctx, username, token, ttl)
	if err := s.store.setSession(ctx, token, snap, ttl); err != nil {
		return nil, err
	}
	return &IdipLoginResult{
		SessionToken:         token,
		ExpiresAt:            exp,
		HeartbeatIntervalSec: cfg.SessionHeartbeatIntervalSec,
		KickedUsername:       kicked,
	}, nil
}

func (s *IdipSessionService) Logout(ctx context.Context, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil
	}
	if snap, ok, _ := s.store.getSession(ctx, token); ok {
		_ = s.store.deleteActiveToken(ctx, snap.Username)
	}
	return s.store.deleteSession(ctx, token)
}

func (s *IdipSessionService) Heartbeat(ctx context.Context, token string) (time.Time, error) {
	snap, err := s.validateSnapshot(ctx, token)
	if err != nil {
		return time.Time{}, err
	}
	now := time.Now().UTC()
	snap.LastHeartbeatAt = now
	ttl := time.Duration(s.cfg.SessionTtlSec) * time.Second
	if err := s.store.setSession(ctx, token, snap, ttl); err != nil {
		return time.Time{}, err
	}
	return snap.ExpiresAt, nil
}

func (s *IdipSessionService) ValidateSession(ctx context.Context, token string) (string, error) {
	snap, err := s.validateSnapshot(ctx, token)
	if err != nil {
		return "", err
	}
	return snap.Username, nil
}

func (s *IdipSessionService) validateSnapshot(ctx context.Context, token string) (idipSessionSnapshot, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return idipSessionSnapshot{}, ErrIdipSessionInvalid
	}
	snap, ok, err := s.store.getSession(ctx, token)
	if err != nil {
		return idipSessionSnapshot{}, err
	}
	if !ok {
		return idipSessionSnapshot{}, ErrIdipSessionInvalid
	}
	now := time.Now().UTC()
	if now.After(snap.ExpiresAt) {
		return idipSessionSnapshot{}, ErrIdipSessionInvalid
	}
	if now.Sub(snap.LastHeartbeatAt) > time.Duration(s.cfg.SessionHeartbeatTimeoutSec)*time.Second {
		return idipSessionSnapshot{}, ErrIdipSessionInvalid
	}
	active, ok, _ := s.store.getActiveToken(ctx, snap.Username)
	if !ok || active != token {
		return idipSessionSnapshot{}, ErrIdipSessionInvalid
	}
	return snap, nil
}

func newSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// --- memory store ---

type memoryIdipSessionStore struct {
	mu         sync.Mutex
	sessions   map[string]idipSessionSnapshot
	activeUser map[string]string
	loginFail  map[string]int
}

func newMemoryIdipSessionStore() *memoryIdipSessionStore {
	return &memoryIdipSessionStore{
		sessions:   make(map[string]idipSessionSnapshot),
		activeUser: make(map[string]string),
		loginFail:  make(map[string]int),
	}
}

func (m *memoryIdipSessionStore) getSession(_ context.Context, token string) (idipSessionSnapshot, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s, ok := m.sessions[token]
	return s, ok, nil
}

func (m *memoryIdipSessionStore) setSession(_ context.Context, token string, snap idipSessionSnapshot, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sessions[token] = snap
	return nil
}

func (m *memoryIdipSessionStore) deleteSession(_ context.Context, token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.sessions, token)
	return nil
}

func (m *memoryIdipSessionStore) getActiveToken(_ context.Context, username string) (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.activeUser[strings.ToLower(username)]
	return t, ok, nil
}

func (m *memoryIdipSessionStore) setActiveToken(_ context.Context, username, token string, _ time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.activeUser[strings.ToLower(username)] = token
	return nil
}

func (m *memoryIdipSessionStore) deleteActiveToken(_ context.Context, username string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.activeUser, strings.ToLower(username))
	return nil
}

func (m *memoryIdipSessionStore) incrLoginFail(_ context.Context, username, clientIP string, windowSec int) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := loginFailKey(username, clientIP)
	m.loginFail[k]++
	return m.loginFail[k], nil
}

func (m *memoryIdipSessionStore) clearLoginFail(_ context.Context, username, clientIP string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.loginFail, loginFailKey(username, clientIP))
	return nil
}

func loginFailKey(username, clientIP string) string {
	return strings.ToLower(strings.TrimSpace(username)) + "|" + strings.TrimSpace(clientIP)
}

// --- redis store ---

type redisIdipSessionStore struct {
	rdb *redis.Client
}

func newRedisIdipSessionStore(addr, password string, db int) (*redisIdipSessionStore, error) {
	if db < 0 || db > 15 {
		db = 0
	}
	if d := strings.TrimSpace(os.Getenv("REDIS_DB")); d != "" {
		if v, err := strconv.Atoi(d); err == nil && v >= 0 && v < 16 {
			db = v
		}
	}
	if _, ok := os.LookupEnv("REDIS_PASSWORD"); ok {
		password = os.Getenv("REDIS_PASSWORD")
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr, Password: password, DB: db})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		return nil, err
	}
	return &redisIdipSessionStore{rdb: rdb}, nil
}

func (r *redisIdipSessionStore) sessionKey(token string) string { return "idip:session:" + token }
func (r *redisIdipSessionStore) activeKey(username string) string {
	return "idip:user-active:" + strings.ToLower(strings.TrimSpace(username))
}
func (r *redisIdipSessionStore) failKey(username, clientIP string) string {
	return "idip:login-fail:" + strings.ToLower(strings.TrimSpace(username)) + ":" + strings.TrimSpace(clientIP)
}

func (r *redisIdipSessionStore) getSession(ctx context.Context, token string) (idipSessionSnapshot, bool, error) {
	raw, err := r.rdb.Get(ctx, r.sessionKey(token)).Bytes()
	if errors.Is(err, redis.Nil) {
		return idipSessionSnapshot{}, false, nil
	}
	if err != nil {
		return idipSessionSnapshot{}, false, err
	}
	var snap idipSessionSnapshot
	if err := json.Unmarshal(raw, &snap); err != nil {
		return idipSessionSnapshot{}, false, err
	}
	return snap, true, nil
}

func (r *redisIdipSessionStore) setSession(ctx context.Context, token string, snap idipSessionSnapshot, ttl time.Duration) error {
	raw, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	return r.rdb.Set(ctx, r.sessionKey(token), raw, ttl).Err()
}

func (r *redisIdipSessionStore) deleteSession(ctx context.Context, token string) error {
	return r.rdb.Del(ctx, r.sessionKey(token)).Err()
}

func (r *redisIdipSessionStore) getActiveToken(ctx context.Context, username string) (string, bool, error) {
	s, err := r.rdb.Get(ctx, r.activeKey(username)).Result()
	if errors.Is(err, redis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return s, true, nil
}

func (r *redisIdipSessionStore) setActiveToken(ctx context.Context, username, token string, ttl time.Duration) error {
	return r.rdb.Set(ctx, r.activeKey(username), token, ttl).Err()
}

func (r *redisIdipSessionStore) deleteActiveToken(ctx context.Context, username string) error {
	return r.rdb.Del(ctx, r.activeKey(username)).Err()
}

func (r *redisIdipSessionStore) incrLoginFail(ctx context.Context, username, clientIP string, windowSec int) (int, error) {
	key := r.failKey(username, clientIP)
	n, err := r.rdb.Incr(ctx, key).Result()
	if err != nil {
		return 0, err
	}
	if n == 1 {
		_ = r.rdb.Expire(ctx, key, time.Duration(windowSec)*time.Second).Err()
	}
	return int(n), nil
}

func (r *redisIdipSessionStore) clearLoginFail(ctx context.Context, username, clientIP string) error {
	return r.rdb.Del(ctx, r.failKey(username, clientIP)).Err()
}
