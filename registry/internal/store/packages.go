package store

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

const (
	VisibilityPublic = "public"
	VisibilityTenant = "tenant"
)

type RegistryPackage struct {
	ID            string `json:"id"`
	TenantID      string `json:"tenant_id"`
	Namespace     string `json:"namespace"`
	Name          string `json:"name"`
	LatestVersion string `json:"latest_version,omitempty"`
	Visibility    string `json:"visibility"`
	CreatedBy     string `json:"created_by"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

type RegistryPackageVersion struct {
	ID           string         `json:"id"`
	PackageID    string         `json:"package_id"`
	TenantID     string         `json:"tenant_id"`
	Version      string         `json:"version"`
	ManifestJSON map[string]any `json:"manifest_json,omitempty"`
	ArtifactKey  string         `json:"artifact_key"`
	Checksum     string         `json:"checksum,omitempty"`
	SizeBytes    int64          `json:"size_bytes,omitempty"`
	PublishedBy  string         `json:"published_by"`
	PublishedAt  string         `json:"published_at"`
}

type APIKeyRecord struct {
	ID        string   `json:"id"`
	KeyHash   string   `json:"key_hash"`
	UserID    string   `json:"user_id"`
	TenantID  string   `json:"tenant_id"`
	Roles     []string `json:"roles"`
	ExpiresAt string   `json:"expires_at,omitempty"`
	RevokedAt string   `json:"revoked_at,omitempty"`
	CreatedAt string   `json:"created_at"`
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

func HashAPIKey(raw string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(raw)))
	return hex.EncodeToString(sum[:])
}

func (s *Store) FindAPIKey(raw string) (*APIKeyRecord, error) {
	repo, err := s.metadataRepo()
	if err != nil {
		return nil, err
	}
	return repo.FindAPIKey(raw)
}

func (s *Store) findAPIKeyJSON(raw string) (*APIKeyRecord, error) {
	keyHash := HashAPIKey(raw)
	s.mu.RLock()
	defer s.mu.RUnlock()
	rec, ok := s.data.APIKeys[keyHash]
	if !ok || rec == nil {
		return nil, fmt.Errorf("api key not found")
	}
	if strings.TrimSpace(rec.RevokedAt) != "" {
		return nil, fmt.Errorf("api key revoked")
	}
	if strings.TrimSpace(rec.ExpiresAt) != "" {
		if exp, err := time.Parse(time.RFC3339, rec.ExpiresAt); err == nil && time.Now().UTC().After(exp) {
			return nil, fmt.Errorf("api key expired")
		}
	}
	cp := *rec
	cp.Roles = append([]string(nil), rec.Roles...)
	return &cp, nil
}

func (s *Store) ListVisiblePackages(in PackageFilter) ([]*RegistryPackage, error) {
	repo, err := s.metadataRepo()
	if err != nil {
		return nil, err
	}
	return repo.ListVisiblePackages(in)
}

func (s *Store) listVisiblePackagesJSON(in PackageFilter) ([]*RegistryPackage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*RegistryPackage, 0, len(s.data.Packages))
	q := strings.ToLower(strings.TrimSpace(in.Query))
	ns := strings.ToLower(strings.TrimSpace(in.Namespace))
	for _, pkg := range s.data.Packages {
		if !pkgVisibleTo(pkg, in.TenantID, in.IncludePublic, in.PublicOnly) {
			continue
		}
		if ns != "" && strings.ToLower(pkg.Namespace) != ns {
			continue
		}
		if q != "" {
			hay := strings.ToLower(pkg.Namespace + "/" + pkg.Name)
			if !strings.Contains(hay, q) {
				continue
			}
		}
		cp := *pkg
		out = append(out, &cp)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].UpdatedAt != out[j].UpdatedAt {
			return out[i].UpdatedAt > out[j].UpdatedAt
		}
		if out[i].Namespace != out[j].Namespace {
			return out[i].Namespace < out[j].Namespace
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func (s *Store) GetVisiblePackage(tenantID, namespace, name string, includePublic bool) (*RegistryPackage, error) {
	repo, err := s.metadataRepo()
	if err != nil {
		return nil, err
	}
	return repo.GetVisiblePackage(tenantID, namespace, name, includePublic)
}

func (s *Store) getVisiblePackageJSON(tenantID, namespace, name string, includePublic bool) (*RegistryPackage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	pkg := findPackageLocked(s.data.Packages, tenantID, namespace, name)
	if pkg != nil {
		cp := *pkg
		return &cp, nil
	}
	if !includePublic {
		return nil, fmt.Errorf("package not found")
	}
	var candidate *RegistryPackage
	for _, p := range s.data.Packages {
		if !strings.EqualFold(p.Namespace, strings.TrimSpace(namespace)) || !strings.EqualFold(p.Name, strings.TrimSpace(name)) {
			continue
		}
		if p.Visibility != VisibilityPublic {
			continue
		}
		if candidate == nil || p.UpdatedAt > candidate.UpdatedAt {
			candidate = p
		}
	}
	if candidate == nil {
		return nil, fmt.Errorf("package not found")
	}
	cp := *candidate
	return &cp, nil
}

func (s *Store) CreatePackage(in CreatePackageInput) (*RegistryPackage, error) {
	repo, err := s.metadataRepo()
	if err != nil {
		return nil, err
	}
	return repo.CreatePackage(in)
}

func (s *Store) createPackageJSON(in CreatePackageInput) (*RegistryPackage, error) {
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
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing := findPackageLocked(s.data.Packages, tenantID, namespace, name); existing != nil {
		return nil, fmt.Errorf("package already exists")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	pkg := &RegistryPackage{
		ID:         fmt.Sprintf("pkg_%d", time.Now().UTC().UnixNano()),
		TenantID:   tenantID,
		Namespace:  namespace,
		Name:       name,
		Visibility: visibility,
		CreatedBy:  createdBy,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	s.data.Packages[pkg.ID] = pkg
	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	cp := *pkg
	return &cp, nil
}

func (s *Store) PublishPackageVersion(in PublishPackageVersionInput) (*RegistryPackageVersion, error) {
	repo, err := s.metadataRepo()
	if err != nil {
		return nil, err
	}
	return repo.PublishPackageVersion(in)
}

func (s *Store) publishPackageVersionJSON(in PublishPackageVersionInput) (*RegistryPackageVersion, error) {
	tenantID := strings.TrimSpace(in.TenantID)
	namespace := normalizeSegment(in.Namespace)
	name := normalizeSegment(in.Name)
	version := strings.TrimSpace(in.Version)
	publishedBy := strings.TrimSpace(in.PublishedBy)
	if tenantID == "" || namespace == "" || name == "" || version == "" || publishedBy == "" {
		return nil, fmt.Errorf("tenant_id, namespace, name, version, and actor are required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	pkg := findPackageLocked(s.data.Packages, tenantID, namespace, name)
	if pkg == nil {
		return nil, fmt.Errorf("package not found")
	}
	if pkg.TenantID != tenantID {
		return nil, fmt.Errorf("tenant mismatch")
	}
	if s.data.PackageVersions[pkg.ID] == nil {
		s.data.PackageVersions[pkg.ID] = map[string]*RegistryPackageVersion{}
	}
	if _, exists := s.data.PackageVersions[pkg.ID][version]; exists {
		return nil, fmt.Errorf("version already exists")
	}
	artifactKey := canonicalArtifactKey(tenantID, namespace, name, version)
	now := time.Now().UTC().Format(time.RFC3339)
	rec := &RegistryPackageVersion{
		ID:           fmt.Sprintf("pkgv_%d", time.Now().UTC().UnixNano()),
		PackageID:    pkg.ID,
		TenantID:     tenantID,
		Version:      version,
		ManifestJSON: cloneMap(in.Manifest),
		ArtifactKey:  artifactKey,
		Checksum:     strings.TrimSpace(in.Checksum),
		SizeBytes:    in.SizeBytes,
		PublishedBy:  publishedBy,
		PublishedAt:  now,
	}
	s.data.PackageVersions[pkg.ID][version] = rec
	pkg.LatestVersion = pickLatestVersion(pkg.LatestVersion, version)
	pkg.UpdatedAt = now
	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	cp := *rec
	cp.ManifestJSON = cloneMap(rec.ManifestJSON)
	return &cp, nil
}

func (s *Store) ListPackageVersions(packageID string) ([]*RegistryPackageVersion, error) {
	repo, err := s.metadataRepo()
	if err != nil {
		return nil, err
	}
	return repo.ListPackageVersions(packageID)
}

func (s *Store) listPackageVersionsJSON(packageID string) ([]*RegistryPackageVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	versions := s.data.PackageVersions[strings.TrimSpace(packageID)]
	out := make([]*RegistryPackageVersion, 0, len(versions))
	for _, v := range versions {
		cp := *v
		cp.ManifestJSON = cloneMap(v.ManifestJSON)
		out = append(out, &cp)
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

func (s *Store) GetPackageVersion(packageID, version string) (*RegistryPackageVersion, error) {
	repo, err := s.metadataRepo()
	if err != nil {
		return nil, err
	}
	return repo.GetPackageVersion(packageID, version)
}

func (s *Store) getPackageVersionJSON(packageID, version string) (*RegistryPackageVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	versions := s.data.PackageVersions[strings.TrimSpace(packageID)]
	rec, ok := versions[strings.TrimSpace(version)]
	if !ok || rec == nil {
		return nil, fmt.Errorf("version not found")
	}
	cp := *rec
	cp.ManifestJSON = cloneMap(rec.ManifestJSON)
	return &cp, nil
}

func (s *Store) UpdatePackageVisibility(in UpdatePackageVisibilityInput) (*RegistryPackage, error) {
	repo, err := s.metadataRepo()
	if err != nil {
		return nil, err
	}
	return repo.UpdatePackageVisibility(in)
}

func (s *Store) updatePackageVisibilityJSON(in UpdatePackageVisibilityInput) (*RegistryPackage, error) {
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
	s.mu.Lock()
	defer s.mu.Unlock()
	pkg := findPackageLocked(s.data.Packages, tenantID, namespace, name)
	if pkg == nil {
		return nil, fmt.Errorf("package not found")
	}
	if pkg.TenantID != tenantID {
		return nil, fmt.Errorf("tenant mismatch")
	}
	pkg.Visibility = visibility
	pkg.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	cp := *pkg
	return &cp, nil
}

func (s *Store) DeletePackage(in DeletePackageInput) error {
	repo, err := s.metadataRepo()
	if err != nil {
		return err
	}
	return repo.DeletePackage(in)
}

func (s *Store) deletePackageJSON(in DeletePackageInput) error {
	tenantID := strings.TrimSpace(in.TenantID)
	namespace := normalizeSegment(in.Namespace)
	name := normalizeSegment(in.Name)
	if tenantID == "" || namespace == "" || name == "" {
		return fmt.Errorf("tenant_id, namespace, and name are required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	pkg := findPackageLocked(s.data.Packages, tenantID, namespace, name)
	if pkg == nil {
		return fmt.Errorf("package not found")
	}
	if pkg.TenantID != tenantID {
		return fmt.Errorf("tenant mismatch")
	}
	delete(s.data.PackageVersions, pkg.ID)
	delete(s.data.Packages, pkg.ID)
	return s.saveLocked()
}

func pkgVisibleTo(pkg *RegistryPackage, tenantID string, includePublic, publicOnly bool) bool {
	if pkg == nil {
		return false
	}
	if publicOnly {
		return pkg.Visibility == VisibilityPublic
	}
	if strings.TrimSpace(tenantID) != "" && pkg.TenantID == strings.TrimSpace(tenantID) {
		return true
	}
	return includePublic && pkg.Visibility == VisibilityPublic
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

func findPackageLocked(pkgs map[string]*RegistryPackage, tenantID, namespace, name string) *RegistryPackage {
	tenantID = strings.TrimSpace(tenantID)
	namespace = normalizeSegment(namespace)
	name = normalizeSegment(name)
	for _, pkg := range pkgs {
		if pkg.TenantID == tenantID && strings.EqualFold(pkg.Namespace, namespace) && strings.EqualFold(pkg.Name, name) {
			return pkg
		}
	}
	return nil
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
