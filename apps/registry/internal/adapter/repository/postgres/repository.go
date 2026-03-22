package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	VisibilityPublic = "public"
	VisibilityTenant = "tenant"
)

type Repository struct {
	db *sql.DB
}

type Package struct {
	ID            string
	TenantID      string
	Namespace     string
	Name          string
	LatestVersion string
	Visibility    string
	CreatedBy     string
	CreatedAt     string
	UpdatedAt     string
}

type PackageVersion struct {
	ID           string
	PackageID    string
	TenantID     string
	Version      string
	ManifestJSON map[string]any
	ArtifactKey  string
	Checksum     string
	SizeBytes    int64
	PublishedBy  string
	PublishedAt  string
}

type APIKeyRecord struct {
	ID        string
	KeyHash   string
	UserID    string
	TenantID  string
	Roles     []string
	ExpiresAt string
	RevokedAt string
	CreatedAt string
}

type PackageFilter struct {
	TenantID      string
	IncludePublic bool
	PublicOnly    bool
	Namespace     string
	Query         string
}

type CreatePackageInput struct {
	TenantID   string
	Namespace  string
	Name       string
	Visibility string
	CreatedBy  string
}

type PublishPackageVersionInput struct {
	TenantID    string
	Namespace   string
	Name        string
	Version     string
	Manifest    map[string]any
	ArtifactKey string
	Checksum    string
	SizeBytes   int64
	PublishedBy string
}

type UpdatePackageVisibilityInput struct {
	TenantID   string
	Namespace  string
	Name       string
	Visibility string
}

type DeletePackageInput struct {
	TenantID  string
	Namespace string
	Name      string
}

type AuditEvent struct {
	Time      string
	Action    string
	ActorID   string
	TenantID  string
	Status    string
	RequestID string
	Details   map[string]any
}

func New(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Enabled() bool {
	return r != nil && r.db != nil
}

func (r *Repository) Migrate(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS packages (
			id TEXT PRIMARY KEY,
			tenant_id TEXT NOT NULL,
			namespace TEXT NOT NULL,
			name TEXT NOT NULL,
			visibility TEXT NOT NULL CHECK (visibility IN ('public','tenant')),
			latest_version TEXT,
			created_by TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE (tenant_id, namespace, name)
		)`,
		`CREATE TABLE IF NOT EXISTS package_versions (
			id TEXT PRIMARY KEY,
			package_id TEXT NOT NULL REFERENCES packages(id) ON DELETE CASCADE,
			tenant_id TEXT NOT NULL,
			version TEXT NOT NULL,
			manifest_json JSONB,
			artifact_key TEXT NOT NULL,
			checksum TEXT,
			size_bytes BIGINT,
			published_by TEXT NOT NULL,
			published_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			UNIQUE (package_id, version)
		)`,
		`CREATE TABLE IF NOT EXISTS api_keys (
			id TEXT PRIMARY KEY,
			key_hash TEXT NOT NULL UNIQUE,
			user_id TEXT NOT NULL,
			tenant_id TEXT NOT NULL,
			roles_json JSONB NOT NULL,
			expires_at TIMESTAMPTZ,
			revoked_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS audit_events (
			id BIGSERIAL PRIMARY KEY,
			event_time TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			action TEXT NOT NULL,
			actor TEXT NOT NULL,
			actor_id TEXT NOT NULL DEFAULT 'unknown',
			tenant_id TEXT NOT NULL DEFAULT 'unknown',
			status TEXT NOT NULL,
			request_id TEXT,
			details_json JSONB
		)`,
		`ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS actor_id TEXT`,
		`ALTER TABLE audit_events ADD COLUMN IF NOT EXISTS tenant_id TEXT`,
		`UPDATE audit_events SET actor_id = actor WHERE actor_id IS NULL OR actor_id = ''`,
		`UPDATE audit_events SET tenant_id = 'unknown' WHERE tenant_id IS NULL OR tenant_id = ''`,
		`ALTER TABLE audit_events ALTER COLUMN actor_id SET DEFAULT 'unknown'`,
		`ALTER TABLE audit_events ALTER COLUMN tenant_id SET DEFAULT 'unknown'`,
		`ALTER TABLE audit_events ALTER COLUMN actor_id SET NOT NULL`,
		`ALTER TABLE audit_events ALTER COLUMN tenant_id SET NOT NULL`,
		`CREATE INDEX IF NOT EXISTS idx_packages_tenant ON packages(tenant_id)`,
		`CREATE INDEX IF NOT EXISTS idx_packages_visibility ON packages(visibility)`,
		`CREATE INDEX IF NOT EXISTS idx_versions_package ON package_versions(package_id)`,
		`CREATE INDEX IF NOT EXISTS idx_versions_tenant ON package_versions(tenant_id)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_tenant ON audit_events(tenant_id)`,
	}
	for _, stmt := range stmts {
		if _, err := r.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) SeedAPIKey(ctx context.Context, id, keyHash, userID, tenantID string, roles []string) error {
	rolesJSON, _ := json.Marshal(roles)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO api_keys (id, key_hash, user_id, tenant_id, roles_json, created_at)
		VALUES ($1,$2,$3,$4,$5,NOW())
		ON CONFLICT (key_hash) DO NOTHING
	`, strings.TrimSpace(id), strings.TrimSpace(keyHash), strings.TrimSpace(userID), strings.TrimSpace(tenantID), string(rolesJSON))
	return err
}

func (r *Repository) FindAPIKeyByHash(ctx context.Context, keyHash string) (*APIKeyRecord, error) {
	var rec APIKeyRecord
	var rolesRaw string
	var expires, revoked sql.NullTime
	var created time.Time
	err := r.db.QueryRowContext(ctx, `
		SELECT id, key_hash, user_id, tenant_id, roles_json::text, expires_at, revoked_at, created_at
		FROM api_keys WHERE key_hash = $1
	`, strings.TrimSpace(keyHash)).Scan(&rec.ID, &rec.KeyHash, &rec.UserID, &rec.TenantID, &rolesRaw, &expires, &revoked, &created)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("api key not found")
		}
		return nil, err
	}
	if revoked.Valid {
		return nil, fmt.Errorf("api key revoked")
	}
	if expires.Valid && time.Now().UTC().After(expires.Time.UTC()) {
		return nil, fmt.Errorf("api key expired")
	}
	_ = json.Unmarshal([]byte(rolesRaw), &rec.Roles)
	rec.CreatedAt = created.UTC().Format(time.RFC3339)
	if expires.Valid {
		rec.ExpiresAt = expires.Time.UTC().Format(time.RFC3339)
	}
	if revoked.Valid {
		rec.RevokedAt = revoked.Time.UTC().Format(time.RFC3339)
	}
	return &rec, nil
}

func (r *Repository) ListVisiblePackages(ctx context.Context, in PackageFilter) ([]*Package, error) {
	tenantID := strings.TrimSpace(in.TenantID)
	ns := strings.TrimSpace(in.Namespace)
	q := strings.TrimSpace(in.Query)
	args := []any{}
	where := []string{}
	if in.PublicOnly {
		where = append(where, "visibility = 'public'")
	} else if tenantID != "" && in.IncludePublic {
		args = append(args, tenantID)
		where = append(where, "(visibility = 'public' OR tenant_id = $1)")
	} else if tenantID != "" {
		args = append(args, tenantID)
		where = append(where, "tenant_id = $1")
	}
	if ns != "" {
		args = append(args, strings.ToLower(ns))
		where = append(where, fmt.Sprintf("LOWER(namespace) = $%d", len(args)))
	}
	if q != "" {
		args = append(args, "%"+strings.ToLower(q)+"%")
		where = append(where, fmt.Sprintf("(LOWER(namespace) LIKE $%d OR LOWER(name) LIKE $%d)", len(args), len(args)))
	}
	query := `SELECT id, tenant_id, namespace, name, latest_version, visibility, created_by, created_at, updated_at FROM packages`
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	query += " ORDER BY updated_at DESC, namespace ASC, name ASC"
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Package{}
	for rows.Next() {
		var p Package
		var created, updated time.Time
		if err := rows.Scan(&p.ID, &p.TenantID, &p.Namespace, &p.Name, &p.LatestVersion, &p.Visibility, &p.CreatedBy, &created, &updated); err != nil {
			return nil, err
		}
		p.CreatedAt = created.UTC().Format(time.RFC3339)
		p.UpdatedAt = updated.UTC().Format(time.RFC3339)
		out = append(out, &p)
	}
	return out, nil
}

func (r *Repository) GetVisiblePackage(ctx context.Context, tenantID, namespace, name string, includePublic bool) (*Package, error) {
	tenantID = strings.TrimSpace(tenantID)
	namespace = normalizeSegment(namespace)
	name = normalizeSegment(name)
	var row *sql.Row
	if includePublic {
		row = r.db.QueryRowContext(ctx, `
			SELECT id, tenant_id, namespace, name, latest_version, visibility, created_by, created_at, updated_at
			FROM packages
			WHERE LOWER(namespace) = $1 AND LOWER(name) = $2 AND (tenant_id = $3 OR visibility = 'public')
			ORDER BY updated_at DESC
			LIMIT 1
		`, namespace, name, tenantID)
	} else {
		row = r.db.QueryRowContext(ctx, `
			SELECT id, tenant_id, namespace, name, latest_version, visibility, created_by, created_at, updated_at
			FROM packages
			WHERE LOWER(namespace) = $1 AND LOWER(name) = $2 AND tenant_id = $3
			LIMIT 1
		`, namespace, name, tenantID)
	}
	var p Package
	var created, updated time.Time
	if err := row.Scan(&p.ID, &p.TenantID, &p.Namespace, &p.Name, &p.LatestVersion, &p.Visibility, &p.CreatedBy, &created, &updated); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("package not found")
		}
		return nil, err
	}
	p.CreatedAt = created.UTC().Format(time.RFC3339)
	p.UpdatedAt = updated.UTC().Format(time.RFC3339)
	return &p, nil
}

func (r *Repository) CreatePackage(ctx context.Context, in CreatePackageInput) (*Package, error) {
	tenantID := strings.TrimSpace(in.TenantID)
	namespace := normalizeSegment(in.Namespace)
	name := normalizeSegment(in.Name)
	visibility := normalizeVisibility(in.Visibility)
	createdBy := strings.TrimSpace(in.CreatedBy)
	if tenantID == "" || namespace == "" || name == "" || createdBy == "" {
		return nil, fmt.Errorf("tenant_id, namespace, name, and actor are required")
	}
	if visibility == "" {
		return nil, fmt.Errorf("visibility must be public or tenant")
	}
	id := fmt.Sprintf("pkg_%d", time.Now().UTC().UnixNano())
	var created, updated time.Time
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO packages (id, tenant_id, namespace, name, visibility, created_by, created_at, updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,NOW(),NOW())
		RETURNING created_at, updated_at
	`, id, tenantID, namespace, name, visibility, createdBy).Scan(&created, &updated)
	if err != nil {
		return nil, err
	}
	return &Package{
		ID:         id,
		TenantID:   tenantID,
		Namespace:  namespace,
		Name:       name,
		Visibility: visibility,
		CreatedBy:  createdBy,
		CreatedAt:  created.UTC().Format(time.RFC3339),
		UpdatedAt:  updated.UTC().Format(time.RFC3339),
	}, nil
}

func (r *Repository) PublishPackageVersion(ctx context.Context, in PublishPackageVersionInput) (*PackageVersion, error) {
	tenantID := strings.TrimSpace(in.TenantID)
	namespace := normalizeSegment(in.Namespace)
	name := normalizeSegment(in.Name)
	version := strings.TrimSpace(in.Version)
	publishedBy := strings.TrimSpace(in.PublishedBy)
	if tenantID == "" || namespace == "" || name == "" || version == "" || publishedBy == "" {
		return nil, fmt.Errorf("tenant_id, namespace, name, version, and actor are required")
	}
	pkg, err := r.GetVisiblePackage(ctx, tenantID, namespace, name, false)
	if err != nil {
		return nil, err
	}
	if pkg.TenantID != tenantID {
		return nil, fmt.Errorf("tenant mismatch")
	}
	artifactKey := canonicalArtifactKey(tenantID, namespace, name, version)
	manifestRaw, _ := json.Marshal(cloneMap(in.Manifest))
	id := fmt.Sprintf("pkgv_%d", time.Now().UTC().UnixNano())
	var publishedAt time.Time
	err = r.db.QueryRowContext(ctx, `
		INSERT INTO package_versions (id, package_id, tenant_id, version, manifest_json, artifact_key, checksum, size_bytes, published_by, published_at)
		VALUES ($1,$2,$3,$4,$5::jsonb,$6,$7,$8,$9,NOW())
		RETURNING published_at
	`, id, pkg.ID, tenantID, version, string(manifestRaw), artifactKey, strings.TrimSpace(in.Checksum), in.SizeBytes, publishedBy).Scan(&publishedAt)
	if err != nil {
		return nil, err
	}
	_, _ = r.db.ExecContext(ctx, `UPDATE packages SET latest_version=$1, updated_at=NOW() WHERE id=$2`, pickLatestVersion(pkg.LatestVersion, version), pkg.ID)
	return &PackageVersion{
		ID:           id,
		PackageID:    pkg.ID,
		TenantID:     tenantID,
		Version:      version,
		ManifestJSON: cloneMap(in.Manifest),
		ArtifactKey:  artifactKey,
		Checksum:     strings.TrimSpace(in.Checksum),
		SizeBytes:    in.SizeBytes,
		PublishedBy:  publishedBy,
		PublishedAt:  publishedAt.UTC().Format(time.RFC3339),
	}, nil
}

func (r *Repository) ListPackageVersions(ctx context.Context, packageID string) ([]*PackageVersion, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, package_id, tenant_id, version, manifest_json::text, artifact_key, checksum, size_bytes, published_by, published_at
		FROM package_versions WHERE package_id = $1
	`, strings.TrimSpace(packageID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*PackageVersion{}
	for rows.Next() {
		var rec PackageVersion
		var manifestRaw string
		var publishedAt time.Time
		if err := rows.Scan(&rec.ID, &rec.PackageID, &rec.TenantID, &rec.Version, &manifestRaw, &rec.ArtifactKey, &rec.Checksum, &rec.SizeBytes, &rec.PublishedBy, &publishedAt); err != nil {
			return nil, err
		}
		if strings.TrimSpace(manifestRaw) != "" {
			_ = json.Unmarshal([]byte(manifestRaw), &rec.ManifestJSON)
		}
		rec.PublishedAt = publishedAt.UTC().Format(time.RFC3339)
		out = append(out, &rec)
	}
	sort.Slice(out, func(i, j int) bool {
		cmp := compareVersion(out[i].Version, out[j].Version)
		if cmp != 0 {
			return cmp > 0
		}
		return out[i].PublishedAt > out[j].PublishedAt
	})
	return out, nil
}

func (r *Repository) GetPackageVersion(ctx context.Context, packageID, version string) (*PackageVersion, error) {
	var rec PackageVersion
	var manifestRaw string
	var publishedAt time.Time
	err := r.db.QueryRowContext(ctx, `
		SELECT id, package_id, tenant_id, version, manifest_json::text, artifact_key, checksum, size_bytes, published_by, published_at
		FROM package_versions WHERE package_id=$1 AND version=$2
	`, strings.TrimSpace(packageID), strings.TrimSpace(version)).Scan(&rec.ID, &rec.PackageID, &rec.TenantID, &rec.Version, &manifestRaw, &rec.ArtifactKey, &rec.Checksum, &rec.SizeBytes, &rec.PublishedBy, &publishedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("version not found")
		}
		return nil, err
	}
	if strings.TrimSpace(manifestRaw) != "" {
		_ = json.Unmarshal([]byte(manifestRaw), &rec.ManifestJSON)
	}
	rec.PublishedAt = publishedAt.UTC().Format(time.RFC3339)
	return &rec, nil
}

func (r *Repository) UpdatePackageVisibility(ctx context.Context, in UpdatePackageVisibilityInput) (*Package, error) {
	tenantID := strings.TrimSpace(in.TenantID)
	namespace := normalizeSegment(in.Namespace)
	name := normalizeSegment(in.Name)
	visibility := normalizeVisibility(in.Visibility)
	if tenantID == "" || namespace == "" || name == "" {
		return nil, fmt.Errorf("tenant_id, namespace, and name are required")
	}
	if visibility == "" {
		return nil, fmt.Errorf("visibility must be public or tenant")
	}
	var p Package
	var created, updated time.Time
	err := r.db.QueryRowContext(ctx, `
		UPDATE packages
		SET visibility = $1, updated_at = NOW()
		WHERE tenant_id = $2 AND LOWER(namespace) = $3 AND LOWER(name) = $4
		RETURNING id, tenant_id, namespace, name, latest_version, visibility, created_by, created_at, updated_at
	`, visibility, tenantID, namespace, name).Scan(&p.ID, &p.TenantID, &p.Namespace, &p.Name, &p.LatestVersion, &p.Visibility, &p.CreatedBy, &created, &updated)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("package not found")
		}
		return nil, err
	}
	p.CreatedAt = created.UTC().Format(time.RFC3339)
	p.UpdatedAt = updated.UTC().Format(time.RFC3339)
	return &p, nil
}

func (r *Repository) DeletePackage(ctx context.Context, in DeletePackageInput) error {
	tenantID := strings.TrimSpace(in.TenantID)
	namespace := normalizeSegment(in.Namespace)
	name := normalizeSegment(in.Name)
	if tenantID == "" || namespace == "" || name == "" {
		return fmt.Errorf("tenant_id, namespace, and name are required")
	}
	res, err := r.db.ExecContext(ctx, `DELETE FROM packages WHERE tenant_id = $1 AND LOWER(namespace) = $2 AND LOWER(name) = $3`, tenantID, namespace, name)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("package not found")
	}
	return nil
}

func (r *Repository) RecordAudit(ctx context.Context, ev AuditEvent) error {
	detailsRaw, _ := json.Marshal(ev.Details)
	tm := time.Now().UTC()
	if parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(ev.Time)); err == nil {
		tm = parsed.UTC()
	}
	actorID := strings.TrimSpace(ev.ActorID)
	if actorID == "" {
		actorID = "unknown"
	}
	tenantID := strings.TrimSpace(ev.TenantID)
	if tenantID == "" {
		tenantID = "unknown"
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO audit_events (event_time, action, actor, actor_id, tenant_id, status, request_id, details_json)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8::jsonb)
	`, tm, strings.TrimSpace(ev.Action), actorID, actorID, tenantID, strings.TrimSpace(ev.Status), strings.TrimSpace(ev.RequestID), string(detailsRaw))
	return err
}

func (r *Repository) ListAudit(ctx context.Context, limit int) ([]AuditEvent, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT event_time, action, actor_id, tenant_id, status, request_id, COALESCE(details_json::text, '{}')
		FROM audit_events
		ORDER BY event_time DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []AuditEvent{}
	for rows.Next() {
		var tm time.Time
		var ev AuditEvent
		var detailsRaw string
		if err := rows.Scan(&tm, &ev.Action, &ev.ActorID, &ev.TenantID, &ev.Status, &ev.RequestID, &detailsRaw); err != nil {
			return nil, err
		}
		ev.Time = tm.UTC().Format(time.RFC3339)
		_ = json.Unmarshal([]byte(detailsRaw), &ev.Details)
		out = append(out, ev)
	}
	return out, nil
}

func normalizeVisibility(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case VisibilityPublic:
		return VisibilityPublic
	case VisibilityTenant, "":
		if strings.TrimSpace(v) == "" {
			return VisibilityTenant
		}
		return VisibilityTenant
	default:
		return ""
	}
}

func normalizeSegment(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	return strings.ToLower(v)
}

func pickLatestVersion(current, incoming string) string {
	if strings.TrimSpace(current) == "" {
		return strings.TrimSpace(incoming)
	}
	if compareVersion(strings.TrimSpace(incoming), strings.TrimSpace(current)) > 0 {
		return strings.TrimSpace(incoming)
	}
	return strings.TrimSpace(current)
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func canonicalArtifactKey(tenantID, namespace, name, version string) string {
	return fmt.Sprintf(
		"tenants/%s/packages/%s/%s/%s/artifact.tar.gz",
		strings.TrimSpace(tenantID),
		normalizeSegment(namespace),
		normalizeSegment(name),
		strings.TrimSpace(version),
	)
}

func compareVersion(a, b string) int {
	am, an, ap, aok := parseSemver(a)
	bm, bn, bp, bok := parseSemver(b)
	if aok && bok {
		if am != bm {
			return sign(am - bm)
		}
		if an != bn {
			return sign(an - bn)
		}
		if ap != bp {
			return sign(ap - bp)
		}
		return 0
	}
	if aok && !bok {
		return 1
	}
	if !aok && bok {
		return -1
	}
	if a == b {
		return 0
	}
	if a > b {
		return 1
	}
	return -1
}

func parseSemver(v string) (int, int, int, bool) {
	v = strings.TrimSpace(strings.TrimPrefix(v, "v"))
	if v == "" {
		return 0, 0, 0, false
	}
	if idx := strings.Index(v, "+"); idx >= 0 {
		v = v[:idx]
	}
	if idx := strings.Index(v, "-"); idx >= 0 {
		v = v[:idx]
	}
	parts := strings.Split(v, ".")
	if len(parts) < 1 {
		return 0, 0, 0, false
	}
	m, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, false
	}
	n := 0
	p := 0
	if len(parts) > 1 {
		n, err = strconv.Atoi(parts[1])
		if err != nil {
			return 0, 0, 0, false
		}
	}
	if len(parts) > 2 {
		p, err = strconv.Atoi(parts[2])
		if err != nil {
			return 0, 0, 0, false
		}
	}
	return m, n, p, true
}

func sign(v int) int {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	default:
		return 0
	}
}
