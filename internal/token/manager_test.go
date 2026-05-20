package token

import (
	"testing"
	"time"
)

func TestIssueAndParseServicePair(t *testing.T) {
	manager := NewManager("secret")
	now := time.Unix(1779328800, 0)
	pair, err := manager.IssuePair(12, "billing", 3, time.Hour, 24*time.Hour, now)
	if err != nil {
		t.Fatalf("IssuePair() error = %v", err)
	}
	claims, err := manager.ParseService(pair.AccessToken, TypeAccess)
	if err != nil {
		t.Fatalf("ParseService(access) error = %v", err)
	}
	serviceID, err := claims.ServiceID()
	if err != nil {
		t.Fatalf("ServiceID() error = %v", err)
	}
	if serviceID != 12 || claims.ServiceName != "billing" || claims.TokenVersion != 3 {
		t.Fatalf("claims = %+v, serviceID = %d", claims, serviceID)
	}
	if _, err := manager.ParseService(pair.AccessToken, TypeRefresh); err == nil {
		t.Fatalf("ParseService(access as refresh) error = nil, want error")
	}
}

func TestIssueAndParseAdmin(t *testing.T) {
	manager := NewManager("secret")
	session, _, err := manager.IssueAdmin("admin", time.Hour, time.Unix(1779328800, 0))
	if err != nil {
		t.Fatalf("IssueAdmin() error = %v", err)
	}
	claims, err := manager.ParseAdmin(session)
	if err != nil {
		t.Fatalf("ParseAdmin() error = %v", err)
	}
	if claims.Subject != "admin" || claims.TokenType != TypeAdmin {
		t.Fatalf("claims = %+v", claims)
	}
}
