package antifraud

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// AdsGate 进程内频控与 slot 白名单（多实例部署请改 Redis 等共享存储）。
type AdsGate struct {
	startAcc       *minuteCounter
	startIP        *minuteCounter
	completeAcc    *minuteCounter
	completeIP     *minuteCounter
	startPerAcc    int
	startPerIP     int
	completePerAcc int
	completePerIP  int
	allowedSlots   map[string]struct{}
}

type minuteCounter struct {
	mu   sync.Mutex
	data map[string]*windowSlot
}

type windowSlot struct {
	minuteEpoch int64
	n           int
}

func newMinuteCounter() *minuteCounter {
	return &minuteCounter{data: make(map[string]*windowSlot)}
}

// NewAdsGateFromEnv 从环境变量加载；限流为 0 或负数表示不限制该项。
func NewAdsGateFromEnv() *AdsGate {
	g := &AdsGate{
		startAcc:       newMinuteCounter(),
		startIP:        newMinuteCounter(),
		completeAcc:    newMinuteCounter(),
		completeIP:     newMinuteCounter(),
		startPerAcc:    intEnv("AD_START_PER_MIN_ACCOUNT", 8),
		startPerIP:     intEnv("AD_START_PER_MIN_IP", 40),
		completePerAcc: intEnv("AD_COMPLETE_PER_MIN_ACCOUNT", 8),
		completePerIP:  intEnv("AD_COMPLETE_PER_MIN_IP", 50),
	}
	raw := strings.TrimSpace(os.Getenv("AD_SLOT_ALLOWLIST"))
	if raw != "" {
		g.allowedSlots = make(map[string]struct{})
		for _, p := range strings.Split(raw, ",") {
			k := strings.ToLower(strings.TrimSpace(p))
			if k != "" {
				g.allowedSlots[k] = struct{}{}
			}
		}
	}
	return g
}

func intEnv(k string, def int) int {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func (c *minuteCounter) try(key string, limit int) bool {
	if limit <= 0 || key == "" {
		return true
	}
	now := time.Now().Unix() / 60
	c.mu.Lock()
	defer c.mu.Unlock()
	w := c.data[key]
	if w == nil {
		w = &windowSlot{minuteEpoch: now, n: 0}
		c.data[key] = w
	}
	if w.minuteEpoch != now {
		w.minuteEpoch = now
		w.n = 0
	}
	if w.n >= limit {
		return false
	}
	w.n++
	return true
}

// CheckSlotValid 若配置了白名单则校验 slot（小写比较）。
func (g *AdsGate) CheckSlotValid(slot string) error {
	if g == nil || len(g.allowedSlots) == 0 {
		return nil
	}
	s := strings.ToLower(strings.TrimSpace(slot))
	if s == "" {
		return ErrSlotNotAllowed
	}
	if _, ok := g.allowedSlots[s]; !ok {
		return ErrSlotNotAllowed
	}
	return nil
}

// AllowStart 在已通过鉴权后调用；被拒绝时返回 ErrRateLimitStart。
func (g *AdsGate) AllowStart(accountID, clientIP string) error {
	if g == nil {
		return nil
	}
	if !g.startAcc.try("a:"+strings.TrimSpace(accountID), g.startPerAcc) {
		return ErrRateLimitStart
	}
	if ip := strings.TrimSpace(clientIP); ip != "" && ip != "127.0.0.1" && ip != "::1" {
		if !g.startIP.try("ip:"+ip, g.startPerIP) {
			return ErrRateLimitStart
		}
	}
	return nil
}

// AllowComplete 在已通过鉴权后调用。
func (g *AdsGate) AllowComplete(accountID, clientIP string) error {
	if g == nil {
		return nil
	}
	if !g.completeAcc.try("a:"+strings.TrimSpace(accountID), g.completePerAcc) {
		return ErrRateLimitComplete
	}
	if ip := strings.TrimSpace(clientIP); ip != "" && ip != "127.0.0.1" && ip != "::1" {
		if !g.completeIP.try("ip:"+ip, g.completePerIP) {
			return ErrRateLimitComplete
		}
	}
	return nil
}
