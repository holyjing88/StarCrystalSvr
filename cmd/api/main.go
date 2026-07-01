package main

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"starcrystal/server/internal/api"
	"starcrystal/server/internal/logger"
	"starcrystal/server/internal/service"
	"starcrystal/server/internal/starcrystaljson"
)

type appConfig struct {
	Log struct {
		Level string `json:"level"`
	} `json:"log"`
	// ApiListenHost HTTP 监听 IP；空或未配置时默认 0.0.0.0（所有网卡）。
	ApiListenHost string `json:"apiListenHost"`
	// ApiListenPort HTTP 监听端口；与 ApiListenHost 一并写入 release/configs/starcrystal.json。
	// 仅当至少配置了 host（非空）或 port（>0）之一时，才使用 JSON 中的监听地址；否则回退到默认 0.0.0.0:8080。
	ApiListenPort int `json:"apiListenPort"`
	// RedisAddr 人气榜 Redis 地址，如 "127.0.0.1:6379"；可由环境变量 REDIS_ADDR 覆盖。
	RedisAddr string `json:"redisAddr"`
	// RedisPassword 可选；可由 REDIS_PASSWORD 覆盖（若进程环境中已设置该变量键）。
	RedisPassword string `json:"redisPassword"`
	// RedisDB 逻辑库 0–15，默认 0；可由 REDIS_DB 覆盖。
	RedisDB int `json:"redisDb"`
	// UseHTTPS 为 true 时仅接受 HTTPS 请求（直连 TLS 或反向代理 X-Forwarded-Proto: https）。
	UseHTTPS bool `json:"useHttps"`
	// TlsCertFile / TlsKeyFile 可选；与 useHttps 同用时由进程直接 TLS 终止（相对 configs/ 目录或绝对路径）。
	TlsCertFile string `json:"tlsCertFile"`
	TlsKeyFile  string `json:"tlsKeyFile"`
}

type loadedAppSettings struct {
	LogLevel    string
	ListenAddr  string
	ListenOK    bool
	ConfigPath  string
	RankRedis   service.RankRedisConfig
	UseHTTPS    bool
	TlsCertFile string
	TlsKeyFile  string
}

func listenAddrFromAppConfig(cfg appConfig) (addr string, ok bool) {
	host := strings.TrimSpace(cfg.ApiListenHost)
	port := cfg.ApiListenPort
	if host == "" && port <= 0 {
		return "", false
	}
	if host == "" {
		host = "0.0.0.0"
	}
	if port <= 0 {
		port = 8080
	}
	return net.JoinHostPort(host, strconv.Itoa(port)), true
}

// listenEndpointFromAddr 从 net.Addr / "host:port" 解析 IP 与端口；未指定 IP 时 host 为 "0.0.0.0"。
func listenEndpointFromAddr(addr string) (host, port string) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr, ""
	}
	host = strings.TrimSpace(host)
	if host == "" || host == "0.0.0.0" || host == "::" || host == "[::]" {
		host = "0.0.0.0"
	}
	return host, port
}

func listenAllInterfaces(host string) bool {
	switch strings.TrimSpace(host) {
	case "", "0.0.0.0", "::", "[::]":
		return true
	default:
		return false
	}
}

func listenBindHostInvalid(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "not valid in its context") || strings.Contains(msg, "cannot assign requested address")
}

// loadStarcrystalAppSettings 读取首个可用的 starcrystal.json：日志、HTTP 监听、Redis（人气榜）、HTTPS。
func loadStarcrystalAppSettings() loadedAppSettings {
	for _, p := range starcrystaljson.ConfigCandidates() {
		raw, err := starcrystaljson.ReadFileUTF8(p)
		if err != nil {
			continue
		}
		var cfg appConfig
		if err := json.Unmarshal(raw, &cfg); err != nil {
			continue
		}
		lv := strings.TrimSpace(cfg.Log.Level)
		la, ok := listenAddrFromAppConfig(cfg)
		db := cfg.RedisDB
		if db < 0 || db > 15 {
			db = 0
		}
		return loadedAppSettings{
			LogLevel:    lv,
			ListenAddr:  la,
			ListenOK:    ok,
			ConfigPath:  p,
			UseHTTPS:    cfg.UseHTTPS,
			TlsCertFile: strings.TrimSpace(cfg.TlsCertFile),
			TlsKeyFile:  strings.TrimSpace(cfg.TlsKeyFile),
			RankRedis: service.RankRedisConfig{
				Addr:     strings.TrimSpace(cfg.RedisAddr),
				Password: cfg.RedisPassword,
				DB:       db,
			},
		}
	}
	return loadedAppSettings{}
}

func resolveConfigRelativePath(configPath, relPath string) string {
	relPath = strings.TrimSpace(relPath)
	if relPath == "" {
		return ""
	}
	if filepath.IsAbs(relPath) {
		return relPath
	}
	if configPath != "" {
		return filepath.Join(filepath.Dir(configPath), relPath)
	}
	return relPath
}

// logDirBesideExecutable returns <directory of starcrystalsvr.exe>/log (resolved symlinks when possible).
func logDirBesideExecutable() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return filepath.Join(filepath.Dir(exe), "log"), nil
}

func initLogger(cfgLevelFromFile string, cfgPath string) {
	logDir, exeErr := logDirBesideExecutable()
	logDirFallback := false
	if exeErr != nil {
		logger.Error(logger.TopicMain, "os.Executable for log dir failed (%v); falling back to cwd/log", exeErr)
		cwd, e2 := os.Getwd()
		if e2 != nil {
			logger.Error(logger.TopicMain, "getwd failed: %v; keep stdout logger only", e2)
			return
		}
		logDir = filepath.Join(cwd, "log")
		logDirFallback = true
	}
	if err := os.MkdirAll(logDir, 0755); err != nil {
		logger.Error(logger.TopicMain, "create log dir failed (%s), keep stdout logger only: %v", logDir, err)
		return
	}
	sink, err := logger.NewRotatingDualFileWriter(logDir)
	if err != nil {
		logger.Error(logger.TopicMain, "open rotating log files failed (%s), keep stdout logger only: %v", logDir, err)
		sink = nil
	}
	envLevel := strings.TrimSpace(os.Getenv("LOG_LEVEL"))
	logLevel := strings.TrimSpace(cfgLevelFromFile)
	logLevelSource := "config"
	if logLevel == "" {
		logLevel = envLevel
		logLevelSource = "env"
	}
	if logLevel == "" {
		// 无配置/env 时默认 debug；打包 Android Release 时由 Unity 将 release/configs/starcrystal.json 写为 error
		logLevel = "debug"
		logLevelSource = "default"
	}
	if sink != nil {
		logger.Init(sink, logLevel)
	} else {
		logger.Init(os.Stdout, logLevel)
	}
	logger.SvrStartBanner()
	mainLog := filepath.Join(logDir, "starcrystalsvr.log")
	errLog := filepath.Join(logDir, "starcrystalsvr_error.log")
	if logLevelSource == "config" && cfgPath != "" {
		logger.Info(logger.TopicMain, "logger initialized, main=%s error=%s level=%s source=config(%s)", mainLog, errLog, logLevel, cfgPath)
	} else {
		logger.Info(logger.TopicMain, "logger initialized, main=%s error=%s level=%s source=%s", mainLog, errLog, logLevel, logLevelSource)
	}
	if logDirFallback {
		logger.Warn(logger.TopicMain, "log directory is cwd/log because os.Executable failed; normal layout is <starcrystalsvr.exe dir>/log")
	}
}

func main() {
	cfg := loadStarcrystalAppSettings()
	initLogger(cfg.LogLevel, cfg.ConfigPath)

	if s := strings.TrimSpace(os.Getenv("API_ADDR")); s != "" {
		logger.Warn(logger.TopicMain, "API_ADDR is set but ignored; use apiListenHost/apiListenPort in release/configs/starcrystal.json (was: %q)", s)
	}

	var addr, addrSource string
	switch {
	case cfg.ListenOK:
		addr = cfg.ListenAddr
		addrSource = "config(" + cfg.ConfigPath + ")"
	default:
		addr = "0.0.0.0:8080"
		if cfg.ConfigPath != "" {
			addrSource = "default (add apiListenHost/apiListenPort to " + cfg.ConfigPath + " to customize)"
		} else {
			addrSource = "default (starcrystal.json not found; 0.0.0.0:8080)"
		}
	}

	s := api.NewServer(cfg.RankRedis)
	if cfg.UseHTTPS {
		s.SetRequireHTTPS(true)
	}
	bgCtx, bgCancel := context.WithCancel(context.Background())
	defer bgCancel()
	s.StartEconomyBackground(bgCtx)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	s.ProbeRankBackend(ctx)
	cancel()

	requestedAddr := addr
	requestedSource := addrSource
	ln, bindErr := net.Listen("tcp", addr)
	if bindErr != nil && cfg.ListenOK && listenBindHostInvalid(bindErr) {
		hostPart, portStr, splitErr := net.SplitHostPort(addr)
		if splitErr == nil && strings.TrimSpace(hostPart) != "" {
			fallback := net.JoinHostPort("", portStr)
			cfgRef := cfg.ConfigPath
			if cfgRef == "" {
				cfgRef = "release/configs/starcrystal.json"
			}
			logger.Warn(logger.TopicMain, "listen on %s failed (%v); retrying %s (all interfaces). Set apiListenHost to \"0.0.0.0\" in %s to skip this.", addr, bindErr, fallback, cfgRef)
			if ln2, e2 := net.Listen("tcp", fallback); e2 == nil {
				ln = ln2
				bindErr = nil
				addr = fallback
				addrSource = "fallback after unsuccessful bind (see " + cfgRef + ")"
			} else {
				bindErr = e2
			}
		}
	}
	if bindErr != nil {
		if listenBindHostInvalid(bindErr) {
			host := requestedAddr
			if h, _, splitErr := net.SplitHostPort(requestedAddr); splitErr == nil {
				host = h
			}
			logger.FatalNotice(logger.TopicMain, "HTTP listen bind failed: apiListenHost %q is not assigned on this machine (use 0.0.0.0, 127.0.0.1, or an IP from ipconfig); err=%v", host, bindErr)
		}
		logger.Fatal(logger.TopicMain, "server stopped: %v", bindErr)
	}
	extra := ""
	if addr != requestedAddr {
		extra = "; retried bind address " + addr + " [" + addrSource + "]"
	}
	socketAddr := ln.Addr().String()
	boundHost, boundPort := listenEndpointFromAddr(socketAddr)
	reqHost, reqPort := listenEndpointFromAddr(requestedAddr)
	logger.FatalNotice(logger.TopicMain,
		"HTTP API listen bound: ip=%s port=%s allInterfaces=%v (kernel socket=%s; config ip=%s port=%s allInterfaces=%v [%s])%s",
		boundHost, boundPort, listenAllInterfaces(boundHost),
		socketAddr, reqHost, reqPort, listenAllInterfaces(reqHost), requestedSource, extra)

	tlsCert := resolveConfigRelativePath(cfg.ConfigPath, cfg.TlsCertFile)
	tlsKey := resolveConfigRelativePath(cfg.ConfigPath, cfg.TlsKeyFile)
	httpSrv := &http.Server{Handler: s.Handler()}
	if cfg.UseHTTPS {
		switch {
		case tlsCert != "" && tlsKey != "":
			if _, err := os.Stat(tlsCert); err != nil {
				logger.Fatal(logger.TopicMain, "useHttps=true but tlsCertFile not found: %s (%v)", tlsCert, err)
			}
			if _, err := os.Stat(tlsKey); err != nil {
				logger.Fatal(logger.TopicMain, "useHttps=true but tlsKeyFile not found: %s (%v)", tlsKey, err)
			}
			logger.FatalNotice(logger.TopicMain, "HTTPS enabled: direct TLS (cert=%s key=%s); plain HTTP rejected", tlsCert, tlsKey)
			if err := httpSrv.ServeTLS(ln, tlsCert, tlsKey); err != nil {
				logger.Fatal(logger.TopicMain, "server stopped: %v", err)
			}
		default:
			logger.FatalNotice(logger.TopicMain, "HTTPS enabled: proxy mode (require X-Forwarded-Proto: https or direct TLS); tlsCertFile/tlsKeyFile unset")
			if err := httpSrv.Serve(ln); err != nil {
				logger.Fatal(logger.TopicMain, "server stopped: %v", err)
			}
		}
		return
	}
	if err := httpSrv.Serve(ln); err != nil {
		logger.Fatal(logger.TopicMain, "server stopped: %v", err)
	}
}
