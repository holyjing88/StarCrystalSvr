package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"starcrystal/server/internal/logger"
	"starcrystal/server/internal/service"
)

type Server struct {
	mux                 *http.ServeMux
	gameService         *service.GameService
	authService         *service.AuthService
	channelTextService  *service.ChannelTextService
	rankService         *service.RankService
	economy             *service.EconomyBundle
	idipSessions        *service.IdipSessionService
	taskService         *service.TaskService
	gameFavoriteService *service.GameFavoriteService
	requireHTTPS        bool
}

func NewServer(rankRedis service.RankRedisConfig) *Server {
	games := service.NewGameService()
	auth := service.NewAuthService()
	var economy *service.EconomyBundle
	if auth.PlayerRepository() != nil {
		economy = service.NewEconomyBundle(auth.PlayerRepository(), rankRedis)
		auth.AttachEconomy(economy.GoldLedger, economy.WelfareRank, economy.GoldRedis)
	}
	taskStore := service.NewMemoryTaskStore()
	service.ReloadTaskRegistry("")
	var taskSvc *service.TaskService
	if economy != nil && economy.GoldLedger != nil {
		taskSvc = service.NewTaskService(taskStore, economy.GoldLedger, auth.PlayerRepository())
	} else {
		taskSvc = service.NewTaskService(taskStore, nil, auth.PlayerRepository())
	}
	var idipSessions *service.IdipSessionService
	if economy != nil {
		idipSessions = service.NewIdipSessionService(economy.Config.Idip, rankRedis)
	}
	s := &Server{
		mux:                 http.NewServeMux(),
		gameService:         games,
		authService:         auth,
		channelTextService:  service.NewChannelTextService(),
		rankService:         service.NewRankService(games, rankRedis),
		economy:             economy,
		idipSessions:        idipSessions,
		taskService:         taskSvc,
		gameFavoriteService: service.NewGameFavoriteService(nil),
	}
	auth.AttachTaskInviteHook(func(ctx context.Context, inviterAccountID string) {
		_ = s.taskService.OnInviteRegistered(ctx, inviterAccountID)
	})
	s.registerRoutes()
	return s
}

// SetRequireHTTPS enables middleware that rejects non-HTTPS requests (see RequestIsHTTPS).
func (s *Server) SetRequireHTTPS(v bool) {
	s.requireHTTPS = v
}

// StartEconomyBackground starts monthly settlement scheduler when MySQL economy is enabled.
func (s *Server) StartEconomyBackground(ctx context.Context) {
	if s.economy != nil && s.economy.Settlement != nil {
		go s.economy.Settlement.StartSettlementScheduler(ctx)
	}
}

func (s *Server) Handler() http.Handler {
	return s.corsMiddleware(s.requireHTTPSMiddleware(s.loggingMiddleware(s.mux)))
}

// ProbeRankBackend 启动时 PING Redis（若已配置）或记录使用内存排行后端。
func (s *Server) ProbeRankBackend(ctx context.Context) {
	s.rankService.LogStartupConnectivity(ctx)
}

// registerRoutes 注册 HTTP 路由。统一账号注册/登录：`POST /api/v1/auth/register`（验证码注册）+ `POST /api/v1/auth/login`（account+密码）。
func (s *Server) registerRoutes() {
	// --- 站点与运维 ---
	s.mux.HandleFunc("GET /", s.handleRoot)               // HTML 占位首页（可选）
	s.mux.HandleFunc("GET /favicon.ico", s.handleFavicon) // 避免浏览器刷屏 404
	s.mux.HandleFunc("GET /healthz", s.handleHealth)      // 存活探针

	// --- 配置与游戏列表（query: appVersion, platform）---
	s.mux.HandleFunc("GET /api/v1/games", s.handleListGames)
	s.mux.HandleFunc("GET /api/v1/games/favorites", s.handleListGameFavorites)
	s.mux.HandleFunc("POST /api/v1/games/favorite", s.handleAddGameFavorite)
	s.mux.HandleFunc("DELETE /api/v1/games/favorite", s.handleRemoveGameFavorite)
	s.mux.HandleFunc("POST /api/v1/i18n/channel/texts", s.handleChannelTexts)

	// --- 排行榜（人气榜：Redis sorted set；未配 REDIS_ADDR 时为进程内内存）---
	s.mux.HandleFunc("POST /api/v1/rank/play", s.handleRankPlay)
	s.mux.HandleFunc("POST /api/v1/rank/activity", s.handleRankActivity)
	s.mux.HandleFunc("GET /api/v1/rank", s.handleRankList)
	s.mux.HandleFunc("POST /api/v1/welfare/exchange", s.handleWelfareExchangeDeprecated)
	s.registerIdipRoutes()

	// --- 认证：第三方 OAuth（Google idToken / Facebook accessToken）---
	s.mux.HandleFunc("POST /api/v1/auth/oauth", s.handleAuthOAuth)

	// --- 认证：统一发验证码（purpose=register|password_reset；channel=phone|email 可选校验；与 POST /api/v1/auth/register / 找回密码配套）---
	s.mux.HandleFunc("POST /api/v1/auth/sendverificationcode", s.handleAuthSendVerificationCode)

	// --- 认证：邮箱或手机 + 验证码注册；account+密码登录（可带 inviteCode）---
	s.mux.HandleFunc("POST /api/v1/auth/register", s.handleAuthRegisterByCode)
	s.mux.HandleFunc("POST /api/v1/auth/login", s.handleAuthLoginByAccount)

	// --- 认证：找回密码确认新密码 ---
	s.mux.HandleFunc("POST /api/v1/auth/password/reset/confirm", s.handleAuthPasswordResetConfirm)

	// --- 认证：访客（guestKey + deviceId，可选 fingerprint）---
	s.mux.HandleFunc("POST /api/v1/auth/guest", s.handleAuthGuest)
	s.mux.HandleFunc("POST /api/v1/auth/guest/verify", s.handleAuthGuestVerify)

	// --- 认证：已登录态（Bearer）改昵称、拉资料、GM 调指标、团队邀请树 ---
	s.mux.HandleFunc("POST /api/v1/auth/account/delete", s.handleAuthAccountDelete)
	s.mux.HandleFunc("POST /api/v1/auth/profile/name", s.handleAuthProfileName)
	s.mux.HandleFunc("GET /api/v1/auth/me", s.handleAuthMe)
	s.mux.HandleFunc("POST /api/v1/auth/gm/metrics", s.handleAuthGmMetrics)
	s.mux.HandleFunc("POST /api/v1/auth/gm/fastforward/silent", s.handleAuthGmFastForwardSilent)
	s.mux.HandleFunc("POST /api/v1/auth/gm/restore/silent", s.handleAuthGmRestoreSilent)
	s.mux.HandleFunc("GET /api/v1/auth/my/team", s.handleAuthMyTeam)
	s.mux.HandleFunc("POST /api/v1/auth/heartbeat", s.handleAuthHeartbeat)
	s.mux.HandleFunc("POST /api/v1/auth/invite-contrib/ack-notify", s.handleAuthInviteContribAckNotify)

	s.registerPublishStaticRoutes() // H5：/h5/*；旧 /assets/h5/* 与 /assets/* 不在 API 提供

	// --- 钱包（占位：余额/流水；不含 withdraw，见 welfare 兑换礼品）---
	s.mux.HandleFunc("GET /api/v1/wallet/balance", s.handleWalletBalance)
	s.mux.HandleFunc("GET /api/v1/wallet/ledger", s.handleWalletLedger)

	// --- 福利 · 兑换 Token 为礼品（占位/正式实现；禁止 withdraw 路径，商店敏感词）---
	s.mux.HandleFunc("POST /api/v1/welfare/redeem-token-gift", s.handleWelfareRedeemTokenGift)
	s.mux.HandleFunc("GET /api/v1/welfare/redeem-gift/", s.handleWelfareRedeemGiftQuery) // 后缀 redeemId

	// --- 激励广告：开始观看 / 完成核销；callback 为渠道回调前缀 ---
	s.mux.HandleFunc("POST /api/v1/ads/start", s.handleAdStart)
	s.mux.HandleFunc("POST /api/v1/ads/complete", s.handleAdComplete)
	s.mux.HandleFunc("POST /api/v1/ads/callback/", s.handleAdCallback)

	s.mux.HandleFunc("GET /api/v1/tasks/welfare", s.handleTasksWelfare)
	s.mux.HandleFunc("POST /api/v1/tasks/claim", s.handleTasksClaim)
	s.mux.HandleFunc("POST /api/v1/tasks/report", s.handleTasksReport)
}

func (s *Server) registerPublishStaticRoutes() {
	if service.PublishServeH5OnAPI() {
		registerH5MinigameStatic(s)
	} else {
		registerH5ApiRejected(s)
	}
	registerAssetsApiRejected(s)
}

// registerAssetsApiRejected：服务端不再挂载 release/assets；/assets/* 统一 404（H5 走 CDN）。
func registerAssetsApiRejected(s *Server) {
	reject := func(w http.ResponseWriter, r *http.Request) {
		rel := strings.TrimPrefix(r.URL.Path, "/assets/")
		if isH5PublicAssetPath(rel) {
			s.rejectH5OnAPI(w)
			return
		}
		s.writeJSON(w, http.StatusNotFound, Response{
			Code:    404,
			Message: "static assets not served on API port",
		})
	}
	s.mux.HandleFunc("GET /assets/", reject)
	s.mux.HandleFunc("HEAD /assets/", reject)
	logger.Info(logger.TopicAPI, "GET /assets/* rejected (no release/assets on server)")
}

func isH5PublicAssetPath(urlPath string) bool {
	p := strings.Trim(strings.ToLower(urlPath), "/")
	return p == "h5" || strings.HasPrefix(p, "h5/")
}

func (s *Server) rejectH5OnAPI(w http.ResponseWriter) {
	s.writeJSON(w, http.StatusNotFound, Response{
		Code:    404,
		Message: "h5 not served on API port; use CDN (gameBaseCDNUrl)",
	})
}

// registerH5ApiRejected 显式 404，避免未匹配路由落到 GET / 占位 JSON。
func registerH5ApiRejected(s *Server) {
	reject := func(w http.ResponseWriter, _ *http.Request) {
		s.rejectH5OnAPI(w)
	}
	s.mux.HandleFunc("GET /h5/", reject)
	s.mux.HandleFunc("HEAD /h5/", reject)
	logger.Info(logger.TopicAPI, "h5 on API rejected with 404 (CDN only / publish.serveH5OnApi=false)")
}

// GET|HEAD /h5/* → H5 包体（联调 CDN 独立时由 publish.serveH5OnApi=false 关闭）。
func registerH5MinigameStatic(s *Server) {
	h5Root := service.H5AssetsDir()
	if st, err := os.Stat(h5Root); err != nil || !st.IsDir() {
		logger.Warn(logger.TopicAPI, "h5 static skipped — H5AssetsDir not found: %s", h5Root)
		return
	}
	fileServer := http.FileServer(http.Dir(h5Root))
	htmlNoCache := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lp := strings.ToLower(r.URL.Path)
		if strings.HasSuffix(lp, ".html") || strings.HasSuffix(lp, ".htm") {
			w.Header().Set("Cache-Control", "no-store, max-age=0, must-revalidate")
			w.Header().Set("Pragma", "no-cache")
		}
		fileServer.ServeHTTP(w, r)
	})
	s.mux.Handle("GET /h5/", http.StripPrefix("/h5/", htmlNoCache))
	s.mux.Handle("HEAD /h5/", http.StripPrefix("/h5/", htmlNoCache))
	logger.Info(logger.TopicAPI, "h5 minigame static served at /h5/* from %s", h5Root)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	s.writeJSON(w, http.StatusOK, Response{
		Code:    0,
		Message: "ok",
		Data: map[string]string{
			"service": "starcrystal-api",
		},
	})
}

func (s *Server) handleRoot(w http.ResponseWriter, _ *http.Request) {
	s.writeJSON(w, http.StatusOK, Response{
		Code:    0,
		Message: "ok",
		Data: map[string]string{
			"service": "starcrystal-api",
		},
	})
}

func (s *Server) handleFavicon(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListGames(w http.ResponseWriter, r *http.Request) {
	appVersion := strings.TrimSpace(r.URL.Query().Get("appVersion"))
	platform := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("platform")))
	langRaw := strings.TrimSpace(r.URL.Query().Get("lang"))
	lang := NormalizeGameListLang(langRaw)
	channelRaw := strings.TrimSpace(r.URL.Query().Get("channel"))
	if channelRaw == "" {
		logger.Warn(logger.TopicAPI, "[games_list] query parameter \"channel\" is empty — channel-filter behavior follows current server build (restricted games omitted when empty)")
	}

	if appVersion == "" || (platform != "android" && platform != "ios") {
		LogGamesListTraceError(r, "validate_query", "400", 1400, "missing appVersion or invalid platform")
		s.writeJSON(w, http.StatusBadRequest, Response{
			Code:    1400,
			Message: "invalid query: appVersion and platform(android|ios) are required",
		})
		return
	}

	LogGamesListTraceRequest(r, appVersion, platform, langRaw, lang, channelRaw)

	raw, err := s.gameService.ListGamesForClient(appVersion, platform, channelRaw)
	if err != nil {
		LogGamesListTraceError(r, "load_config", "500", 2001, err.Error())
		s.writeJSON(w, http.StatusInternalServerError, Response{
			Code:    2001,
			Message: "load games config failed: " + err.Error(),
		})
		return
	}
	var favSet map[string]struct{}
	if accountID, ok := s.bearerAccountIDOptional(r); ok && s.gameFavoriteService != nil {
		if set, err := s.gameFavoriteService.FavoriteSet(r.Context(), accountID); err == nil {
			favSet = set
		}
	}

	list := make([]GameItem, 0, len(raw))
	for _, g := range raw {
		item := GameItem{
			GameID:         g.GameID,
			Name:           g.Name,
			NameEn:         g.NameEn,
			NameUr:         g.NameUr,
			Note:           g.Note,
			NoteEn:         g.NoteEn,
			NoteUr:         g.NoteUr,
			IconLink:       g.IconLink,
			CoverURL:       g.CoverURL,
			EntryType:      g.EntryType,
			EntryURL:       g.EntryURL,
			DownloadURL:    g.DownloadURL,
			PackageBytes:   g.PackageBytes,
			DownloadSha256: g.DownloadSha256,
			MinAppVersion:  g.MinAppVersion,
			Sort:           g.Sort,
			RewardRuleID:   g.RewardRuleID,
		}
		item = LocalizeGameItem(item, lang)
		if favSet != nil {
			if _, ok := favSet[strings.TrimSpace(g.GameID)]; ok {
				item.Favorited = true
			}
		}
		list = append(list, item)
	}

	configVersion, verErr := service.LoadGamesConfigVersionForAPI()
	if verErr != nil {
		LogGamesListTraceError(r, "load_config_version", "500", 2001, verErr.Error())
		s.writeJSON(w, http.StatusInternalServerError, Response{
			Code:    2001,
			Message: "load games config failed: " + verErr.Error(),
		})
		return
	}

	serverTime := time.Now().UTC().Format(time.RFC3339)
	LogGamesListTraceResponse(r, configVersion, serverTime, channelRaw, list)

	s.writeJSON(w, http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: GameListResponseData{
			ConfigVersion: configVersion,
			ServerTime:    serverTime,
			Games:         list,
		},
	})
}

func (s *Server) handleWalletBalance(w http.ResponseWriter, _ *http.Request) {
	s.writeJSON(w, http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: WalletBalance{
			Balance:         12.36,
			FrozenAmount:    1.00,
			TotalIncome:     50.20,
			TotalGiftRedeem: 36.84,
		},
	})
}

func (s *Server) handleWalletLedger(w http.ResponseWriter, _ *http.Request) {
	s.writeJSON(w, http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"total": 2,
			"list": []WalletLedgerItem{
				{
					LedgerNo:  "L1001",
					BizType:   "ad_reward",
					BizNo:     "E1001",
					Amount:    0.10,
					Direction: "in",
					Status:    "success",
					CreatedAt: "2026-04-25 15:00:00",
				},
				{
					LedgerNo:  "L1002",
					BizType:   "redeem_gift_apply",
					BizNo:     "R1001",
					Amount:    1.00,
					Direction: "freeze",
					Status:    "success",
					CreatedAt: "2026-04-25 15:10:00",
				},
			},
		},
	})
}

// handleWelfareRedeemTokenGift 兑换 curtoken 为礼品（策划 §4）。
func (s *Server) handleWelfareRedeemTokenGift(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeJSON(w, http.StatusMethodNotAllowed, Response{Code: 1405, Message: "method not allowed"})
		return
	}
	accountID, ok := s.requireBearerAccountID(w, r)
	if !ok {
		return
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(r.Body, 1<<20))
	redeemAmount, after, err := s.authService.RedeemTokenForGift(accountID)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "no token") {
			s.writeJSON(w, http.StatusBadRequest, Response{Code: 1400, Message: msg})
			return
		}
		if strings.Contains(msg, "database not configured") {
			s.writeJSON(w, http.StatusServiceUnavailable, Response{Code: 1501, Message: msg})
			return
		}
		logger.Error(logger.TopicAPI, "[welfare/redeem-token-gift] failed account=%s err=%v", accountID, err)
		s.writeJSON(w, http.StatusInternalServerError, Response{Code: 2002, Message: "redeem failed"})
		return
	}
	logger.FatalNotice(logger.TopicAuth, "[redeem_token_gift_ok] account=%s redeemAmount=%.4f", accountID, redeemAmount)
	s.writeJSON(w, http.StatusOK, Response{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"redeemAmount": redeemAmount,
			"curtoken":     after.CurToken,
			"totaltoken":   after.TotalToken,
			"curgold":      after.CurGold,
			"totalgold":    after.TotalGold,
		},
	})
}

// handleWelfareRedeemGiftQuery 查询礼品兑换单状态（占位；路径后缀 redeemId）。
func (s *Server) handleWelfareRedeemGiftQuery(w http.ResponseWriter, r *http.Request) {
	redeemID := strings.TrimPrefix(r.URL.Path, "/api/v1/welfare/redeem-gift/")
	s.writeJSON(w, http.StatusOK, Response{
		Code:    0,
		Message: "success (placeholder)",
		Data: map[string]string{
			"redeemId": redeemID,
			"status":   "pending",
		},
	})
}

func (s *Server) handleAdCallback(w http.ResponseWriter, r *http.Request) {
	network := strings.TrimPrefix(r.URL.Path, "/api/v1/ads/callback/")
	s.writeJSON(w, http.StatusOK, Response{
		Code:    0,
		Message: "success (placeholder)",
		Data: map[string]string{
			"network": network,
			"status":  "received",
		},
	})
}

func (s *Server) writePlaceholder(w http.ResponseWriter, feature string) {
	s.writeJSON(w, http.StatusNotImplemented, Response{
		Code:    1001,
		Message: feature + " not implemented yet",
	})
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, payload Response) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		logger.Error(logger.TopicAPI, "write json failed: %v", err)
		return
	}
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !logger.DebugEnabled() {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		traceID := "req-" + strconv.FormatInt(time.Now().UnixNano(), 10)
		authPresent := "absent"
		if strings.TrimSpace(r.Header.Get("Authorization")) != "" {
			authPresent = "present"
		}

		maxBody := apiDebugHTTPMaxBodyBytes()
		doBodies := apiPathForHTTPLog(r.URL.Path) && apiDebugHTTPBodies()

		var reqBody string
		if doBodies && r.Body != nil {
			reqBody = readRequestBodyForLog(r)
		}

		logger.DebugTrace(traceID, logger.TopicAPI,
			"http request remote=%s method=%s path=%s query=%s proto=%s host=%s ct=%s cl=%d auth=%s ua=%q body=%s",
			r.RemoteAddr, r.Method, r.URL.Path, emptyDash(r.URL.RawQuery), r.Proto, r.Host,
			emptyDash(r.Header.Get("Content-Type")), r.ContentLength, authPresent, r.UserAgent(),
			httpLogBodySnippet(reqBody, maxBody))

		rw := &apiResponseRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
			maxCapture:     maxBody,
			captureBody:    doBodies,
		}
		next.ServeHTTP(rw, r)

		dur := time.Since(start).Milliseconds()
		respSnippet := "-"
		if doBodies {
			respSnippet = httpLogBodySnippet(rw.captureBuf.String(), maxBody)
		}

		logger.DebugTrace(traceID, logger.TopicAPI,
			"http response status=%d bytes=%d duration_ms=%d path=%s query=%s body=%s",
			rw.statusCode, rw.bytesWritten, dur, r.URL.Path, emptyDash(r.URL.RawQuery), respSnippet)
	})
}

// apiResponseRecorder：记录 HTTP 状态、写出字节数，并在 captureBody=true 时装订响应正文片段供 debug。
type apiResponseRecorder struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
	maxCapture   int
	captureBody  bool
	captureBuf   bytes.Buffer
}

func (a *apiResponseRecorder) WriteHeader(code int) {
	a.statusCode = code
	a.ResponseWriter.WriteHeader(code)
}

func (a *apiResponseRecorder) Write(b []byte) (int, error) {
	if a.captureBody && a.maxCapture > 0 && a.captureBuf.Len() < a.maxCapture {
		room := a.maxCapture - a.captureBuf.Len()
		if room > len(b) {
			room = len(b)
		}
		if room > 0 {
			_, _ = a.captureBuf.Write(b[:room])
		}
	}
	n, err := a.ResponseWriter.Write(b)
	a.bytesWritten += n
	return n, err
}

func apiPathForHTTPLog(path string) bool {
	return strings.HasPrefix(path, "/api/")
}

// apiDebugHTTPBodies：默认 true；设 API_DEBUG_HTTP_BODY=0/false/off 可关闭请求/响应体正文日志（仍会打摘要行；仅日志级别为 debug 时进入本中间件）。
func apiDebugHTTPBodies() bool {
	raw := strings.ToLower(strings.TrimSpace(os.Getenv("API_DEBUG_HTTP_BODY")))
	switch raw {
	case "0", "false", "no", "off":
		return false
	default:
		return true
	}
}

var apiDebugMaxBodyMu sync.Mutex
var apiDebugMaxBodyCached int = -2 // -2 unset

func apiDebugHTTPMaxBodyBytes() int {
	apiDebugMaxBodyMu.Lock()
	defer apiDebugMaxBodyMu.Unlock()
	if apiDebugMaxBodyCached != -2 {
		return apiDebugMaxBodyCached
	}
	raw := strings.TrimSpace(os.Getenv("API_DEBUG_HTTP_MAX_BYTES"))
	def := 1 << 20 // 1 MiB
	const hardCap = 10 << 20
	if raw == "" {
		apiDebugMaxBodyCached = def
		return def
	}
	if raw == "-1" {
		apiDebugMaxBodyCached = hardCap
		return hardCap
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		apiDebugMaxBodyCached = def
		return def
	}
	if n == 0 {
		apiDebugMaxBodyCached = 4096
		return 4096
	}
	if n > hardCap {
		apiDebugMaxBodyCached = hardCap
		return hardCap
	}
	apiDebugMaxBodyCached = n
	return n
}

func httpLogBodySnippet(body string, max int) string {
	if max <= 0 {
		max = 1 << 20
	}
	return truncateForLog(body, max)
}

func readRequestBodyForLog(r *http.Request) string {
	if r == nil || r.Body == nil {
		return ""
	}
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		return "<read body failed: " + err.Error() + ">"
	}
	r.Body = io.NopCloser(bytes.NewBuffer(raw))
	return string(raw)
}

func truncateForLog(s string, limit int) string {
	if limit <= 0 {
		limit = 1024
	}
	if len(s) <= limit {
		return s
	}
	return s[:limit] + "...(truncated)"
}

func emptyDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
