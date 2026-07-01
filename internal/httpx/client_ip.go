package httpx

import (
	"net"
	"net/http"
	"strings"
)

// ClientIP 优先信任 X-Forwarded-For 首段（部署在反向代理后时需代理清洗），其次 X-Real-IP，最后 RemoteAddr。
func ClientIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
		for _, seg := range strings.Split(xff, ",") {
			ip := strings.TrimSpace(seg)
			if ip != "" {
				return ip
			}
		}
	}
	if xri := strings.TrimSpace(r.Header.Get("X-Real-IP")); xri != "" {
		return xri
	}
	addr := strings.TrimSpace(r.RemoteAddr)
	if addr == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	return host
}

func IsLoopback(ip string) bool {
	s := strings.TrimSpace(ip)
	return s == "127.0.0.1" || s == "::1"
}
