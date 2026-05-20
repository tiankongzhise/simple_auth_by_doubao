package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"simple_auth_by_doubao/internal/config"
	"simple_auth_by_doubao/internal/store"
	"simple_auth_by_doubao/internal/token"
)

type Manager struct {
	cfg    *config.Config
	repo   *store.Repository
	tokens *token.Manager
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

type TokenResponse struct {
	AccessToken           string    `json:"accessToken"`
	RefreshToken          string    `json:"refreshToken"`
	AccessTokenExpiresAt  time.Time `json:"accessTokenExpiresAt"`
	RefreshTokenExpiresAt time.Time `json:"refreshTokenExpiresAt"`
}

func NewManager(cfg *config.Config, repo *store.Repository, tokens *token.Manager) *Manager {
	return &Manager{
		cfg:    cfg,
		repo:   repo,
		tokens: tokens,
		now:    time.Now,
	}
}

func (m *Manager) ListServices(ctx context.Context) ([]store.Service, error) {
	return m.repo.ListServices(ctx)
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

func (m *Manager) RefreshTokensForService(ctx context.Context, id int64) (TokenResponse, error) {
	svc, err := m.repo.GetServiceByID(ctx, id)
	if err != nil {
		return TokenResponse{}, mapStoreError(err)
	}
	return m.refreshTokens(ctx, svc)
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
	if svc.AccessTokenExpiresAt == nil || svc.RefreshTokenExpiresAt == nil {
		return TokenResponse{}, fmt.Errorf("token expiration missing after save")
	}
	return TokenResponse{
		AccessToken:           svc.AccessToken,
		RefreshToken:          svc.RefreshToken,
		AccessTokenExpiresAt:  *svc.AccessTokenExpiresAt,
		RefreshTokenExpiresAt: *svc.RefreshTokenExpiresAt,
	}, nil
}

func cleanServiceName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("%w: serviceName is required", ErrBadRequest)
	}
	if len(name) > 120 {
		return "", fmt.Errorf("%w: serviceName is too long", ErrBadRequest)
	}
	return name, nil
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
