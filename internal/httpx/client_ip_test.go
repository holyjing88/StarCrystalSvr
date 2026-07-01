package httpx

import (
	"net/http"
	"testing"
)

func TestClientIP(t *testing.T) {
	r := http.Request{
		Header: http.Header{"X-Forwarded-For": []string{"203.0.113.10, 10.0.0.1"}},
		RemoteAddr: "127.0.0.1:5566",
	}
	if got := ClientIP(&r); got != "203.0.113.10" {
		t.Fatalf("xff first hop: got %q", got)
	}
	r = http.Request{
		Header:     http.Header{},
		RemoteAddr: "172.31.4.21:8899",
	}
	r.Header.Set("X-Real-Ip", "203.0.113.99")
	if got := ClientIP(&r); got != "203.0.113.99" {
		t.Fatalf("x-real-ip: got %q", got)
	}
}
