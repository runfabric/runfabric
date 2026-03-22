package store

import (
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolve_DeterministicSelection(t *testing.T) {
	tmp := t.TempDir()
	s, err := Open(OpenOptions{DBPath: filepath.Join(tmp, "registry.db.json")})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	res, err := s.Resolve(ResolveInput{ID: "provider-aws", Core: "0.9.0", OS: "darwin", Arch: "arm64"})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if res.Version.Version != "1.0.0" {
		t.Fatalf("version=%q want 1.0.0", res.Version.Version)
	}
	if res.Artifact.Signature == nil {
		t.Fatal("expected signature")
	}
	want := sha256.Sum256([]byte("provider-aws@1.0.0:darwin-arm64"))
	if res.Artifact.ChecksumValue != hex.EncodeToString(want[:]) {
		t.Fatalf("checksum=%q want %q", res.Artifact.ChecksumValue, hex.EncodeToString(want[:]))
	}
}

func TestPublishFlow_FinalizeAddsResolvableVersion(t *testing.T) {
	tmp := t.TempDir()
	s, err := Open(OpenOptions{DBPath: filepath.Join(tmp, "registry.db.json")})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	body := []byte("plugin-bytes")
	sum := sha256.Sum256(body)
	sess, err := s.CreatePublishSession(PublishInitInput{
		Publisher:  "runfabric",
		ID:         "provider-demo",
		Version:    "1.0.0",
		Type:       "plugin",
		PluginKind: "provider",
		Files: []PublishFileInput{{
			Key:       "artifact",
			Name:      "plugin.bin",
			SizeBytes: int64(len(body)),
			Algorithm: "sha256",
			Checksum:  hex.EncodeToString(sum[:]),
		}},
	})
	if err != nil {
		t.Fatalf("publish init: %v", err)
	}
	if err := s.UploadPublishFile(sess.ID, "artifact", body); err != nil {
		t.Fatalf("upload: %v", err)
	}
	if _, err := s.FinalizePublish(sess.ID); err != nil {
		t.Fatalf("finalize: %v", err)
	}

	res, err := s.Resolve(ResolveInput{ID: "provider-demo", Core: "0.9.0", OS: "darwin", Arch: "arm64"})
	if err != nil {
		t.Fatalf("resolve published extension: %v", err)
	}
	if res.Version.Version != "1.0.0" {
		t.Fatalf("version=%q want 1.0.0", res.Version.Version)
	}
	if res.Artifact.URL == "" {
		t.Fatal("expected artifact URL")
	}
}

func TestPackageVisibilityAndTenantRules(t *testing.T) {
	tmp := t.TempDir()
	s, err := Open(OpenOptions{DBPath: filepath.Join(tmp, "registry.db.json")})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}

	pkg, err := s.CreatePackage(CreatePackageInput{
		TenantID:   "tenant_a",
		Namespace:  "acme",
		Name:       "demo",
		Visibility: VisibilityPublic,
		CreatedBy:  "alice",
	})
	if err != nil {
		t.Fatalf("create package: %v", err)
	}
	if _, err := s.PublishPackageVersion(PublishPackageVersionInput{
		TenantID:    "tenant_a",
		Namespace:   "acme",
		Name:        "demo",
		Version:     "1.0.0",
		PublishedBy: "alice",
	}); err != nil {
		t.Fatalf("publish version: %v", err)
	}

	list, err := s.ListVisiblePackages(PackageFilter{PublicOnly: true, IncludePublic: true})
	if err != nil {
		t.Fatalf("list public: %v", err)
	}
	if len(list) == 0 {
		t.Fatal("expected at least one public package")
	}

	if err := s.DeletePackage(DeletePackageInput{TenantID: "tenant_b", Namespace: "acme", Name: "demo"}); err == nil {
		t.Fatal("expected tenant isolation error on delete")
	}
	if err := s.DeletePackage(DeletePackageInput{TenantID: "tenant_a", Namespace: "acme", Name: "demo"}); err != nil {
		t.Fatalf("delete package: %v", err)
	}
	if _, err := s.GetPackageVersion(pkg.ID, "1.0.0"); err == nil {
		t.Fatal("expected version to be deleted with package")
	}
}

func TestFindAPIKey_LocalSeed(t *testing.T) {
	tmp := t.TempDir()
	s, err := Open(OpenOptions{
		DBPath:           filepath.Join(tmp, "registry.db.json"),
		SeedLocalDevData: true,
	})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	key, err := s.FindAPIKey("rk_local_dev")
	if err != nil {
		t.Fatalf("find api key: %v", err)
	}
	if key.TenantID != "tenant_runfabric" {
		t.Fatalf("tenant=%q want tenant_runfabric", key.TenantID)
	}
	if len(key.Roles) == 0 {
		t.Fatal("expected roles on api key")
	}
}

func TestPublishPackageVersion_ArtifactKeyCanonicalized(t *testing.T) {
	tmp := t.TempDir()
	s, err := Open(OpenOptions{DBPath: filepath.Join(tmp, "registry.db.json")})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if _, err := s.CreatePackage(CreatePackageInput{
		TenantID:   "tenant_a",
		Namespace:  "Acme",
		Name:       "Demo",
		Visibility: VisibilityPublic,
		CreatedBy:  "alice",
	}); err != nil {
		t.Fatalf("create package: %v", err)
	}
	rec, err := s.PublishPackageVersion(PublishPackageVersionInput{
		TenantID:    "tenant_a",
		Namespace:   "Acme",
		Name:        "Demo",
		Version:     "1.0.0",
		ArtifactKey: "tenants/tenant_other/packages/x/y/1.0.0/artifact.tar.gz",
		PublishedBy: "alice",
	})
	if err != nil {
		t.Fatalf("publish version: %v", err)
	}
	want := "tenants/tenant_a/packages/acme/demo/1.0.0/artifact.tar.gz"
	if rec.ArtifactKey != want {
		t.Fatalf("artifact key=%q want=%q", rec.ArtifactKey, want)
	}
}

func TestOpen_MetadataProviderJSON(t *testing.T) {
	tmp := t.TempDir()
	s, err := Open(OpenOptions{
		DBPath:           filepath.Join(tmp, "registry.db.json"),
		MetadataProvider: "json",
	})
	if err != nil {
		t.Fatalf("open store (json provider): %v", err)
	}
	if s.metadata == nil {
		t.Fatal("expected metadata repository to be configured")
	}
}

func TestOpen_MetadataProviderMongoDBRequiresURI(t *testing.T) {
	tmp := t.TempDir()
	_, err := Open(OpenOptions{
		DBPath:           filepath.Join(tmp, "registry.db.json"),
		MetadataProvider: "mongodb",
	})
	if err == nil {
		t.Fatal("expected mongodb provider configuration error")
	}
	if !strings.Contains(err.Error(), "--mongodb-uri") {
		t.Fatalf("expected mongodb uri hint in error, got: %v", err)
	}
}
