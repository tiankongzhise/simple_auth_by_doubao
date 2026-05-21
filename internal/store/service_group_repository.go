package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"

	"simple_auth_by_doubao/internal/timefmt"
)

func (r *Repository) CreateServiceGroup(ctx context.Context, in CreateServiceGroupInput) (ServiceGroup, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return ServiceGroup{}, fmt.Errorf("begin create service group: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const query = `
WITH next_id AS (
  SELECT nextval(pg_get_serial_sequence('service_groups', 'id')) AS id
)
INSERT INTO service_groups (
  id, service_group_name, service_group_url, authorization_code_hash, authorization_code_masked
)
SELECT id, $1, 'service-group://' || id::TEXT, $2, $3
FROM next_id
RETURNING id, service_group_name, service_group_url, authorization_code_hash,
  authorization_code_masked, access_token, access_token_expires_at, token_version,
  created_at, updated_at`
	group, err := scanServiceGroup(tx.QueryRow(ctx, query, in.ServiceGroupName, in.AuthorizationCodeHash, in.AuthorizationCodeMasked))
	if err != nil {
		if isUniqueViolation(err) {
			return ServiceGroup{}, ErrConflict
		}
		return ServiceGroup{}, fmt.Errorf("create service group: %w", err)
	}
	if err := r.replaceServiceGroupMembersTx(ctx, tx, group.ID, in.ServiceIDs); err != nil {
		return ServiceGroup{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ServiceGroup{}, fmt.Errorf("commit create service group: %w", err)
	}
	return r.GetServiceGroupByIDFresh(ctx, group.ID)
}

func (r *Repository) ListServiceGroups(ctx context.Context) ([]ServiceGroup, error) {
	const query = `
SELECT id, service_group_name, service_group_url, authorization_code_hash,
  authorization_code_masked, access_token, access_token_expires_at, token_version,
  created_at, updated_at
FROM service_groups
ORDER BY id DESC`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list service groups: %w", err)
	}
	defer rows.Close()

	groups := make([]ServiceGroup, 0)
	for rows.Next() {
		group, err := scanServiceGroup(rows)
		if err != nil {
			return nil, fmt.Errorf("scan service group: %w", err)
		}
		group, err = r.withServiceGroupMembers(ctx, group)
		if err != nil {
			return nil, err
		}
		groups = append(groups, group)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate service groups: %w", err)
	}
	return groups, nil
}

func (r *Repository) GetServiceGroupByIDFresh(ctx context.Context, id int64) (ServiceGroup, error) {
	const query = `
SELECT id, service_group_name, service_group_url, authorization_code_hash,
  authorization_code_masked, access_token, access_token_expires_at, token_version,
  created_at, updated_at
FROM service_groups
WHERE id = $1`
	group, err := scanServiceGroup(r.db.QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ServiceGroup{}, ErrNotFound
		}
		return ServiceGroup{}, fmt.Errorf("get service group by id: %w", err)
	}
	return r.withServiceGroupMembers(ctx, group)
}

func (r *Repository) GetServiceGroupByNameFresh(ctx context.Context, name string) (ServiceGroup, error) {
	const query = `
SELECT id, service_group_name, service_group_url, authorization_code_hash,
  authorization_code_masked, access_token, access_token_expires_at, token_version,
  created_at, updated_at
FROM service_groups
WHERE service_group_name = $1`
	group, err := scanServiceGroup(r.db.QueryRow(ctx, query, name))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ServiceGroup{}, ErrNotFound
		}
		return ServiceGroup{}, fmt.Errorf("get service group by name: %w", err)
	}
	return r.withServiceGroupMembers(ctx, group)
}

func (r *Repository) UpdateServiceGroup(ctx context.Context, id int64, in UpdateServiceGroupInput) (ServiceGroup, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return ServiceGroup{}, fmt.Errorf("begin update service group: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const query = `
UPDATE service_groups
SET service_group_name = $2,
  updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE id = $1
RETURNING id, service_group_name, service_group_url, authorization_code_hash,
  authorization_code_masked, access_token, access_token_expires_at, token_version,
  created_at, updated_at`
	group, err := scanServiceGroup(tx.QueryRow(ctx, query, id, in.ServiceGroupName))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ServiceGroup{}, ErrNotFound
		}
		if isUniqueViolation(err) {
			return ServiceGroup{}, ErrConflict
		}
		return ServiceGroup{}, fmt.Errorf("update service group: %w", err)
	}
	if err := r.replaceServiceGroupMembersTx(ctx, tx, group.ID, in.ServiceIDs); err != nil {
		return ServiceGroup{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return ServiceGroup{}, fmt.Errorf("commit update service group: %w", err)
	}
	return r.GetServiceGroupByIDFresh(ctx, group.ID)
}

func (r *Repository) DeleteServiceGroup(ctx context.Context, id int64) error {
	result, err := r.db.Exec(ctx, `DELETE FROM service_groups WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete service group: %w", err)
	}
	if result.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) RefreshServiceGroupAccessToken(ctx context.Context, id int64, issue func(ServiceGroup) (string, time.Time, error)) (ServiceGroup, error) {
	return r.refreshLockedServiceGroupAccessToken(ctx, "id", id, true, time.Time{}, 0, issue)
}

func (r *Repository) EnsureServiceGroupAccessToken(ctx context.Context, name string, now time.Time, minRemaining time.Duration, issue func(ServiceGroup) (string, time.Time, error)) (ServiceGroup, error) {
	return r.refreshLockedServiceGroupAccessToken(ctx, "name", name, false, now, minRemaining, issue)
}

func (r *Repository) ServiceGroupHasService(ctx context.Context, groupID int64, serviceID int64) (bool, error) {
	const query = `
SELECT EXISTS (
  SELECT 1
  FROM service_group_members
  WHERE service_group_id = $1 AND service_id = $2
)`
	var exists bool
	if err := r.db.QueryRow(ctx, query, groupID, serviceID).Scan(&exists); err != nil {
		return false, fmt.Errorf("check service group membership: %w", err)
	}
	return exists, nil
}

func (r *Repository) refreshLockedServiceGroupAccessToken(ctx context.Context, lookup string, value any, force bool, now time.Time, minRemaining time.Duration, issue func(ServiceGroup) (string, time.Time, error)) (ServiceGroup, error) {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return ServiceGroup{}, fmt.Errorf("begin refresh service group token: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	where := "id = $1"
	if lookup == "name" {
		where = "service_group_name = $1"
	}
	query := fmt.Sprintf(`
SELECT id, service_group_name, service_group_url, authorization_code_hash,
  authorization_code_masked, access_token, access_token_expires_at, token_version,
  created_at, updated_at
FROM service_groups
WHERE %s
FOR UPDATE`, where)
	group, err := scanServiceGroup(tx.QueryRow(ctx, query, value))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ServiceGroup{}, ErrNotFound
		}
		return ServiceGroup{}, fmt.Errorf("lock service group: %w", err)
	}

	refreshThreshold := timefmt.UnixSeconds(now.Add(minRemaining))
	if !force && group.AccessToken != "" && group.AccessTokenExpiresAt > refreshThreshold {
		if err := tx.Commit(ctx); err != nil {
			return ServiceGroup{}, fmt.Errorf("commit reuse service group token: %w", err)
		}
		return group, nil
	}

	accessToken, expiresAt, err := issue(group)
	if err != nil {
		return ServiceGroup{}, err
	}
	const update = `
UPDATE service_groups
SET access_token = $2,
  access_token_expires_at = $3,
  token_version = token_version + 1,
  updated_at = EXTRACT(EPOCH FROM NOW())::BIGINT
WHERE id = $1
RETURNING id, service_group_name, service_group_url, authorization_code_hash,
  authorization_code_masked, access_token, access_token_expires_at, token_version,
  created_at, updated_at`
	group, err = scanServiceGroup(tx.QueryRow(ctx, update, group.ID, accessToken, timefmt.UnixSeconds(expiresAt)))
	if err != nil {
		return ServiceGroup{}, fmt.Errorf("set service group access token: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return ServiceGroup{}, fmt.Errorf("commit refresh service group token: %w", err)
	}
	return group, nil
}

func (r *Repository) replaceServiceGroupMembersTx(ctx context.Context, tx pgx.Tx, groupID int64, serviceIDs []int64) error {
	if _, err := tx.Exec(ctx, `DELETE FROM service_group_members WHERE service_group_id = $1`, groupID); err != nil {
		return fmt.Errorf("delete service group members: %w", err)
	}
	for _, serviceID := range uniquePositiveIDs(serviceIDs) {
		_, err := tx.Exec(ctx, `
INSERT INTO service_group_members (service_group_id, service_id)
VALUES ($1, $2)`, groupID, serviceID)
		if err != nil {
			if isForeignKeyViolation(err) {
				return ErrNotFound
			}
			return fmt.Errorf("insert service group member: %w", err)
		}
	}
	return nil
}

func (r *Repository) withServiceGroupMembers(ctx context.Context, group ServiceGroup) (ServiceGroup, error) {
	services, err := r.listServiceGroupMemberServices(ctx, group.ID)
	if err != nil {
		return ServiceGroup{}, err
	}
	group.Services = services
	group.ServiceIDs = make([]int64, 0, len(services))
	for _, svc := range services {
		group.ServiceIDs = append(group.ServiceIDs, svc.ID)
	}
	return group, nil
}

func (r *Repository) listServiceGroupMemberServices(ctx context.Context, groupID int64) ([]Service, error) {
	const query = `
SELECT s.id, s.service_name, s.service_url, s.authorization_code_hash, s.authorization_code_masked,
  s.qps, s.qpm, s.access_token, s.refresh_token, s.access_token_expires_at, s.refresh_token_expires_at,
  s.token_version, s.created_at, s.updated_at
FROM service_group_members m
JOIN services s ON s.id = m.service_id
WHERE m.service_group_id = $1
ORDER BY s.id ASC`
	rows, err := r.db.Query(ctx, query, groupID)
	if err != nil {
		return nil, fmt.Errorf("list service group members: %w", err)
	}
	defer rows.Close()

	services := make([]Service, 0)
	for rows.Next() {
		svc, err := scanService(rows)
		if err != nil {
			return nil, fmt.Errorf("scan service group member: %w", err)
		}
		services = append(services, svc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate service group members: %w", err)
	}
	return services, nil
}

func scanServiceGroup(row serviceScanner) (ServiceGroup, error) {
	var group ServiceGroup
	var accessExp sql.NullInt64
	err := row.Scan(
		&group.ID,
		&group.ServiceGroupName,
		&group.ServiceGroupURL,
		&group.AuthorizationCodeHash,
		&group.AuthorizationCodeMasked,
		&group.AccessToken,
		&accessExp,
		&group.TokenVersion,
		&group.CreatedAt,
		&group.UpdatedAt,
	)
	if err != nil {
		return ServiceGroup{}, err
	}
	if accessExp.Valid {
		group.AccessTokenExpiresAt = accessExp.Int64
	}
	group.AccessTokenExpiresAtLocal = timefmt.BeijingLocal(group.AccessTokenExpiresAt)
	group.CreatedAtLocal = timefmt.BeijingLocal(group.CreatedAt)
	group.UpdatedAtLocal = timefmt.BeijingLocal(group.UpdatedAt)
	return group, nil
}

func uniquePositiveIDs(ids []int64) []int64 {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[int64]struct{}, len(ids))
	values := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		values = append(values, id)
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	return values
}
