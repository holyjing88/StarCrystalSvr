package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"starcrystal/server/internal/logger"
)

// LogGamesListTraceRequest 打印客户端查询参数（便于与 Unity 请求对照）。
func LogGamesListTraceRequest(r *http.Request, appVersion, platform, langRaw, langNorm, channelRaw string) {
	if r == nil {
		return
	}
	logger.Info(logger.TopicAPI, "[games_list][request] remote=%s method=%s path=%s raw_query=%s appVersion=%q platform=%q lang_raw=%q lang_norm=%q channel=%q ua=%q",
		r.RemoteAddr, r.Method, r.URL.Path, r.URL.RawQuery,
		appVersion, platform, langRaw, langNorm, channelRaw, strings.TrimSpace(r.UserAgent()))
}

// LogGamesListTraceResponse 打印本地化后的条目摘要及可选完整 JSON（Debug 级别）。
func LogGamesListTraceResponse(r *http.Request, configVersion, serverTime string, channelRaw string, list []GameItem) {
	logger.Info(logger.TopicAPI, "[games_list][response] channel=%q games_count=%d configVersion=%q serverTime=%q",
		channelRaw, len(list), configVersion, serverTime)
	for i := range list {
		g := list[i]
		logger.Info(logger.TopicAPI, "[games_list][game] idx=%d gameId=%q name=%q note=%q entryType=%q entryUrl=%q minAppVersion=%q sort=%d",
			i, g.GameID, truncateGamesListLog(g.Name, 160), truncateGamesListLog(g.Note, 240),
			g.EntryType, truncateGamesListLog(g.EntryURL, 200), g.MinAppVersion, g.Sort)
	}

	if !logger.DebugEnabled() {
		return
	}
	payload := Response{
		Code:    0,
		Message: "success",
		Data: GameListResponseData{
			ConfigVersion: configVersion,
			ServerTime:    serverTime,
			Games:         list,
		},
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		logger.Debug(logger.TopicAPI, "[games_list][response_json] marshal_error: %v", err)
		return
	}
	logger.Debug(logger.TopicAPI, "[games_list][response_json] bytes=%d body=%s", len(raw), truncateGamesListLog(string(raw), 65536))
}

// LogGamesListTraceError 请求校验失败或服务内部错误时的跟踪行。
func LogGamesListTraceError(r *http.Request, phase string, httpHint string, code int, message string) {
	q := ""
	if r != nil && r.URL != nil {
		q = r.URL.RawQuery
	}
	logger.Warn(logger.TopicAPI, "[games_list][error] phase=%s remote=%s query=%s http=%s code=%d msg=%s",
		phase, remoteAddrGamesList(r), q, httpHint, code, message)
}

func remoteAddrGamesList(r *http.Request) string {
	if r == nil {
		return ""
	}
	return r.RemoteAddr
}

func truncateGamesListLog(s string, maxBytes int) string {
	if maxBytes <= 0 || len(s) <= maxBytes {
		return s
	}
	return s[:maxBytes] + "...(truncated)"
}
