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

func TestCleanServiceNameURLDecode(t *testing.T) {
	got, err := cleanServiceName("%E8%AE%A2%E5%8D%95%E6%9C%8D%E5%8A%A1")
	if err != nil {
		t.Fatalf("cleanServiceName(encoded Chinese) error = %v", err)
	}
	if got != "订单服务" {
		t.Fatalf("cleanServiceName(encoded Chinese) = %q, want 订单服务", got)
	}

	got, err = cleanServiceName("订单服务")
	if err != nil {
		t.Fatalf("cleanServiceName(raw Chinese) error = %v", err)
	}
	if got != "订单服务" {
		t.Fatalf("cleanServiceName(raw Chinese) = %q, want 订单服务", got)
	}

	if _, err := cleanServiceName("%E8%AE%A2%E5%8D%ZZ"); err == nil {
		t.Fatalf("cleanServiceName(invalid escape) error = nil, want error")
	}
}

func TestCleanServiceGroupNameURLDecode(t *testing.T) {
	got, err := cleanServiceGroupName("core%20group")
	if err != nil {
		t.Fatalf("cleanServiceGroupName(encoded) error = %v", err)
	}
	if got != "core group" {
		t.Fatalf("cleanServiceGroupName(encoded) = %q, want core group", got)
	}
	if _, err := cleanServiceGroupName("   "); err == nil {
		t.Fatalf("cleanServiceGroupName(blank) error = nil, want error")
	}
	if _, err := cleanServiceGroupName("%E8%AE%A2%E5%8D%ZZ"); err == nil {
		t.Fatalf("cleanServiceGroupName(invalid escape) error = nil, want error")
	}
}

func TestValidateServiceIDs(t *testing.T) {
	if err := validateServiceIDs([]int64{1, 2, 2}); err != nil {
		t.Fatalf("validateServiceIDs(valid) error = %v", err)
	}
	if err := validateServiceIDs([]int64{1, 0}); err == nil {
		t.Fatalf("validateServiceIDs(zero) error = nil, want error")
	}
	if err := validateServiceIDs([]int64{-1}); err == nil {
		t.Fatalf("validateServiceIDs(negative) error = nil, want error")
	}
}
