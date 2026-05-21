package service

import "testing"

func TestNormalizeOrigin(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "https host lower", raw: "https://Example.COM/path?q=1", want: "https://example.com"},
		{name: "with port", raw: "http://Example.COM:8080/a", want: "http://example.com:8080"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizeOrigin(tt.raw)
			if err != nil {
				t.Fatalf("NormalizeOrigin() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("NormalizeOrigin() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestOriginFromRequestHeaders(t *testing.T) {
	got, err := OriginFromRequestHeaders("", "https://example.com/path")
	if err != nil {
		t.Fatalf("OriginFromRequestHeaders() error = %v", err)
	}
	if got != "https://example.com" {
		t.Fatalf("OriginFromRequestHeaders() = %q", got)
	}
	if _, err := OriginFromRequestHeaders("", ""); err == nil {
		t.Fatalf("OriginFromRequestHeaders(empty) error = nil, want error")
	}
}
