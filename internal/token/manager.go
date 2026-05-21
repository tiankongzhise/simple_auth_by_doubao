package token

import (
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	TypeAdmin          = "admin"
	TypeAccess         = "access"
	TypeRefresh        = "refresh"
	SubjectTypeService = "service"
	SubjectTypeGroup   = "serviceGroup"
)

type Manager struct {
	secret []byte
}

type ServiceClaims struct {
	ServiceName  string `json:"serviceName"`
	TokenType    string `json:"typ"`
	TokenVersion int64  `json:"tokenVersion"`
	SubjectType  string `json:"subjectType,omitempty"`
	jwt.RegisteredClaims
}

type ServiceGroupClaims struct {
	ServiceGroupName string `json:"serviceGroupName"`
	TokenType        string `json:"typ"`
	TokenVersion     int64  `json:"tokenVersion"`
	SubjectType      string `json:"subjectType"`
	jwt.RegisteredClaims
}

type AdminClaims struct {
	TokenType string `json:"typ"`
	jwt.RegisteredClaims
}

type Pair struct {
	AccessToken           string
	RefreshToken          string
	AccessTokenExpiresAt  time.Time
	RefreshTokenExpiresAt time.Time
}

func NewManager(secret string) *Manager {
	return &Manager{secret: []byte(secret)}
}

func (m *Manager) IssueAdmin(username string, ttl time.Duration, now time.Time) (string, time.Time, error) {
	expiresAt := now.Add(ttl)
	claims := AdminClaims{
		TokenType: TypeAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   username,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign admin token: %w", err)
	}
	return signed, expiresAt, nil
}

func (m *Manager) ParseAdmin(raw string) (*AdminClaims, error) {
	claims := &AdminClaims{}
	parsed, err := jwt.ParseWithClaims(raw, claims, m.keyFunc)
	if err != nil {
		return nil, err
	}
	if !parsed.Valid || claims.TokenType != TypeAdmin {
		return nil, fmt.Errorf("invalid admin session")
	}
	return claims, nil
}

func (m *Manager) IssuePair(serviceID int64, serviceName string, tokenVersion int64, accessTTL time.Duration, refreshTTL time.Duration, now time.Time) (Pair, error) {
	accessExpiresAt := now.Add(accessTTL)
	refreshExpiresAt := now.Add(refreshTTL)

	access, err := m.issueServiceToken(serviceID, serviceName, TypeAccess, tokenVersion, accessExpiresAt, now)
	if err != nil {
		return Pair{}, err
	}
	refresh, err := m.issueServiceToken(serviceID, serviceName, TypeRefresh, tokenVersion, refreshExpiresAt, now)
	if err != nil {
		return Pair{}, err
	}
	return Pair{
		AccessToken:           access,
		RefreshToken:          refresh,
		AccessTokenExpiresAt:  accessExpiresAt,
		RefreshTokenExpiresAt: refreshExpiresAt,
	}, nil
}

func (m *Manager) IssueServiceGroupAccess(serviceGroupID int64, serviceGroupName string, tokenVersion int64, accessTTL time.Duration, now time.Time) (string, time.Time, error) {
	expiresAt := now.Add(accessTTL)
	claims := ServiceGroupClaims{
		ServiceGroupName: serviceGroupName,
		TokenType:        TypeAccess,
		TokenVersion:     tokenVersion,
		SubjectType:      SubjectTypeGroup,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(serviceGroupID, 10),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign service group access token: %w", err)
	}
	return signed, expiresAt, nil
}

func (m *Manager) ParseService(raw string, expectedType string) (*ServiceClaims, error) {
	claims := &ServiceClaims{}
	parsed, err := jwt.ParseWithClaims(raw, claims, m.keyFunc)
	if err != nil {
		return nil, err
	}
	if !parsed.Valid || claims.TokenType != expectedType {
		return nil, fmt.Errorf("invalid %s token", expectedType)
	}
	if claims.SubjectType != "" && claims.SubjectType != SubjectTypeService {
		return nil, fmt.Errorf("invalid service token subject")
	}
	if _, err := claims.ServiceID(); err != nil {
		return nil, err
	}
	return claims, nil
}

func (m *Manager) ParseServiceGroup(raw string, expectedType string) (*ServiceGroupClaims, error) {
	claims := &ServiceGroupClaims{}
	parsed, err := jwt.ParseWithClaims(raw, claims, m.keyFunc)
	if err != nil {
		return nil, err
	}
	if !parsed.Valid || claims.TokenType != expectedType || claims.SubjectType != SubjectTypeGroup {
		return nil, fmt.Errorf("invalid service group %s token", expectedType)
	}
	if _, err := claims.ServiceGroupID(); err != nil {
		return nil, err
	}
	return claims, nil
}

func (c *ServiceClaims) ServiceID() (int64, error) {
	id, err := strconv.ParseInt(c.Subject, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid service id claim: %w", err)
	}
	return id, nil
}

func (c *ServiceGroupClaims) ServiceGroupID() (int64, error) {
	id, err := strconv.ParseInt(c.Subject, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid service group id claim: %w", err)
	}
	return id, nil
}

func (m *Manager) issueServiceToken(serviceID int64, serviceName string, tokenType string, tokenVersion int64, expiresAt time.Time, now time.Time) (string, error) {
	claims := ServiceClaims{
		ServiceName:  serviceName,
		TokenType:    tokenType,
		TokenVersion: tokenVersion,
		SubjectType:  SubjectTypeService,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.FormatInt(serviceID, 10),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return "", fmt.Errorf("sign %s token: %w", tokenType, err)
	}
	return signed, nil
}

func (m *Manager) keyFunc(t *jwt.Token) (any, error) {
	if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
		return nil, fmt.Errorf("unexpected signing method %s", t.Method.Alg())
	}
	return m.secret, nil
}
