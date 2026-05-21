package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"simple_auth_by_doubao/internal/config"
	"simple_auth_by_doubao/internal/timefmt"
)

const serviceCacheTTL = 5 * time.Minute

type Repository struct {
	db     *pgxpool.Pool
	redis  *redis.Client
	prefix string
}

func NewRepository(ctx context.Context, cfg *config.Config) (*Repository, error) {
	db, err := pgxpool.New(ctx, cfg.DB.DSN())
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	if err := db.Ping(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Username: cfg.Redis.Username,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		db.Close()
		_ = rdb.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	repo := &Repository{
		db:     db,
		redis:  rdb,
		prefix: cfg.Redis.KeyPrefix,
	}
	if err := repo.Migrate(ctx); err != nil {
		repo.Close()
		return nil, err
	}
	return repo, nil
}

func (r *Repository) Close() {
	if r.redis != nil {
		_ = r.redis.Close()
	}
	if r.db != nil {
		r.db.Close()
	}
}

func (r *Repository) Migrate(ctx context.Context) error {
	const ddl = `
CREATE TABLE IF NOT EXISTS services (
  id BIGSERIAL PRIMARY KEY,
  service_name TEXT NOT NULL UNIQUE,
  service_url TEXT NOT NULL UNIQUE,
  authorization_code_hash TEXT NOT NULL,
  authorization_code_masked TEXT NOT NULL,
  qps INTEGER NOT NULL DEFAULT 0,
  qpm INTEGER NOT NULL DEFAULT 0,
  access_token TEXT NOT NULL DEFAULT '',
  refresh_token TEXT NOT NULL DEFAULT '',
  access_token_expires_at BIGINT,
  refresh_token_expires_at BIGINT,
  token_version BIGINT NOT NULL DEFAULT 0,
  created_at BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT,
  updated_at BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT
);

CREATE TABLE IF NOT EXISTS service_groups (
  id BIGSERIAL PRIMARY KEY,
  service_group_name TEXT NOT NULL UNIQUE,
  service_group_url TEXT NOT NULL UNIQUE,
  authorization_code_hash TEXT NOT NULL,
  authorization_code_masked TEXT NOT NULL,
  access_token TEXT NOT NULL DEFAULT '',
  access_token_expires_at BIGINT,
  token_version BIGINT NOT NULL DEFAULT 0,
  created_at BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT,
  updated_at BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT
);

CREATE TABLE IF NOT EXISTS service_group_members (
  service_group_id BIGINT NOT NULL REFERENCES service_groups(id) ON DELETE CASCADE,
  service_id BIGINT NOT NULL REFERENCES services(id) ON DELETE CASCADE,
  created_at BIGINT NOT NULL DEFAULT EXTRACT(EPOCH FROM NOW())::BIGINT,
  PRIMARY KEY (service_group_id, service_id)
);`
	if _, err := r.db.Exec(ctx, ddl); err != nil {
		return fmt.Errorf("migrate postgres: %w", err)
	}
	return nil
}

func (r *Repository) CreateService(ctx context.Context, in CreateServiceInput) (Service, error) {
	const query = `
INSERT INTO services (
  service_name, service_url, authorization_code_hash, authorization_code_masked, qps, qpm
) VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, service_name, service_url, authorization_code_hash, authorization_code_masked,
  qps, qpm, access_token, refresh_token, access_token_expires_at, refresh_token_expires_at,
  token_version, created_at, updated_at`
	row := r.db.QueryRow(ctx, query, in.ServiceName, in.ServiceURL, in.AuthorizationCodeHash, in.AuthorizationCodeMasked, in.QPS, in.QPM)
	svc, err := scanService(row)
	if err != nil {
		if isUniqueViolation(err) {
			return Service{}, ErrConflict
		}
		return Service{}, fmt.Errorf("create service: %w", err)
	}
	r.cacheServiceBestEffort(ctx, svc)
	return svc, nil
}

func (r *Repository) ListServices(ctx context.Context) ([]Service, error) {
	const query = `
SELECT id, service_name, service_url, authorization_code_hash, authorization_code_masked,
  qps, qpm, access_token, refresh_token, access_token_expires_at, refresh_token_expires_at,
  token_version, created_at, updated_at
FROM services
ORDER BY id DESC`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}
	defer rows.Close()

	services := make([]Service, 0)
	for rows.Next() {
		svc, err := scanService(rows)
		if err != nil {
			return nil, fmt.Errorf("scan service: %w", err)
		}
		services = append(services, svc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate services: %w", err)
	}
	return services, nil
}

func (r *Repository) GetServiceByID(ctx context.Context, id int64) (Service, error) {
	if svc, ok := r.getCachedService(ctx, r.keyServiceID(id)); ok {
		return svc, nil
	}
	return r.GetServiceByIDFresh(ctx, id)
}

func (r *Repository) GetServiceByIDFresh(ctx context.Context, id int64) (Service, error) {
	const query = `
SELECT id, service_name, service_url, authorization_code_hash, authorization_code_masked,
  qps, qpm, access_token, refresh_token, access_token_expires_at, refresh_token_expires_at,
  token_version, created_at, updated_at
FROM services
WHERE id = $1`
	svc, err := scanService(r.db.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Service{}, ErrNotFound
		}
		return Service{}, fmt.Errorf("get service by id: %w", err)
	}
	r.cacheServiceBestEffort(ctx, svc)
	return svc, nil
}

func (r *Repository) GetServiceByName(ctx context.Context, name string) (Service, error) {
	if svc, ok := r.getCachedService(ctx, r.keyServiceName(name)); ok {
		return svc, nil
	}
	return r.GetServiceByNameFresh(ctx, name)
}

func (r *Repository) GetServiceByNameFresh(ctx context.Context, name string) (Service, error) {
	const query = `
SELECT id, service_name, service_url, authorization_code_hash, authorization_code_masked,
  qps, qpm, access_token, refresh_token, access_token_expires_at, refresh_token_expires_at,
  token_version, created_at, updated_at
FROM services
WHERE service_name = $1`
	svc, err := scanService(r.db.QueryRow(ctx, query, name))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Service{}, ErrNotFound
		}
		return Service{}, fmt.Errorf("get service by name: %w", err)
	}
	r.cacheServiceBestEffort(ctx, svc)
	return svc, nil
}

func (r *Repository) UpdateService(ctx context.Context, id int64, in UpdateServiceInput) (Service, error) {
	oldSvc, err := r.GetServiceByID(ctx, id)
	if err != nil {
		return Service{}, err
	}
	const query = `
UPDATE services
SET service_name = $2,
  service_url = $3,
  qps = $4,
  qpm = $5,
  updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE id = $1
RETURNING id, service_name, service_url, authorization_code_hash, authorization_code_masked,
  qps, qpm, access_token, refresh_token, access_token_expires_at, refresh_token_expires_at,
  token_version, created_at, updated_at`
	svc, err := scanService(r.db.QueryRow(ctx, query, id, in.ServiceName, in.ServiceURL, in.QPS, in.QPM))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Service{}, ErrNotFound
		}
		if isUniqueViolation(err) {
			return Service{}, ErrConflict
		}
		return Service{}, fmt.Errorf("update service: %w", err)
	}
	r.invalidateServiceBestEffort(ctx, oldSvc)
	r.cacheServiceBestEffort(ctx, svc)
	return svc, nil
}

func (r *Repository) DeleteService(ctx context.Context, id int64) error {
	oldSvc, err := r.GetServiceByIDFresh(ctx, id)
	if err != nil {
		return err
	}
	result, err := r.db.Exec(ctx, `DELETE FROM services WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete service: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	r.invalidateServiceBestEffort(ctx, oldSvc)
	return nil
}

func (r *Repository) SetTokens(ctx context.Context, id int64, accessToken string, refreshToken string, accessExpiresAt time.Time, refreshExpiresAt time.Time) (Service, error) {
	oldSvc, err := r.GetServiceByIDFresh(ctx, id)
	if err != nil {
		return Service{}, err
	}
	const query = `
UPDATE services
SET access_token = $2,
  refresh_token = $3,
  access_token_expires_at = $4,
  refresh_token_expires_at = $5,
  token_version = token_version + 1,
  updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE id = $1
RETURNING id, service_name, service_url, authorization_code_hash, authorization_code_masked,
  qps, qpm, access_token, refresh_token, access_token_expires_at, refresh_token_expires_at,
  token_version, created_at, updated_at`
	svc, err := scanService(r.db.QueryRow(ctx, query, id, accessToken, refreshToken, timefmt.UnixSeconds(accessExpiresAt), timefmt.UnixSeconds(refreshExpiresAt)))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Service{}, ErrNotFound
		}
		return Service{}, fmt.Errorf("set tokens: %w", err)
	}
	r.invalidateServiceBestEffort(ctx, oldSvc)
	r.cacheServiceBestEffort(ctx, svc)
	return svc, nil
}

func (r *Repository) keyServiceID(id int64) string {
	return fmt.Sprintf("%sservice:id:%d", r.prefix, id)
}

func (r *Repository) keyServiceName(name string) string {
	return fmt.Sprintf("%sservice:name:%s", r.prefix, name)
}

func (r *Repository) RateSecondKey(serviceID int64, unixSecond int64) string {
	return fmt.Sprintf("%srate:%d:sec:%d", r.prefix, serviceID, unixSecond)
}

func (r *Repository) RateMinuteKey(serviceID int64, unixMinute int64) string {
	return fmt.Sprintf("%srate:%d:min:%d", r.prefix, serviceID, unixMinute)
}

func (r *Repository) Redis() *redis.Client {
	return r.redis
}

func (r *Repository) getCachedService(ctx context.Context, key string) (Service, bool) {
	raw, err := r.redis.Get(ctx, key).Result()
	if err != nil {
		return Service{}, false
	}
	var svc Service
	if err := json.Unmarshal([]byte(raw), &svc); err != nil {
		return Service{}, false
	}
	return svc, true
}

func (r *Repository) cacheServiceBestEffort(ctx context.Context, svc Service) {
	raw, err := json.Marshal(svc)
	if err != nil {
		log.Printf("marshal service cache failed: %v", err)
		return
	}
	pipe := r.redis.Pipeline()
	pipe.Set(ctx, r.keyServiceID(svc.ID), raw, serviceCacheTTL)
	pipe.Set(ctx, r.keyServiceName(svc.ServiceName), raw, serviceCacheTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		log.Printf("write service cache failed: %v", err)
	}
}

func (r *Repository) invalidateServiceBestEffort(ctx context.Context, svc Service) {
	if err := r.redis.Del(ctx, r.keyServiceID(svc.ID), r.keyServiceName(svc.ServiceName)).Err(); err != nil {
		log.Printf("delete service cache failed: %v", err)
	}
}

type serviceScanner interface {
	Scan(dest ...any) error
}

func scanService(row serviceScanner) (Service, error) {
	var svc Service
	var accessExp sql.NullInt64
	var refreshExp sql.NullInt64
	err := row.Scan(
		&svc.ID,
		&svc.ServiceName,
		&svc.ServiceURL,
		&svc.AuthorizationCodeHash,
		&svc.AuthorizationCodeMasked,
		&svc.QPS,
		&svc.QPM,
		&svc.AccessToken,
		&svc.RefreshToken,
		&accessExp,
		&refreshExp,
		&svc.TokenVersion,
		&svc.CreatedAt,
		&svc.UpdatedAt,
	)
	if err != nil {
		return Service{}, err
	}
	if accessExp.Valid {
		svc.AccessTokenExpiresAt = accessExp.Int64
	}
	if refreshExp.Valid {
		svc.RefreshTokenExpiresAt = refreshExp.Int64
	}
	svc.AccessTokenExpiresAtLocal = timefmt.BeijingLocal(svc.AccessTokenExpiresAt)
	svc.RefreshTokenExpiresAtLocal = timefmt.BeijingLocal(svc.RefreshTokenExpiresAt)
	svc.CreatedAtLocal = timefmt.BeijingLocal(svc.CreatedAt)
	svc.UpdatedAtLocal = timefmt.BeijingLocal(svc.UpdatedAt)
	return svc, nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func isForeignKeyViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23503"
}
