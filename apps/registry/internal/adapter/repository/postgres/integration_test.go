package postgres_test

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	pgrepo "github.com/runfabric/runfabric/registry/internal/adapter/repository/postgres"
)

func TestRepositoryIntegration_PostgresLifecycle(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("REGISTRY_TEST_POSTGRES_DSN"))
	if dsn == "" {
		t.Skip("set REGISTRY_TEST_POSTGRES_DSN to run Postgres integration test")
	}
	driver := strings.TrimSpace(os.Getenv("REGISTRY_TEST_POSTGRES_DRIVER"))
	if driver == "" {
		driver = "pgx"
	}

	db, err := sql.Open(driver, dsn)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unknown driver") {
			t.Skipf("driver %q is not linked in this binary: %v", driver, err)
		}
		t.Fatalf("open postgres: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("ping postgres: %v", err)
	}

	repo := pgrepo.New(db)
	if err := repo.Migrate(ctx); err != nil {
		t.Fatalf("migrate postgres: %v", err)
	}
	if err := repo.SeedAPIKey(ctx, "key_test_seed", "hash_test_seed", "test-user", "tenant_test", []string{"reader"}); err != nil {
		t.Fatalf("seed api key: %v", err)
	}
	if _, err := repo.FindAPIKeyByHash(ctx, "hash_test_seed"); err != nil {
		t.Fatalf("find seeded api key: %v", err)
	}

	tenantID := fmt.Sprintf("tenant_it_%d", time.Now().UTC().UnixNano())
	pkg, err := repo.CreatePackage(ctx, pgrepo.CreatePackageInput{
		TenantID:   tenantID,
		Namespace:  "acme",
		Name:       "integration-demo",
		Visibility: pgrepo.VisibilityPublic,
		CreatedBy:  "integration-test",
	})
	if err != nil {
		t.Fatalf("create package: %v", err)
	}

	if _, err := repo.PublishPackageVersion(ctx, pgrepo.PublishPackageVersionInput{
		TenantID:    tenantID,
		Namespace:   "acme",
		Name:        "integration-demo",
		Version:     "1.0.0",
		Manifest:    map[string]any{"runtime": "nodejs"},
		PublishedBy: "integration-test",
	}); err != nil {
		t.Fatalf("publish package version: %v", err)
	}

	items, err := repo.ListVisiblePackages(ctx, pgrepo.PackageFilter{TenantID: tenantID, IncludePublic: true})
	if err != nil {
		t.Fatalf("list visible packages: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("expected at least one package in tenant scope")
	}

	versions, err := repo.ListPackageVersions(ctx, pkg.ID)
	if err != nil {
		t.Fatalf("list package versions: %v", err)
	}
	if len(versions) == 0 || versions[0].Version != "1.0.0" {
		t.Fatalf("expected published version 1.0.0, got %+v", versions)
	}

	if err := repo.DeletePackage(ctx, pgrepo.DeletePackageInput{TenantID: tenantID, Namespace: "acme", Name: "integration-demo"}); err != nil {
		t.Fatalf("delete package: %v", err)
	}
}
