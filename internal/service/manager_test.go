package service

import "testing"

func TestRemoteIPOnly(t *testing.T) {
	tests := map[string]string{
		"127.0.0.1:1234":  "127.0.0.1",
		"[::1]:1234":      "::1",
		"192.168.1.10":    "192.168.1.10",
		"2001:db8::1":     "2001:db8::1",
		" 10.0.0.1:9000 ": "10.0.0.1",
	}
	for raw, want := range tests {
		if got := remoteIPOnly(raw); got != want {
			t.Fatalf("remoteIPOnly(%q) = %q, want %q", raw, got, want)
		}
	}
}
