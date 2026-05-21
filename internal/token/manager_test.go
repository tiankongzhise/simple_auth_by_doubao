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
	if claims.SubjectType != SubjectTypeService {
		t.Fatalf("SubjectType = %q, want %q", claims.SubjectType, SubjectTypeService)
	}
	if _, err := manager.ParseService(pair.AccessToken, TypeRefresh); err == nil {
		t.Fatalf("ParseService(access as refresh) error = nil, want error")
	}
}

func TestIssueAndParseServiceGroupAccess(t *testing.T) {
	manager := NewManager("secret")
	now := time.Unix(1779328800, 0)
	raw, expiresAt, err := manager.IssueServiceGroupAccess(7, "core-group", 2, time.Hour, now)
	if err != nil {
		t.Fatalf("IssueServiceGroupAccess() error = %v", err)
	}
	if !expiresAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("expiresAt = %v, want %v", expiresAt, now.Add(time.Hour))
	}
	claims, err := manager.ParseServiceGroup(raw, TypeAccess)
	if err != nil {
		t.Fatalf("ParseServiceGroup(access) error = %v", err)
	}
	serviceGroupID, err := claims.ServiceGroupID()
	if err != nil {
		t.Fatalf("ServiceGroupID() error = %v", err)
	}
	if serviceGroupID != 7 || claims.ServiceGroupName != "core-group" || claims.TokenVersion != 2 || claims.SubjectType != SubjectTypeGroup {
		t.Fatalf("claims = %+v, serviceGroupID = %d", claims, serviceGroupID)
	}
	if _, err := manager.ParseServiceGroup(raw, TypeRefresh); err == nil {
		t.Fatalf("ParseServiceGroup(access as refresh) error = nil, want error")
	}
	if _, err := manager.ParseService(raw, TypeAccess); err == nil {
		t.Fatalf("ParseService(group access as service access) error = nil, want error")
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
