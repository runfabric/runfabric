package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	mongorepo "github.com/runfabric/runfabric/registry/internal/adapter/repository/mongodb"
	pgrepo "github.com/runfabric/runfabric/registry/internal/adapter/repository/postgres"
)

const metadataOpTimeout = 8 * time.Second

type MetadataRepository interface {
	FindAPIKey(raw string) (*APIKeyRecord, error)
	ListVisiblePackages(in PackageFilter) ([]*RegistryPackage, error)
	GetVisiblePackage(tenantID, namespace, name string, includePublic bool) (*RegistryPackage, error)
	CreatePackage(in CreatePackageInput) (*RegistryPackage, error)
	PublishPackageVersion(in PublishPackageVersionInput) (*RegistryPackageVersion, error)
	ListPackageVersions(packageID string) ([]*RegistryPackageVersion, error)
	GetPackageVersion(packageID, version string) (*RegistryPackageVersion, error)
	UpdatePackageVisibility(in UpdatePackageVisibilityInput) (*RegistryPackage, error)
	DeletePackage(in DeletePackageInput) error
	RecordAudit(ev AuditEvent) error
	ListAudit(limit int) ([]AuditEvent, error)
	CreateOrganization(in CreateOrganizationInput) (*Organization, error)
	ListOrganizations(in ListOrganizationsInput) ([]*Organization, error)
	GetOrganization(slug string) (*Organization, error)
	AddOrganizationMember(in OrgMemberInput) (*Organization, error)
	RemoveOrganizationMember(slug, userID string) (*Organization, error)
}

type metadataBackend struct {
	repo    MetadataRepository
	closeFn func() error
}

type jsonMetadataRepository struct {
	store *Store
}

type postgresMetadataRepository struct {
	repo *pgrepo.Repository
}

type mongodbMetadataRepository struct {
	repo *mongorepo.Repository
}

func normalizeMetadataProvider(opts OpenOptions) string {
	provider := strings.ToLower(strings.TrimSpace(opts.MetadataProvider))
	if provider == "" || provider == "auto" {
		if strings.TrimSpace(opts.PostgresDSN) != "" {
			return "postgres"
		}
		if strings.TrimSpace(opts.MongoDBURI) != "" {
			return "mongodb"
		}
		return "json"
	}
	if provider == "mongo" {
		return "mongodb"
	}
	return provider
}

func openMetadataBackend(s *Store, provider string, opts OpenOptions) (*metadataBackend, error) {
	switch provider {
	case "json", "":
		return &metadataBackend{repo: &jsonMetadataRepository{store: s}}, nil
	case "postgres":
		if strings.TrimSpace(opts.PostgresDSN) == "" {
			return nil, fmt.Errorf("postgres metadata provider requires --postgres-dsn")
		}
		driver := strings.TrimSpace(opts.PostgresDriver)
		if driver == "" {
			driver = "pgx"
		}
		db, err := sql.Open(driver, strings.TrimSpace(opts.PostgresDSN))
		if err != nil {
			return nil, fmt.Errorf("open postgres: %w", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), metadataOpTimeout)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("ping postgres: %w", err)
		}
		repo := pgrepo.New(db)
		if err := repo.Migrate(ctx); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("migrate postgres: %w", err)
		}
		if opts.SeedLocalDevData {
			if err := repo.SeedAPIKey(ctx, "key_local_dev", HashAPIKey("rk_local_dev"), "ci-bot", "tenant_runfabric", []string{"admin", "publisher", "reader"}); err != nil {
				_ = db.Close()
				return nil, fmt.Errorf("seed api key: %w", err)
			}
		}
		return &metadataBackend{
			repo: &postgresMetadataRepository{repo: repo},
			closeFn: func() error {
				return db.Close()
			},
		}, nil
	case "mongodb":
		uri := strings.TrimSpace(opts.MongoDBURI)
		if uri == "" {
			return nil, fmt.Errorf("mongodb metadata provider requires --mongodb-uri")
		}
		dbName := strings.TrimSpace(opts.MongoDBDatabase)
		if dbName == "" {
			dbName = "runfabric_registry"
		}
		timeout := opts.MongoDBConnectTimeout
		if timeout <= 0 {
			timeout = metadataOpTimeout
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		repo, err := mongorepo.Connect(ctx, uri, dbName)
		if err != nil {
			return nil, fmt.Errorf("connect mongodb: %w", err)
		}
		if err := repo.Migrate(ctx); err != nil {
			_ = repo.Close(context.Background())
			return nil, fmt.Errorf("migrate mongodb: %w", err)
		}
		if opts.SeedLocalDevData {
			if err := repo.SeedAPIKey(ctx, "key_local_dev", HashAPIKey("rk_local_dev"), "ci-bot", "tenant_runfabric", []string{"admin", "publisher", "reader"}); err != nil {
				_ = repo.Close(context.Background())
				return nil, fmt.Errorf("seed api key: %w", err)
			}
		}
		return &metadataBackend{
			repo: &mongodbMetadataRepository{repo: repo},
			closeFn: func() error {
				ctx, cancel := context.WithTimeout(context.Background(), timeout)
				defer cancel()
				return repo.Close(ctx)
			},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported metadata provider: %s", provider)
	}
}

func (r *jsonMetadataRepository) FindAPIKey(raw string) (*APIKeyRecord, error) {
	return r.store.findAPIKeyJSON(raw)
}

func (r *jsonMetadataRepository) ListVisiblePackages(in PackageFilter) ([]*RegistryPackage, error) {
	return r.store.listVisiblePackagesJSON(in)
}

func (r *jsonMetadataRepository) GetVisiblePackage(tenantID, namespace, name string, includePublic bool) (*RegistryPackage, error) {
	return r.store.getVisiblePackageJSON(tenantID, namespace, name, includePublic)
}

func (r *jsonMetadataRepository) CreatePackage(in CreatePackageInput) (*RegistryPackage, error) {
	return r.store.createPackageJSON(in)
}

func (r *jsonMetadataRepository) PublishPackageVersion(in PublishPackageVersionInput) (*RegistryPackageVersion, error) {
	return r.store.publishPackageVersionJSON(in)
}

func (r *jsonMetadataRepository) ListPackageVersions(packageID string) ([]*RegistryPackageVersion, error) {
	return r.store.listPackageVersionsJSON(packageID)
}

func (r *jsonMetadataRepository) GetPackageVersion(packageID, version string) (*RegistryPackageVersion, error) {
	return r.store.getPackageVersionJSON(packageID, version)
}

func (r *jsonMetadataRepository) UpdatePackageVisibility(in UpdatePackageVisibilityInput) (*RegistryPackage, error) {
	return r.store.updatePackageVisibilityJSON(in)
}

func (r *jsonMetadataRepository) DeletePackage(in DeletePackageInput) error {
	return r.store.deletePackageJSON(in)
}

func (r *jsonMetadataRepository) RecordAudit(ev AuditEvent) error {
	return r.store.recordAuditJSON(ev)
}

func (r *jsonMetadataRepository) ListAudit(limit int) ([]AuditEvent, error) {
	return r.store.listAuditJSON(limit)
}

func (r *jsonMetadataRepository) CreateOrganization(in CreateOrganizationInput) (*Organization, error) {
	return r.store.createOrganizationJSON(in)
}

func (r *jsonMetadataRepository) ListOrganizations(in ListOrganizationsInput) ([]*Organization, error) {
	return r.store.listOrganizationsJSON(in)
}

func (r *jsonMetadataRepository) GetOrganization(slug string) (*Organization, error) {
	return r.store.getOrganizationJSON(slug)
}

func (r *jsonMetadataRepository) AddOrganizationMember(in OrgMemberInput) (*Organization, error) {
	return r.store.addOrganizationMemberJSON(in)
}

func (r *jsonMetadataRepository) RemoveOrganizationMember(slug, userID string) (*Organization, error) {
	return r.store.removeOrganizationMemberJSON(slug, userID)
}

func (r *postgresMetadataRepository) FindAPIKey(raw string) (*APIKeyRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), metadataOpTimeout)
	defer cancel()
	rec, err := r.repo.FindAPIKeyByHash(ctx, HashAPIKey(raw))
	if err != nil {
		return nil, err
	}
	return fromPGAPIKey(rec), nil
}

func (r *postgresMetadataRepository) ListVisiblePackages(in PackageFilter) ([]*RegistryPackage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), metadataOpTimeout)
	defer cancel()
	items, err := r.repo.ListVisiblePackages(ctx, toPGPackageFilter(in))
	if err != nil {
		return nil, err
	}
	out := make([]*RegistryPackage, 0, len(items))
	for _, item := range items {
		out = append(out, fromPGPackage(item))
	}
	return out, nil
}

func (r *postgresMetadataRepository) GetVisiblePackage(tenantID, namespace, name string, includePublic bool) (*RegistryPackage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), metadataOpTimeout)
	defer cancel()
	pkg, err := r.repo.GetVisiblePackage(ctx, tenantID, namespace, name, includePublic)
	if err != nil {
		return nil, err
	}
	return fromPGPackage(pkg), nil
}

func (r *postgresMetadataRepository) CreatePackage(in CreatePackageInput) (*RegistryPackage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), metadataOpTimeout)
	defer cancel()
	pkg, err := r.repo.CreatePackage(ctx, toPGCreatePackageInput(in))
	if err != nil {
		return nil, err
	}
	return fromPGPackage(pkg), nil
}

func (r *postgresMetadataRepository) PublishPackageVersion(in PublishPackageVersionInput) (*RegistryPackageVersion, error) {
	ctx, cancel := context.WithTimeout(context.Background(), metadataOpTimeout)
	defer cancel()
	rec, err := r.repo.PublishPackageVersion(ctx, toPGPublishInput(in))
	if err != nil {
		return nil, err
	}
	return fromPGPackageVersion(rec), nil
}

func (r *postgresMetadataRepository) ListPackageVersions(packageID string) ([]*RegistryPackageVersion, error) {
	ctx, cancel := context.WithTimeout(context.Background(), metadataOpTimeout)
	defer cancel()
	items, err := r.repo.ListPackageVersions(ctx, packageID)
	if err != nil {
		return nil, err
	}
	out := make([]*RegistryPackageVersion, 0, len(items))
	for _, item := range items {
		out = append(out, fromPGPackageVersion(item))
	}
	return out, nil
}

func (r *postgresMetadataRepository) GetPackageVersion(packageID, version string) (*RegistryPackageVersion, error) {
	ctx, cancel := context.WithTimeout(context.Background(), metadataOpTimeout)
	defer cancel()
	rec, err := r.repo.GetPackageVersion(ctx, packageID, version)
	if err != nil {
		return nil, err
	}
	return fromPGPackageVersion(rec), nil
}

func (r *postgresMetadataRepository) UpdatePackageVisibility(in UpdatePackageVisibilityInput) (*RegistryPackage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), metadataOpTimeout)
	defer cancel()
	pkg, err := r.repo.UpdatePackageVisibility(ctx, toPGUpdateVisibilityInput(in))
	if err != nil {
		return nil, err
	}
	return fromPGPackage(pkg), nil
}

func (r *postgresMetadataRepository) DeletePackage(in DeletePackageInput) error {
	ctx, cancel := context.WithTimeout(context.Background(), metadataOpTimeout)
	defer cancel()
	return r.repo.DeletePackage(ctx, toPGDeletePackageInput(in))
}

func (r *postgresMetadataRepository) RecordAudit(ev AuditEvent) error {
	ctx, cancel := context.WithTimeout(context.Background(), metadataOpTimeout)
	defer cancel()
	return r.repo.RecordAudit(ctx, pgrepo.AuditEvent{
		Time:      ev.Time,
		Action:    ev.Action,
		ActorID:   ev.ActorID,
		TenantID:  ev.TenantID,
		Status:    ev.Status,
		RequestID: ev.RequestID,
		Details:   cloneMap(ev.Details),
	})
}

func (r *postgresMetadataRepository) ListAudit(limit int) ([]AuditEvent, error) {
	ctx, cancel := context.WithTimeout(context.Background(), metadataOpTimeout)
	defer cancel()
	items, err := r.repo.ListAudit(ctx, limit)
	if err != nil {
		return nil, err
	}
	out := make([]AuditEvent, 0, len(items))
	for _, item := range items {
		out = append(out, AuditEvent{
			Time:      item.Time,
			Action:    item.Action,
			ActorID:   item.ActorID,
			TenantID:  item.TenantID,
			Status:    item.Status,
			RequestID: item.RequestID,
			Details:   cloneMap(item.Details),
		})
	}
	return out, nil
}

// Organizations are not yet persisted on the postgres backend; the json and
// mongodb backends implement them fully.
func (r *postgresMetadataRepository) CreateOrganization(CreateOrganizationInput) (*Organization, error) {
	return nil, errOrganizationsUnsupported
}

func (r *postgresMetadataRepository) ListOrganizations(ListOrganizationsInput) ([]*Organization, error) {
	return nil, errOrganizationsUnsupported
}

func (r *postgresMetadataRepository) GetOrganization(string) (*Organization, error) {
	return nil, errOrganizationsUnsupported
}

func (r *postgresMetadataRepository) AddOrganizationMember(OrgMemberInput) (*Organization, error) {
	return nil, errOrganizationsUnsupported
}

func (r *postgresMetadataRepository) RemoveOrganizationMember(string, string) (*Organization, error) {
	return nil, errOrganizationsUnsupported
}

func (r *mongodbMetadataRepository) FindAPIKey(raw string) (*APIKeyRecord, error) {
	ctx, cancel := context.WithTimeout(context.Background(), metadataOpTimeout)
	defer cancel()
	rec, err := r.repo.FindAPIKeyByHash(ctx, HashAPIKey(raw))
	if err != nil {
		return nil, err
	}
	return fromMongoAPIKey(rec), nil
}

func (r *mongodbMetadataRepository) ListVisiblePackages(in PackageFilter) ([]*RegistryPackage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), metadataOpTimeout)
	defer cancel()
	items, err := r.repo.ListVisiblePackages(ctx, toMongoPackageFilter(in))
	if err != nil {
		return nil, err
	}
	out := make([]*RegistryPackage, 0, len(items))
	for _, item := range items {
		out = append(out, fromMongoPackage(item))
	}
	return out, nil
}

func (r *mongodbMetadataRepository) GetVisiblePackage(tenantID, namespace, name string, includePublic bool) (*RegistryPackage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), metadataOpTimeout)
	defer cancel()
	pkg, err := r.repo.GetVisiblePackage(ctx, tenantID, namespace, name, includePublic)
	if err != nil {
		return nil, err
	}
	return fromMongoPackage(pkg), nil
}

func (r *mongodbMetadataRepository) CreatePackage(in CreatePackageInput) (*RegistryPackage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), metadataOpTimeout)
	defer cancel()
	pkg, err := r.repo.CreatePackage(ctx, toMongoCreatePackageInput(in))
	if err != nil {
		return nil, err
	}
	return fromMongoPackage(pkg), nil
}

func (r *mongodbMetadataRepository) PublishPackageVersion(in PublishPackageVersionInput) (*RegistryPackageVersion, error) {
	ctx, cancel := context.WithTimeout(context.Background(), metadataOpTimeout)
	defer cancel()
	rec, err := r.repo.PublishPackageVersion(ctx, toMongoPublishInput(in))
	if err != nil {
		return nil, err
	}
	return fromMongoPackageVersion(rec), nil
}

func (r *mongodbMetadataRepository) ListPackageVersions(packageID string) ([]*RegistryPackageVersion, error) {
	ctx, cancel := context.WithTimeout(context.Background(), metadataOpTimeout)
	defer cancel()
	items, err := r.repo.ListPackageVersions(ctx, packageID)
	if err != nil {
		return nil, err
	}
	out := make([]*RegistryPackageVersion, 0, len(items))
	for _, item := range items {
		out = append(out, fromMongoPackageVersion(item))
	}
	return out, nil
}

func (r *mongodbMetadataRepository) GetPackageVersion(packageID, version string) (*RegistryPackageVersion, error) {
	ctx, cancel := context.WithTimeout(context.Background(), metadataOpTimeout)
	defer cancel()
	rec, err := r.repo.GetPackageVersion(ctx, packageID, version)
	if err != nil {
		return nil, err
	}
	return fromMongoPackageVersion(rec), nil
}

func (r *mongodbMetadataRepository) UpdatePackageVisibility(in UpdatePackageVisibilityInput) (*RegistryPackage, error) {
	ctx, cancel := context.WithTimeout(context.Background(), metadataOpTimeout)
	defer cancel()
	pkg, err := r.repo.UpdatePackageVisibility(ctx, toMongoUpdateVisibilityInput(in))
	if err != nil {
		return nil, err
	}
	return fromMongoPackage(pkg), nil
}

func (r *mongodbMetadataRepository) DeletePackage(in DeletePackageInput) error {
	ctx, cancel := context.WithTimeout(context.Background(), metadataOpTimeout)
	defer cancel()
	return r.repo.DeletePackage(ctx, toMongoDeletePackageInput(in))
}

func (r *mongodbMetadataRepository) RecordAudit(ev AuditEvent) error {
	ctx, cancel := context.WithTimeout(context.Background(), metadataOpTimeout)
	defer cancel()
	return r.repo.RecordAudit(ctx, mongorepo.AuditEvent{
		Time:      ev.Time,
		Action:    ev.Action,
		ActorID:   ev.ActorID,
		TenantID:  ev.TenantID,
		Status:    ev.Status,
		RequestID: ev.RequestID,
		Details:   cloneMap(ev.Details),
	})
}

func (r *mongodbMetadataRepository) ListAudit(limit int) ([]AuditEvent, error) {
	ctx, cancel := context.WithTimeout(context.Background(), metadataOpTimeout)
	defer cancel()
	items, err := r.repo.ListAudit(ctx, limit)
	if err != nil {
		return nil, err
	}
	out := make([]AuditEvent, 0, len(items))
	for _, item := range items {
		out = append(out, AuditEvent{
			Time:      item.Time,
			Action:    item.Action,
			ActorID:   item.ActorID,
			TenantID:  item.TenantID,
			Status:    item.Status,
			RequestID: item.RequestID,
			Details:   cloneMap(item.Details),
		})
	}
	return out, nil
}

func fromMongoOrg(in *mongorepo.Organization) *Organization {
	if in == nil {
		return nil
	}
	members := make([]OrgMember, 0, len(in.Members))
	for _, m := range in.Members {
		members = append(members, OrgMember{UserID: m.UserID, Email: m.Email, Role: m.Role})
	}
	return &Organization{
		ID:          in.ID,
		Slug:        in.Slug,
		Name:        in.Name,
		Description: in.Description,
		Visibility:  in.Visibility,
		CreatedBy:   in.CreatedBy,
		CreatedAt:   in.CreatedAt,
		UpdatedAt:   in.UpdatedAt,
		Members:     members,
	}
}

func mapMongoOrgErr(err error) error {
	if errors.Is(err, mongorepo.ErrOrgNotFound) {
		return fmt.Errorf("organization not found")
	}
	return err
}

func (r *mongodbMetadataRepository) CreateOrganization(in CreateOrganizationInput) (*Organization, error) {
	slug := normalizeOrgSlug(in.Slug)
	name := strings.TrimSpace(in.Name)
	if name == "" {
		name = slug
	}
	createdBy := strings.TrimSpace(in.CreatedBy)
	if slug == "" || createdBy == "" {
		return nil, fmt.Errorf("slug and actor are required")
	}
	visibility := normalizeVisibility(in.Visibility)
	if visibility == "" {
		visibility = VisibilityPublic
	}
	now := time.Now().UTC().Format(time.RFC3339)
	doc := mongorepo.Organization{
		ID:          fmt.Sprintf("org_%d", time.Now().UTC().UnixNano()),
		Slug:        slug,
		Name:        name,
		Description: strings.TrimSpace(in.Description),
		Visibility:  visibility,
		CreatedBy:   createdBy,
		CreatedAt:   now,
		UpdatedAt:   now,
		Members:     []mongorepo.OrgMember{{UserID: createdBy, Email: strings.TrimSpace(in.CreatedByEmail), Role: OrgRoleOwner}},
	}
	ctx, cancel := context.WithTimeout(context.Background(), metadataOpTimeout)
	defer cancel()
	saved, err := r.repo.CreateOrganization(ctx, doc)
	if err != nil {
		return nil, err
	}
	return fromMongoOrg(saved), nil
}

func (r *mongodbMetadataRepository) ListOrganizations(in ListOrganizationsInput) ([]*Organization, error) {
	ctx, cancel := context.WithTimeout(context.Background(), metadataOpTimeout)
	defer cancel()
	orgs, err := r.repo.ListOrganizations(ctx, strings.TrimSpace(in.MemberUserID))
	if err != nil {
		return nil, err
	}
	out := make([]*Organization, 0, len(orgs))
	for _, org := range orgs {
		out = append(out, fromMongoOrg(org))
	}
	return out, nil
}

func (r *mongodbMetadataRepository) GetOrganization(slug string) (*Organization, error) {
	ctx, cancel := context.WithTimeout(context.Background(), metadataOpTimeout)
	defer cancel()
	org, err := r.repo.GetOrganization(ctx, slug)
	if err != nil {
		return nil, mapMongoOrgErr(err)
	}
	return fromMongoOrg(org), nil
}

func (r *mongodbMetadataRepository) AddOrganizationMember(in OrgMemberInput) (*Organization, error) {
	slug := normalizeOrgSlug(in.Slug)
	userID := strings.TrimSpace(in.UserID)
	role := normalizeOrgRole(in.Role)
	if slug == "" || userID == "" {
		return nil, fmt.Errorf("slug and user are required")
	}
	if role == "" {
		return nil, fmt.Errorf("role must be owner, admin, or developer")
	}
	ctx, cancel := context.WithTimeout(context.Background(), metadataOpTimeout)
	defer cancel()
	org, err := r.repo.GetOrganization(ctx, slug)
	if err != nil {
		return nil, mapMongoOrgErr(err)
	}
	members := org.Members
	updated := false
	for i := range members {
		if members[i].UserID == userID {
			members[i].Role = role
			if strings.TrimSpace(in.Email) != "" {
				members[i].Email = strings.TrimSpace(in.Email)
			}
			updated = true
			break
		}
	}
	if !updated {
		members = append(members, mongorepo.OrgMember{UserID: userID, Email: strings.TrimSpace(in.Email), Role: role})
	}
	saved, err := r.repo.SetMembers(ctx, slug, members, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return nil, mapMongoOrgErr(err)
	}
	return fromMongoOrg(saved), nil
}

func (r *mongodbMetadataRepository) RemoveOrganizationMember(slug, userID string) (*Organization, error) {
	slug = normalizeOrgSlug(slug)
	userID = strings.TrimSpace(userID)
	ctx, cancel := context.WithTimeout(context.Background(), metadataOpTimeout)
	defer cancel()
	org, err := r.repo.GetOrganization(ctx, slug)
	if err != nil {
		return nil, mapMongoOrgErr(err)
	}
	owners := 0
	for _, m := range org.Members {
		if m.Role == OrgRoleOwner {
			owners++
		}
	}
	kept := make([]mongorepo.OrgMember, 0, len(org.Members))
	removed := false
	for _, m := range org.Members {
		if m.UserID == userID {
			if m.Role == OrgRoleOwner && owners <= 1 {
				return nil, fmt.Errorf("cannot remove the last owner")
			}
			removed = true
			continue
		}
		kept = append(kept, m)
	}
	if !removed {
		return nil, fmt.Errorf("member not found")
	}
	saved, err := r.repo.SetMembers(ctx, slug, kept, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return nil, mapMongoOrgErr(err)
	}
	return fromMongoOrg(saved), nil
}

func toPGPackageFilter(in PackageFilter) pgrepo.PackageFilter {
	return pgrepo.PackageFilter{
		TenantID:      in.TenantID,
		IncludePublic: in.IncludePublic,
		PublicOnly:    in.PublicOnly,
		Namespace:     in.Namespace,
		Query:         in.Query,
	}
}

func toPGCreatePackageInput(in CreatePackageInput) pgrepo.CreatePackageInput {
	return pgrepo.CreatePackageInput{
		TenantID:   in.TenantID,
		Namespace:  in.Namespace,
		Name:       in.Name,
		Visibility: in.Visibility,
		CreatedBy:  in.CreatedBy,
	}
}

func toPGPublishInput(in PublishPackageVersionInput) pgrepo.PublishPackageVersionInput {
	return pgrepo.PublishPackageVersionInput{
		TenantID:    in.TenantID,
		Namespace:   in.Namespace,
		Name:        in.Name,
		Version:     in.Version,
		Manifest:    cloneMap(in.Manifest),
		ArtifactKey: in.ArtifactKey,
		Checksum:    in.Checksum,
		SizeBytes:   in.SizeBytes,
		PublishedBy: in.PublishedBy,
	}
}

func toPGUpdateVisibilityInput(in UpdatePackageVisibilityInput) pgrepo.UpdatePackageVisibilityInput {
	return pgrepo.UpdatePackageVisibilityInput{
		TenantID:   in.TenantID,
		Namespace:  in.Namespace,
		Name:       in.Name,
		Visibility: in.Visibility,
	}
}

func toPGDeletePackageInput(in DeletePackageInput) pgrepo.DeletePackageInput {
	return pgrepo.DeletePackageInput{
		TenantID:  in.TenantID,
		Namespace: in.Namespace,
		Name:      in.Name,
	}
}

func fromPGPackage(in *pgrepo.Package) *RegistryPackage {
	if in == nil {
		return nil
	}
	return &RegistryPackage{
		ID:            in.ID,
		TenantID:      in.TenantID,
		Namespace:     in.Namespace,
		Name:          in.Name,
		LatestVersion: in.LatestVersion,
		Visibility:    in.Visibility,
		CreatedBy:     in.CreatedBy,
		CreatedAt:     in.CreatedAt,
		UpdatedAt:     in.UpdatedAt,
	}
}

func fromPGPackageVersion(in *pgrepo.PackageVersion) *RegistryPackageVersion {
	if in == nil {
		return nil
	}
	return &RegistryPackageVersion{
		ID:           in.ID,
		PackageID:    in.PackageID,
		TenantID:     in.TenantID,
		Version:      in.Version,
		ManifestJSON: cloneMap(in.ManifestJSON),
		ArtifactKey:  in.ArtifactKey,
		Checksum:     in.Checksum,
		SizeBytes:    in.SizeBytes,
		PublishedBy:  in.PublishedBy,
		PublishedAt:  in.PublishedAt,
	}
}

func fromPGAPIKey(in *pgrepo.APIKeyRecord) *APIKeyRecord {
	if in == nil {
		return nil
	}
	return &APIKeyRecord{
		ID:        in.ID,
		KeyHash:   in.KeyHash,
		UserID:    in.UserID,
		TenantID:  in.TenantID,
		Roles:     append([]string(nil), in.Roles...),
		ExpiresAt: in.ExpiresAt,
		RevokedAt: in.RevokedAt,
		CreatedAt: in.CreatedAt,
	}
}

func toMongoPackageFilter(in PackageFilter) mongorepo.PackageFilter {
	return mongorepo.PackageFilter{
		TenantID:      in.TenantID,
		IncludePublic: in.IncludePublic,
		PublicOnly:    in.PublicOnly,
		Namespace:     in.Namespace,
		Query:         in.Query,
	}
}

func toMongoCreatePackageInput(in CreatePackageInput) mongorepo.CreatePackageInput {
	return mongorepo.CreatePackageInput{
		TenantID:   in.TenantID,
		Namespace:  in.Namespace,
		Name:       in.Name,
		Visibility: in.Visibility,
		CreatedBy:  in.CreatedBy,
	}
}

func toMongoPublishInput(in PublishPackageVersionInput) mongorepo.PublishPackageVersionInput {
	return mongorepo.PublishPackageVersionInput{
		TenantID:    in.TenantID,
		Namespace:   in.Namespace,
		Name:        in.Name,
		Version:     in.Version,
		Manifest:    cloneMap(in.Manifest),
		ArtifactKey: in.ArtifactKey,
		Checksum:    in.Checksum,
		SizeBytes:   in.SizeBytes,
		PublishedBy: in.PublishedBy,
	}
}

func toMongoUpdateVisibilityInput(in UpdatePackageVisibilityInput) mongorepo.UpdatePackageVisibilityInput {
	return mongorepo.UpdatePackageVisibilityInput{
		TenantID:   in.TenantID,
		Namespace:  in.Namespace,
		Name:       in.Name,
		Visibility: in.Visibility,
	}
}

func toMongoDeletePackageInput(in DeletePackageInput) mongorepo.DeletePackageInput {
	return mongorepo.DeletePackageInput{
		TenantID:  in.TenantID,
		Namespace: in.Namespace,
		Name:      in.Name,
	}
}

func fromMongoPackage(in *mongorepo.Package) *RegistryPackage {
	if in == nil {
		return nil
	}
	return &RegistryPackage{
		ID:            in.ID,
		TenantID:      in.TenantID,
		Namespace:     in.Namespace,
		Name:          in.Name,
		LatestVersion: in.LatestVersion,
		Visibility:    in.Visibility,
		CreatedBy:     in.CreatedBy,
		CreatedAt:     in.CreatedAt,
		UpdatedAt:     in.UpdatedAt,
	}
}

func fromMongoPackageVersion(in *mongorepo.PackageVersion) *RegistryPackageVersion {
	if in == nil {
		return nil
	}
	return &RegistryPackageVersion{
		ID:           in.ID,
		PackageID:    in.PackageID,
		TenantID:     in.TenantID,
		Version:      in.Version,
		ManifestJSON: cloneMap(in.ManifestJSON),
		ArtifactKey:  in.ArtifactKey,
		Checksum:     in.Checksum,
		SizeBytes:    in.SizeBytes,
		PublishedBy:  in.PublishedBy,
		PublishedAt:  in.PublishedAt,
	}
}

func fromMongoAPIKey(in *mongorepo.APIKeyRecord) *APIKeyRecord {
	if in == nil {
		return nil
	}
	return &APIKeyRecord{
		ID:        in.ID,
		KeyHash:   in.KeyHash,
		UserID:    in.UserID,
		TenantID:  in.TenantID,
		Roles:     append([]string(nil), in.Roles...),
		ExpiresAt: in.ExpiresAt,
		RevokedAt: in.RevokedAt,
		CreatedAt: in.CreatedAt,
	}
}

func (s *Store) metadataRepo() (MetadataRepository, error) {
	if s == nil || s.metadata == nil {
		return nil, fmt.Errorf("metadata repository is not configured")
	}
	return s.metadata, nil
}
