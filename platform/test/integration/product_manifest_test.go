package integration

// This test promotes the two throwaway scratchpad harnesses (`manifestcheck/`
// and `backendcheck/`) from the July 2026 session into CI. They proved that the
// manifests the product's workspace-builder emits survive the framework's real
// loader + Validate + Resolve for every backend kind, and that the gcs/azblob
// prefix requirement (which the product defaults to "runfabric") is genuinely
// enforced by Validate.
//
// The manifest strings below mirror product/apps/app-service/src/modules/
// runfabric/application/workspace-builder.service.ts (buildManifest +
// appendBackend): list-format functions using `env:` (NOT `environment:` —
// only the canonical map format accepts that alias), and backend blocks with
// exactly the keys the generator writes. If the product generator or the
// framework loader drift apart, this test fails first.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/runfabric/runfabric/platform/core/model/config"
)

// productManifest renders a manifest the way the product's workspace-builder
// does: a list-format function with `env:` and an http trigger, optionally
// followed by a backend block. backendBlock is the raw YAML for the `backend:`
// section (already indented) or "" for a local/no backend.
func productManifest(backendBlock string) string {
	var b strings.Builder
	b.WriteString("service: sample-svc\n")
	b.WriteString("provider:\n")
	b.WriteString("  name: aws-lambda\n")
	b.WriteString("  runtime: nodejs\n")
	if backendBlock != "" {
		b.WriteString(backendBlock)
	}
	b.WriteString("functions:\n")
	b.WriteString("  - name: api\n")
	b.WriteString("    runtime: nodejs\n")
	b.WriteString("    entry: dist/api.handler\n")
	b.WriteString("    env:\n")
	b.WriteString("      LOG_LEVEL: info\n")
	b.WriteString("    triggers:\n")
	b.WriteString("      - type: http\n")
	b.WriteString("        method: get\n")
	b.WriteString("        path: /api\n")
	return b.String()
}

// loadValidateResolve runs the full pipeline the daemon runs on a submitted
// manifest, returning the first error encountered.
func loadValidateResolve(t *testing.T, manifest, stage string) (*config.Config, error) {
	t.Helper()
	cfg, err := config.LoadFromBytes([]byte(manifest))
	if err != nil {
		return nil, fmt.Errorf("load: %w", err)
	}
	if err := config.Validate(cfg); err != nil {
		return nil, fmt.Errorf("validate: %w", err)
	}
	resolved, err := config.Resolve(cfg, stage)
	if err != nil {
		return nil, fmt.Errorf("resolve: %w", err)
	}
	return resolved, nil
}

// TestProductManifestAllBackendKinds is the `backendcheck/` harness: every
// backend kind the product's StorageBackendFields offers must load, validate,
// and resolve when embedded in a product-shaped manifest, with the backend
// fields preserved through the pipeline.
func TestProductManifestAllBackendKinds(t *testing.T) {
	cases := []struct {
		name     string
		block    string
		wantKind string
		check    func(t *testing.T, b *config.BackendConfig)
	}{
		{
			name:     "local (omitted backend block)",
			block:    "",
			wantKind: "",
		},
		{
			name: "s3",
			block: "backend:\n" +
				"  kind: s3\n" +
				"  s3Bucket: my-state-bucket\n" +
				"  s3Prefix: runfabric\n",
			wantKind: "s3",
			check: func(t *testing.T, b *config.BackendConfig) {
				if b.S3Bucket != "my-state-bucket" || b.S3Prefix != "runfabric" {
					t.Errorf("s3 fields lost: %+v", b)
				}
			},
		},
		{
			name: "gcs",
			block: "backend:\n" +
				"  kind: gcs\n" +
				"  gcsBucket: my-state-bucket\n" +
				"  gcsPrefix: runfabric\n",
			wantKind: "gcs",
			check: func(t *testing.T, b *config.BackendConfig) {
				if b.GCSBucket != "my-state-bucket" || b.GCSPrefix != "runfabric" {
					t.Errorf("gcs fields lost: %+v", b)
				}
			},
		},
		{
			name: "azblob",
			block: "backend:\n" +
				"  kind: azblob\n" +
				"  azblobContainer: state\n" +
				"  azblobPrefix: runfabric\n",
			wantKind: "azblob",
			check: func(t *testing.T, b *config.BackendConfig) {
				if b.AzblobContainer != "state" || b.AzblobPrefix != "runfabric" {
					t.Errorf("azblob fields lost: %+v", b)
				}
			},
		},
		{
			name: "postgres (defaults filled by Validate)",
			block: "backend:\n" +
				"  kind: postgres\n",
			wantKind: "postgres",
			check: func(t *testing.T, b *config.BackendConfig) {
				// Validate backfills the default DSN env key and table.
				if b.PostgresConnectionStringEnv != "RUNFABRIC_STATE_POSTGRES_URL" {
					t.Errorf("postgres DSN env default not applied: %+v", b)
				}
				if b.PostgresTable != "runfabric_receipts" {
					t.Errorf("postgres table default not applied: %+v", b)
				}
			},
		},
		{
			name: "dynamodb",
			block: "backend:\n" +
				"  kind: dynamodb\n" +
				"  lockTable: runfabric-locks\n",
			wantKind: "dynamodb",
			check: func(t *testing.T, b *config.BackendConfig) {
				if b.LockTable != "runfabric-locks" {
					t.Errorf("dynamodb lockTable lost: %+v", b)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resolved, err := loadValidateResolve(t, productManifest(tc.block), "dev")
			if err != nil {
				t.Fatalf("product manifest rejected: %v", err)
			}
			// The list-format function with `env:` must survive to a resolved function.
			fn, ok := resolved.Functions["api"]
			if !ok {
				t.Fatalf("function api not resolved: %+v", resolved.Functions)
			}
			if fn.Environment["LOG_LEVEL"] != "info" {
				t.Errorf("list-format env: not carried through: %v", fn.Environment)
			}
			if len(fn.Events) != 1 || fn.Events[0].HTTP == nil {
				t.Errorf("http trigger lost: %+v", fn.Events)
			}
			if tc.wantKind == "" {
				if resolved.Backend != nil && resolved.Backend.Kind != "" {
					t.Errorf("expected no backend, got %+v", resolved.Backend)
				}
				return
			}
			if resolved.Backend == nil {
				t.Fatalf("backend dropped through pipeline")
			}
			if resolved.Backend.Kind != tc.wantKind {
				t.Fatalf("backend kind=%q want %q", resolved.Backend.Kind, tc.wantKind)
			}
			if tc.check != nil {
				tc.check(t, resolved.Backend)
			}
		})
	}
}

// TestProductManifestPrefixRejection is the `manifestcheck/` regression: the
// framework hard-requires a prefix for gcs/azblob. The product defaults an
// empty UI field to "runfabric" precisely because a prefix-less manifest must
// NOT reach deploy — it must fail Validate. These cases lock that in: if the
// requirement were ever relaxed, the product's defaulting would become silent
// dead code and this test would flag the change.
func TestProductManifestPrefixRejection(t *testing.T) {
	cases := []struct {
		name    string
		block   string
		wantErr string
	}{
		{
			name: "gcs without prefix",
			block: "backend:\n" +
				"  kind: gcs\n" +
				"  gcsBucket: my-state-bucket\n",
			wantErr: "backend.gcsPrefix is required",
		},
		{
			name: "gcs without bucket",
			block: "backend:\n" +
				"  kind: gcs\n" +
				"  gcsPrefix: runfabric\n",
			wantErr: "backend.gcsBucket is required",
		},
		{
			name: "azblob without prefix",
			block: "backend:\n" +
				"  kind: azblob\n" +
				"  azblobContainer: state\n",
			wantErr: "backend.azblobPrefix is required",
		},
		{
			name: "azblob without container",
			block: "backend:\n" +
				"  kind: azblob\n" +
				"  azblobPrefix: runfabric\n",
			wantErr: "backend.azblobContainer is required",
		},
		{
			name: "s3 without bucket",
			block: "backend:\n" +
				"  kind: s3\n",
			wantErr: "backend.s3Bucket is required",
		},
		{
			name: "unsupported kind",
			block: "backend:\n" +
				"  kind: bogus\n",
			wantErr: "unsupported backend.kind",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := loadValidateResolve(t, productManifest(tc.block), "dev")
			if err == nil {
				t.Fatalf("expected manifest to be rejected, but it passed")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}
