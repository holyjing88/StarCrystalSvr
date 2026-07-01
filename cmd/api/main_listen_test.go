package main

import "testing"

func TestListenEndpointFromAddr(t *testing.T) {
	tests := []struct {
		addr     string
		wantHost string
		wantPort string
	}{
		{"0.0.0.0:8080", "0.0.0.0", "8080"},
		{":8080", "0.0.0.0", "8080"},
		{"[::]:8080", "0.0.0.0", "8080"},
		{"127.0.0.1:18080", "127.0.0.1", "18080"},
	}
	for _, tc := range tests {
		host, port := listenEndpointFromAddr(tc.addr)
		if host != tc.wantHost || port != tc.wantPort {
			t.Fatalf("listenEndpointFromAddr(%q) = (%q,%q), want (%q,%q)", tc.addr, host, port, tc.wantHost, tc.wantPort)
		}
	}
}

func TestListenAllInterfaces(t *testing.T) {
	for _, host := range []string{"", "0.0.0.0", "::", "[::]"} {
		if !listenAllInterfaces(host) {
			t.Fatalf("expected all interfaces for %q", host)
		}
	}
	if listenAllInterfaces("127.0.0.1") {
		t.Fatal("127.0.0.1 should not be all interfaces")
	}
}
