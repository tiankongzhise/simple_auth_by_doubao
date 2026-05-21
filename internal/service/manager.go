package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"simple_auth_by_doubao/internal/config"
	"simple_auth_by_doubao/internal/limiter"
	"simple_auth_by_doubao/internal/store"
	"simple_auth_by_doubao/internal/timefmt"
	"simple_auth_by_doubao/internal/token"
)

type Manager struct {
	cfg    *config.Config
	repo   *store.Repository
	tokens *token.Manager
	limit  *limiter.Limiter
	now    func() time.Time
}

type CreateServiceInput struct {
	ServiceName       string `json:"serviceName"`
	ServiceURL        string `json:"serviceUrl"`
	AuthorizationCode string `json:"authorizationCode"`
	QPS               int    `json:"qps"`
	QPM               int    `json:"qpm"`
}

type UpdateServiceInput struct {
	ServiceName string `json:"serviceName"`
	ServiceURL  string `json:"serviceUrl"`
	QPS         int    `json:"qps"`
	QPM         int    `json:"qpm"`
}

type CreateServiceGroupInput struct {
	ServiceGroupName  string  `json:"serviceGroupName"`
	AuthorizationCode string  `json:"authorizationCode"`
	ServiceIDs        []int64 `json:"serviceIds"`
}

type UpdateServiceGroupInput struct {
	ServiceGroupName string  `json:"serviceGroupName"`
	ServiceIDs       []int64 `json:"serviceIds"`
}

type TokenResponse struct {
	AccessToken                string `json:"accessToken"`
	RefreshToken               string `json:"refreshToken"`
	AccessTokenExpiresAt       int64  `json:"accessTokenExpiresAt"`
	AccessTokenExpiresAtLocal  string `json:"accessTokenExpiresAtLocal"`
	RefreshTokenExpiresAt      int64  `json:"refreshTokenExpiresAt"`
	RefreshTokenExpiresAtLocal string `json:"refreshTokenExpiresAtLocal"`
}

type ServiceGroupTokenResponse struct {
	AccessToken               string `json:"accessToken"`
	AccessTokenExpiresAt      int64  `json:"accessTokenExpiresAt"`
	AccessTokenExpiresAtLocal string `json:"accessTokenExpiresAtLocal"`
}

type ExchangeTokenInput struct {
	ServiceName       string `json:"serviceName"`
	AuthorizationCode string `json:"authorizationCode"`
	Origin            string
	Referer           string
	RemoteAddr        string
	Model             string
}

type RefreshTokenInput struct {
	ServiceName  string `json:"serviceName"`
	RefreshToken string `json:"refreshToken"`
	Origin       string
	Referer      string
	RemoteAddr   string
	Model        string
}

type VerifyInput struct {
	ServiceName       string
	TargetServiceName string
	AccessToken       string
	Origin            string
	Referer           string
	RemoteAddr        string
	Model             string
}

type VerifyResponse struct {
	OK               bool   `json:"ok"`
	ServiceName      string `json:"serviceName"`
	ServiceGroupName string `json:"serviceGroupName,omitempty"`
}

type LatestServiceGroupTokenInput struct {
	ServiceGroupName  string `json:"serviceGroupName"`
	AuthorizationCode string `json:"authorizationCode"`
}

func NewManager(cfg *config.Config, repo *store.Repository, tokens *token.Manager) *Manager {
	return &Manager{
		cfg:    cfg,
		repo:   repo,
		tokens: tokens,
		limit:  limiter.New(repo),
		now:    time.Now,
	}
}

func (m *Manager) ListServices(ctx context.Context) ([]store.Service, error) {
	return m.repo.ListServices(ctx)
}

func (m *Manager) ListServiceGroups(ctx context.Context) ([]store.ServiceGroup, error) {
	return m.repo.ListServiceGroups(ctx)
}

func (m *Manager) CreateService(ctx context.Context, in CreateServiceInput) (store.Service, string, error) {
	name, err := cleanServiceName(in.ServiceName)
	if err != nil {
		return store.Service{}, "", err
	}
	origin, err := NormalizeOrigin(in.ServiceURL)
	if err != nil {
		return store.Service{}, "", err
	}
	if in.QPS < 0 || in.QPM < 0 {
		return store.Service{}, "", fmt.Errorf("%w: qps and qpm must be greater than or equal to 0", ErrBadRequest)
	}

	code := strings.TrimSpace(in.AuthorizationCode)
	if code == "" {
		code, err = GenerateAuthorizationCode()
		if err != nil {
			return store.Service{}, "", err
		}
	}
	if err := ValidateAuthorizationCode(code); err != nil {
		return store.Service{}, "", err
	}
	codeHash, err := HashAuthorizationCode(code)
	if err != nil {
		return store.Service{}, "", err
	}

	svc, err := m.repo.CreateService(ctx, store.CreateServiceInput{
		ServiceName:             name,
		ServiceURL:              origin,
		AuthorizationCodeHash:   codeHash,
		AuthorizationCodeMasked: MaskAuthorizationCode(code),
		QPS:                     in.QPS,
		QPM:                     in.QPM,
	})
	if err != nil {
		return store.Service{}, "", mapStoreError(err)
	}
	return svc, code, nil
}

func (m *Manager) UpdateService(ctx context.Context, id int64, in UpdateServiceInput) (store.Service, error) {
	name, err := cleanServiceName(in.ServiceName)
	if err != nil {
		return store.Service{}, err
	}
	origin, err := NormalizeOrigin(in.ServiceURL)
	if err != nil {
		return store.Service{}, err
	}
	if in.QPS < 0 || in.QPM < 0 {
		return store.Service{}, fmt.Errorf("%w: qps and qpm must be greater than or equal to 0", ErrBadRequest)
	}
	svc, err := m.repo.UpdateService(ctx, id, store.UpdateServiceInput{
		ServiceName: name,
		ServiceURL:  origin,
		QPS:         in.QPS,
		QPM:         in.QPM,
	})
	if err != nil {
		return store.Service{}, mapStoreError(err)
	}
	return svc, nil
}

func (m *Manager) DeleteService(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("%w: service id is invalid", ErrBadRequest)
	}
	if err := m.repo.DeleteService(ctx, id); err != nil {
		return mapStoreError(err)
	}
	return nil
}

func (m *Manager) CreateServiceGroup(ctx context.Context, in CreateServiceGroupInput) (store.ServiceGroup, string, error) {
	name, err := cleanServiceGroupName(in.ServiceGroupName)
	if err != nil {
		return store.ServiceGroup{}, "", err
	}
	if err := validateServiceIDs(in.ServiceIDs); err != nil {
		return store.ServiceGroup{}, "", err
	}
	code := strings.TrimSpace(in.AuthorizationCode)
	if code == "" {
		code, err = GenerateAuthorizationCode()
		if err != nil {
			return store.ServiceGroup{}, "", err
		}
	}
	if err := ValidateAuthorizationCode(code); err != nil {
		return store.ServiceGroup{}, "", err
	}
	codeHash, err := HashAuthorizationCode(code)
	if err != nil {
		return store.ServiceGroup{}, "", err
	}
	group, err := m.repo.CreateServiceGroup(ctx, store.CreateServiceGroupInput{
		ServiceGroupName:        name,
		AuthorizationCodeHash:   codeHash,
		AuthorizationCodeMasked: MaskAuthorizationCode(code),
		ServiceIDs:              in.ServiceIDs,
	})
	if err != nil {
		return store.ServiceGroup{}, "", mapStoreError(err)
	}
	return group, code, nil
}

func (m *Manager) UpdateServiceGroup(ctx context.Context, id int64, in UpdateServiceGroupInput) (store.ServiceGroup, error) {
	name, err := cleanServiceGroupName(in.ServiceGroupName)
	if err != nil {
		return store.ServiceGroup{}, err
	}
	if err := validateServiceIDs(in.ServiceIDs); err != nil {
		return store.ServiceGroup{}, err
	}
	group, err := m.repo.UpdateServiceGroup(ctx, id, store.UpdateServiceGroupInput{
		ServiceGroupName: name,
		ServiceIDs:       in.ServiceIDs,
	})
	if err != nil {
		return store.ServiceGroup{}, mapStoreError(err)
	}
	return group, nil
}

func (m *Manager) DeleteServiceGroup(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("%w: service group id is invalid", ErrBadRequest)
	}
	if err := m.repo.DeleteServiceGroup(ctx, id); err != nil {
		return mapStoreError(err)
	}
	return nil
}

func (m *Manager) RefreshTokensForService(ctx context.Context, id int64) (TokenResponse, error) {
	svc, err := m.repo.GetServiceByIDFresh(ctx, id)
	if err != nil {
		return TokenResponse{}, mapStoreError(err)
	}
	return m.refreshTokens(ctx, svc)
}

func (m *Manager) RefreshTokenForServiceGroup(ctx context.Context, id int64) (ServiceGroupTokenResponse, error) {
	group, err := m.repo.RefreshServiceGroupAccessToken(ctx, id, func(group store.ServiceGroup) (string, time.Time, error) {
		return m.issueServiceGroupAccess(group, group.TokenVersion+1)
	})
	if err != nil {
		return ServiceGroupTokenResponse{}, mapStoreError(err)
	}
	return serviceGroupTokenResponse(group)
}

func (m *Manager) LatestServiceGroupToken(ctx context.Context, in LatestServiceGroupTokenInput) (ServiceGroupTokenResponse, error) {
	name, err := cleanServiceGroupName(in.ServiceGroupName)
	if err != nil {
		return ServiceGroupTokenResponse{}, err
	}
	if err := ValidateAuthorizationCode(strings.TrimSpace(in.AuthorizationCode)); err != nil {
		return ServiceGroupTokenResponse{}, err
	}
	current, err := m.repo.GetServiceGroupByNameFresh(ctx, name)
	if err != nil {
		return ServiceGroupTokenResponse{}, mapStoreError(err)
	}
	if !CheckAuthorizationCode(current.AuthorizationCodeHash, strings.TrimSpace(in.AuthorizationCode)) {
		return ServiceGroupTokenResponse{}, fmt.Errorf("%w: authorizationCode is invalid", ErrUnauthorized)
	}
	group, err := m.repo.EnsureServiceGroupAccessToken(ctx, name, m.now(), time.Hour, func(group store.ServiceGroup) (string, time.Time, error) {
		return m.issueServiceGroupAccess(group, group.TokenVersion+1)
	})
	if err != nil {
		return ServiceGroupTokenResponse{}, mapStoreError(err)
	}
	return serviceGroupTokenResponse(group)
}

func (m *Manager) ExchangeToken(ctx context.Context, in ExchangeTokenInput) (TokenResponse, error) {
	name, err := cleanServiceName(in.ServiceName)
	if err != nil {
		return TokenResponse{}, err
	}
	if err := ValidateAuthorizationCode(strings.TrimSpace(in.AuthorizationCode)); err != nil {
		return TokenResponse{}, err
	}
	svc, err := m.repo.GetServiceByNameFresh(ctx, name)
	if err != nil {
		return TokenResponse{}, mapStoreError(err)
	}
	if err := m.ensureOriginAllowed(svc, in.Origin, in.Referer, in.RemoteAddr, in.Model); err != nil {
		return TokenResponse{}, err
	}
	if !CheckAuthorizationCode(svc.AuthorizationCodeHash, strings.TrimSpace(in.AuthorizationCode)) {
		return TokenResponse{}, fmt.Errorf("%w: authorizationCode is invalid", ErrUnauthorized)
	}
	if err := m.applyLimit(ctx, svc); err != nil {
		return TokenResponse{}, err
	}
	return m.refreshTokens(ctx, svc)
}

func (m *Manager) RefreshToken(ctx context.Context, in RefreshTokenInput) (TokenResponse, error) {
	name, err := cleanServiceName(in.ServiceName)
	if err != nil {
		return TokenResponse{}, err
	}
	svc, err := m.repo.GetServiceByNameFresh(ctx, name)
	if err != nil {
		return TokenResponse{}, mapStoreError(err)
	}
	if err := m.ensureOriginAllowed(svc, in.Origin, in.Referer, in.RemoteAddr, in.Model); err != nil {
		return TokenResponse{}, err
	}
	claims, err := m.tokens.ParseService(strings.TrimSpace(in.RefreshToken), token.TypeRefresh)
	if err != nil {
		return TokenResponse{}, fmt.Errorf("%w: refreshToken is invalid", ErrUnauthorized)
	}
	if err := m.ensureCurrentToken(svc, claims, in.RefreshToken, token.TypeRefresh); err != nil {
		return TokenResponse{}, err
	}
	if err := m.applyLimit(ctx, svc); err != nil {
		return TokenResponse{}, err
	}
	return m.refreshTokens(ctx, svc)
}

func (m *Manager) Verify(ctx context.Context, in VerifyInput) (VerifyResponse, error) {
	name, err := cleanServiceName(in.ServiceName)
	if err != nil {
		return VerifyResponse{}, err
	}
	if strings.TrimSpace(in.TargetServiceName) != "" {
		return m.verifyServiceGroup(ctx, name, in)
	}
	svc, err := m.repo.GetServiceByNameFresh(ctx, name)
	if err != nil {
		return VerifyResponse{}, mapStoreError(err)
	}
	if err := m.ensureOriginAllowed(svc, in.Origin, in.Referer, in.RemoteAddr, in.Model); err != nil {
		return VerifyResponse{}, err
	}
	claims, err := m.tokens.ParseService(strings.TrimSpace(in.AccessToken), token.TypeAccess)
	if err != nil {
		return VerifyResponse{}, fmt.Errorf("%w: access token is invalid", ErrUnauthorized)
	}
	if err := m.ensureCurrentToken(svc, claims, in.AccessToken, token.TypeAccess); err != nil {
		return VerifyResponse{}, err
	}
	if err := m.applyLimit(ctx, svc); err != nil {
		return VerifyResponse{}, err
	}
	return VerifyResponse{OK: true, ServiceName: svc.ServiceName}, nil
}

func (m *Manager) verifyServiceGroup(ctx context.Context, groupName string, in VerifyInput) (VerifyResponse, error) {
	targetName, err := cleanServiceName(in.TargetServiceName)
	if err != nil {
		return VerifyResponse{}, err
	}
	group, err := m.repo.GetServiceGroupByNameFresh(ctx, groupName)
	if err != nil {
		return VerifyResponse{}, mapStoreError(err)
	}
	target, err := m.repo.GetServiceByNameFresh(ctx, targetName)
	if err != nil {
		return VerifyResponse{}, mapStoreError(err)
	}
	claims, err := m.tokens.ParseServiceGroup(strings.TrimSpace(in.AccessToken), token.TypeAccess)
	if err != nil {
		return VerifyResponse{}, fmt.Errorf("%w: service group access token is invalid", ErrUnauthorized)
	}
	if err := m.ensureCurrentServiceGroupToken(group, claims, in.AccessToken); err != nil {
		return VerifyResponse{}, err
	}
	hasService, err := m.repo.ServiceGroupHasService(ctx, group.ID, target.ID)
	if err != nil {
		return VerifyResponse{}, err
	}
	if !hasService {
		return VerifyResponse{}, fmt.Errorf("%w: target service is not managed by service group", ErrForbidden)
	}
	if err := m.applyLimit(ctx, target); err != nil {
		return VerifyResponse{}, err
	}
	return VerifyResponse{OK: true, ServiceName: target.ServiceName, ServiceGroupName: group.ServiceGroupName}, nil
}

func (m *Manager) refreshTokens(ctx context.Context, svc store.Service) (TokenResponse, error) {
	nextVersion := svc.TokenVersion + 1
	pair, err := m.tokens.IssuePair(svc.ID, svc.ServiceName, nextVersion, m.cfg.AccessTTL, m.cfg.RefreshTTL, m.now())
	if err != nil {
		return TokenResponse{}, err
	}
	svc, err = m.repo.SetTokens(ctx, svc.ID, pair.AccessToken, pair.RefreshToken, pair.AccessTokenExpiresAt, pair.RefreshTokenExpiresAt)
	if err != nil {
		return TokenResponse{}, mapStoreError(err)
	}
	if svc.AccessTokenExpiresAt <= 0 || svc.RefreshTokenExpiresAt <= 0 {
		return TokenResponse{}, fmt.Errorf("token expiration missing after save")
	}
	return TokenResponse{
		AccessToken:                svc.AccessToken,
		RefreshToken:               svc.RefreshToken,
		AccessTokenExpiresAt:       svc.AccessTokenExpiresAt,
		AccessTokenExpiresAtLocal:  timefmt.BeijingLocal(svc.AccessTokenExpiresAt),
		RefreshTokenExpiresAt:      svc.RefreshTokenExpiresAt,
		RefreshTokenExpiresAtLocal: timefmt.BeijingLocal(svc.RefreshTokenExpiresAt),
	}, nil
}

func (m *Manager) issueServiceGroupAccess(group store.ServiceGroup, tokenVersion int64) (string, time.Time, error) {
	return m.tokens.IssueServiceGroupAccess(group.ID, group.ServiceGroupName, tokenVersion, m.cfg.AccessTTL, m.now())
}

func serviceGroupTokenResponse(group store.ServiceGroup) (ServiceGroupTokenResponse, error) {
	if group.AccessTokenExpiresAt <= 0 {
		return ServiceGroupTokenResponse{}, fmt.Errorf("service group token expiration missing after save")
	}
	return ServiceGroupTokenResponse{
		AccessToken:               group.AccessToken,
		AccessTokenExpiresAt:      group.AccessTokenExpiresAt,
		AccessTokenExpiresAtLocal: timefmt.BeijingLocal(group.AccessTokenExpiresAt),
	}, nil
}

func (m *Manager) ensureCurrentToken(svc store.Service, claims *token.ServiceClaims, raw string, tokenType string) error {
	serviceID, err := claims.ServiceID()
	if err != nil {
		return fmt.Errorf("%w: token service id is invalid", ErrUnauthorized)
	}
	if serviceID != svc.ID || claims.ServiceName != svc.ServiceName || claims.TokenVersion != svc.TokenVersion {
		return fmt.Errorf("%w: token does not match current service", ErrUnauthorized)
	}
	switch tokenType {
	case token.TypeAccess:
		if raw == "" || raw != svc.AccessToken {
			return fmt.Errorf("%w: access token is not current", ErrUnauthorized)
		}
	case token.TypeRefresh:
		if raw == "" || raw != svc.RefreshToken {
			return fmt.Errorf("%w: refreshToken is not current", ErrUnauthorized)
		}
	default:
		return fmt.Errorf("%w: token type is invalid", ErrUnauthorized)
	}
	return nil
}

func (m *Manager) ensureCurrentServiceGroupToken(group store.ServiceGroup, claims *token.ServiceGroupClaims, raw string) error {
	serviceGroupID, err := claims.ServiceGroupID()
	if err != nil {
		return fmt.Errorf("%w: token service group id is invalid", ErrUnauthorized)
	}
	if serviceGroupID != group.ID || claims.ServiceGroupName != group.ServiceGroupName || claims.TokenVersion != group.TokenVersion {
		return fmt.Errorf("%w: token does not match current service group", ErrUnauthorized)
	}
	if raw == "" || raw != group.AccessToken {
		return fmt.Errorf("%w: service group access token is not current", ErrUnauthorized)
	}
	return nil
}

func (m *Manager) ensureOriginAllowed(svc store.Service, origin string, referer string, remoteAddr string, model string) error {
	if m.isDevBypassAllowed(remoteAddr, model) {
		return nil
	}
	requestOrigin, err := OriginFromRequestHeaders(origin, referer)
	if err != nil {
		return err
	}
	if requestOrigin != svc.ServiceURL {
		return fmt.Errorf("%w: request origin is not registered for service", ErrForbidden)
	}
	return nil
}

func (m *Manager) isDevBypassAllowed(remoteAddr string, model string) bool {
	if !m.cfg.DevMode || strings.TrimSpace(strings.ToLower(model)) != "dev" {
		return false
	}
	remoteIP := remoteIPOnly(remoteAddr)
	for _, ip := range m.cfg.DevIPs {
		if remoteIP == strings.TrimSpace(ip) {
			return true
		}
	}
	return false
}

func (m *Manager) applyLimit(ctx context.Context, svc store.Service) error {
	if err := m.limit.Allow(ctx, svc); err != nil {
		return fmt.Errorf("%w: %v", ErrTooManyRequests, err)
	}
	return nil
}

func remoteIPOnly(remoteAddr string) string {
	remoteAddr = strings.TrimSpace(remoteAddr)
	host, _, ok := strings.Cut(remoteAddr, ":")
	if ok && strings.Count(remoteAddr, ":") == 1 {
		return host
	}
	if strings.HasPrefix(remoteAddr, "[") {
		end := strings.Index(remoteAddr, "]")
		if end > 0 {
			return remoteAddr[1:end]
		}
	}
	return remoteAddr
}

func cleanServiceName(name string) (string, error) {
	name = strings.TrimSpace(name)
	decoded, err := url.PathUnescape(name)
	if err != nil {
		return "", fmt.Errorf("%w: serviceName url decode failed", ErrBadRequest)
	}
	name = strings.TrimSpace(decoded)
	if name == "" {
		return "", fmt.Errorf("%w: serviceName is required", ErrBadRequest)
	}
	if len(name) > 120 {
		return "", fmt.Errorf("%w: serviceName is too long", ErrBadRequest)
	}
	return name, nil
}

func cleanServiceGroupName(name string) (string, error) {
	name = strings.TrimSpace(name)
	decoded, err := url.PathUnescape(name)
	if err != nil {
		return "", fmt.Errorf("%w: serviceGroupName url decode failed", ErrBadRequest)
	}
	name = strings.TrimSpace(decoded)
	if name == "" {
		return "", fmt.Errorf("%w: serviceGroupName is required", ErrBadRequest)
	}
	if len(name) > 120 {
		return "", fmt.Errorf("%w: serviceGroupName is too long", ErrBadRequest)
	}
	return name, nil
}

func validateServiceIDs(serviceIDs []int64) error {
	for _, id := range serviceIDs {
		if id <= 0 {
			return fmt.Errorf("%w: serviceIds must contain positive integers", ErrBadRequest)
		}
	}
	return nil
}

func mapStoreError(err error) error {
	switch {
	case errors.Is(err, store.ErrNotFound):
		return ErrNotFound
	case errors.Is(err, store.ErrConflict):
		return ErrConflict
	default:
		return err
	}
}
