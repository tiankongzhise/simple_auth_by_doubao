package service

import "testing"

func TestAuthorizationCodeLifecycle(t *testing.T) {
	code, err := GenerateAuthorizationCode()
	if err != nil {
		t.Fatalf("GenerateAuthorizationCode() error = %v", err)
	}
	if len(code) != 32 {
		t.Fatalf("code length = %d, want 32", len(code))
	}
	if err := ValidateAuthorizationCode(code); err != nil {
		t.Fatalf("ValidateAuthorizationCode() error = %v", err)
	}
	if got := MaskAuthorizationCode(code); got[:4] != code[:4] || got[len(got)-4:] != code[len(code)-4:] {
		t.Fatalf("MaskAuthorizationCode() = %q, want first and last four visible", got)
	}
	hash, err := HashAuthorizationCode(code)
	if err != nil {
		t.Fatalf("HashAuthorizationCode() error = %v", err)
	}
	if !CheckAuthorizationCode(hash, code) {
		t.Fatalf("CheckAuthorizationCode() = false, want true")
	}
	if CheckAuthorizationCode(hash, code[:31]+"x") {
		t.Fatalf("CheckAuthorizationCode() accepted wrong code")
	}
}

func TestValidateAuthorizationCode(t *testing.T) {
	if err := ValidateAuthorizationCode("short"); err == nil {
		t.Fatalf("ValidateAuthorizationCode(short) error = nil, want error")
	}
	if err := ValidateAuthorizationCode("abcd1234abcd1234abcd1234abcd12!!"); err == nil {
		t.Fatalf("ValidateAuthorizationCode(symbols) error = nil, want error")
	}
}
