package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestMetadataParity_JSON(t *testing.T) {
	runMetadataParitySuite(t, "json", OpenOptions{MetadataProvider: "json"})
}

func TestMetadataParity_Postgres(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("REGISTRY_TEST_POSTGRES_DSN"))
	if dsn == "" {
		t.Skip("set REGISTRY_TEST_POSTGRES_DSN to run Postgres metadata parity test")
	}
	driver := strings.TrimSpace(os.Getenv("REGISTRY_TEST_POSTGRES_DRIVER"))
	if driver == "" {
		driver = "pgx"
	}
	runMetadataParitySuite(t, "postgres", OpenOptions{
		MetadataProvider: "postgres",
		PostgresDSN:      dsn,
		PostgresDriver:   driver,
	})
}

func TestMetadataParity_MongoDB(t *testing.T) {
	uri := strings.TrimSpace(os.Getenv("REGISTRY_TEST_MONGODB_URI"))
	if uri == "" {
		t.Skip("set REGISTRY_TEST_MONGODB_URI to run MongoDB metadata parity test")
	}
	dbName := strings.TrimSpace(os.Getenv("REGISTRY_TEST_MONGODB_DATABASE"))
	if dbName == "" {
		dbName = "runfabric_registry_test"
	}
	runMetadataParitySuite(t, "mongodb", OpenOptions{
		MetadataProvider: "mongodb",
		MongoDBURI:       uri,
		MongoDBDatabase:  dbName,
	})
}

func runMetadataParitySuite(t *testing.T, backend string, opts OpenOptions) {
	t.Helper()
	opts.SeedLocalDevData = true
	tmp := t.TempDir()
	if strings.TrimSpace(opts.DBPath) == "" {
		opts.DBPath = filepath.Join(tmp, "registry.db.json")
	}
	if strings.TrimSpace(opts.UploadsDir) == "" {
		opts.UploadsDir = filepath.Join(tmp, "uploads")
	}

	s, err := Open(opts)
	if err != nil {
		t.Fatalf("open store for %s: %v", backend, err)
	}
	t.Cleanup(func() { _ = s.Close() })

	key, err := s.FindAPIKey("rk_local_dev")
	if err != nil {
		t.Fatalf("%s: find seeded api key: %v", backend, err)
	}
	if key.TenantID != "tenant_runfabric" {
		t.Fatalf("%s: seeded key tenant mismatch: %q", backend, key.TenantID)
	}

	runID := fmt.Sprintf("%d", time.Now().UTC().UnixNano())
	tenantA := "tenant_parity_a_" + runID
	tenantB := "tenant_parity_b_" + runID
	ns := "ns" + runID
	name := "pkg" + runID
	actor := "parity-tester"

	pkg, err := s.CreatePackage(CreatePackageInput{
		TenantID:   tenantA,
		Namespace:  ns,
		Name:       name,
		Visibility: VisibilityPublic,
		CreatedBy:  actor,
	})
	if err != nil {
		t.Fatalf("%s: create package: %v", backend, err)
	}

	if _, err := s.CreatePackage(CreatePackageInput{
		TenantID:   tenantA,
		Namespace:  ns,
		Name:       name,
		Visibility: VisibilityPublic,
		CreatedBy:  actor,
	}); err == nil {
		t.Fatalf("%s: expected duplicate package create error", backend)
	}

	v1, err := s.PublishPackageVersion(PublishPackageVersionInput{
		TenantID:    tenantA,
		Namespace:   ns,
		Name:        name,
		Version:     "1.0.0",
		Manifest:    map[string]any{"runtime": "nodejs"},
		PublishedBy: actor,
	})
	if err != nil {
		t.Fatalf("%s: publish 1.0.0: %v", backend, err)
	}
	if !strings.Contains(v1.ArtifactKey, "/"+tenantA+"/") {
		t.Fatalf("%s: artifact key not tenant-scoped: %s", backend, v1.ArtifactKey)
	}

	if _, err := s.GetPackageVersion(pkg.ID, "1.0.0"); err != nil {
		t.Fatalf("%s: get package version 1.0.0: %v", backend, err)
	}

	otherNS := "other" + runID
	otherPkg, err := s.CreatePackage(CreatePackageInput{
		TenantID:   tenantA,
		Namespace:  otherNS,
		Name:       name,
		Visibility: VisibilityPublic,
		CreatedBy:  actor,
	})
	if err != nil {
		t.Fatalf("%s: create package in secondary namespace: %v", backend, err)
	}
	if _, err := s.PublishPackageVersion(PublishPackageVersionInput{
		TenantID:    tenantA,
		Namespace:   otherNS,
		Name:        name,
		Version:     "1.0.0",
		PublishedBy: actor,
	}); err != nil {
		t.Fatalf("%s: publish secondary namespace version: %v", backend, err)
	}
	if _, err := s.GetPackageVersion(otherPkg.ID, "1.0.0"); err != nil {
		t.Fatalf("%s: get secondary namespace version 1.0.0: %v", backend, err)
	}

	filtered, err := s.ListVisiblePackages(PackageFilter{
		TenantID:      tenantB,
		IncludePublic: true,
		Namespace:     ns,
		Query:         name,
	})
	if err != nil {
		t.Fatalf("%s: list filtered packages (namespace+query): %v", backend, err)
	}
	if countPackages(filtered, tenantA, ns, name) != 1 {
		t.Fatalf("%s: expected exactly one primary namespace package in filtered result", backend)
	}
	if countPackages(filtered, tenantA, otherNS, name) != 0 {
		t.Fatalf("%s: namespace filter leaked secondary namespace package", backend)
	}

	publicList, err := s.ListVisiblePackages(PackageFilter{
		PublicOnly:    true,
		IncludePublic: true,
		Namespace:     ns,
	})
	if err != nil {
		t.Fatalf("%s: list public packages: %v", backend, err)
	}
	if !containsPackage(publicList, tenantA, ns, name) {
		t.Fatalf("%s: expected package in public list", backend)
	}

	if _, err := s.GetVisiblePackage(tenantB, ns, name, true); err != nil {
		t.Fatalf("%s: tenant_b should read public package: %v", backend, err)
	}
	if _, err := s.GetVisiblePackage(tenantB, ns, name, false); err == nil {
		t.Fatalf("%s: tenant_b should not read tenant-owned package without includePublic", backend)
	}

	if _, err := s.UpdatePackageVisibility(UpdatePackageVisibilityInput{
		TenantID:   tenantA,
		Namespace:  ns,
		Name:       name,
		Visibility: VisibilityTenant,
	}); err != nil {
		t.Fatalf("%s: update visibility tenant: %v", backend, err)
	}

	publicAfter, err := s.ListVisiblePackages(PackageFilter{
		PublicOnly:    true,
		IncludePublic: true,
		Namespace:     ns,
	})
	if err != nil {
		t.Fatalf("%s: list public packages after visibility update: %v", backend, err)
	}
	if containsPackage(publicAfter, tenantA, ns, name) {
		t.Fatalf("%s: package should not appear in public list after tenant visibility", backend)
	}

	if _, err := s.GetVisiblePackage(tenantB, ns, name, true); err == nil {
		t.Fatalf("%s: tenant_b should not read tenant visibility package", backend)
	}

	if _, err := s.PublishPackageVersion(PublishPackageVersionInput{
		TenantID:    tenantA,
		Namespace:   ns,
		Name:        name,
		Version:     "1.1.0",
		PublishedBy: actor,
	}); err != nil {
		t.Fatalf("%s: publish 1.1.0: %v", backend, err)
	}

	versions, err := s.ListPackageVersions(pkg.ID)
	if err != nil {
		t.Fatalf("%s: list versions: %v", backend, err)
	}
	if len(versions) < 2 {
		t.Fatalf("%s: expected at least two package versions, got %d", backend, len(versions))
	}
	if versions[0].Version != "1.1.0" {
		t.Fatalf("%s: expected semver-desc ordering with 1.1.0 first, got %s", backend, versions[0].Version)
	}

	if err := s.DeletePackage(DeletePackageInput{TenantID: tenantB, Namespace: ns, Name: name}); err == nil {
		t.Fatalf("%s: expected tenant isolation error on cross-tenant delete", backend)
	}

	reqID := "req_" + runID
	s.RecordAudit(AuditEvent{
		Time:      time.Now().UTC().Format(time.RFC3339),
		Action:    "metadata_parity",
		ActorID:   actor,
		TenantID:  tenantA,
		Status:    "ok",
		RequestID: reqID,
		Details:   map[string]any{"backend": backend},
	})
	audit := s.ListAudit(200)
	if !containsAuditRequestID(audit, reqID) {
		t.Fatalf("%s: expected parity audit event %s to be recorded", backend, reqID)
	}

	if err := s.DeletePackage(DeletePackageInput{TenantID: tenantA, Namespace: ns, Name: name}); err != nil {
		t.Fatalf("%s: delete package as owner: %v", backend, err)
	}
	if _, err := s.GetPackageVersion(pkg.ID, "1.0.0"); err == nil {
		t.Fatalf("%s: expected package versions removed after package delete", backend)
	}
}

func containsPackage(items []*RegistryPackage, tenantID, namespace, name string) bool {
	for _, item := range items {
		if item == nil {
			continue
		}
		if item.TenantID == tenantID && strings.EqualFold(item.Namespace, namespace) && strings.EqualFold(item.Name, name) {
			return true
		}
	}
	return false
}

func containsAuditRequestID(items []AuditEvent, requestID string) bool {
	for _, item := range items {
		if strings.TrimSpace(item.RequestID) == strings.TrimSpace(requestID) {
			return true
		}
	}
	return false
}

func countPackages(items []*RegistryPackage, tenantID, namespace, name string) int {
	count := 0
	for _, item := range items {
		if item == nil {
			continue
		}
		if item.TenantID == tenantID && strings.EqualFold(item.Namespace, namespace) && strings.EqualFold(item.Name, name) {
			count++
		}
	}
	return count
}
