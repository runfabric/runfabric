APP=runfabric
DAEMON_APP=runfabricd
WORKER_APP=runfabricw

.PHONY: all help build build-daemon build-worker build-platform build-daemon-platform build-worker-platform build-all-platforms build-upx build-platform-upx build-all-platforms-upx build-provider-plugins install-provider-plugins build-secretmanager-plugins install-secretmanager-plugins test test-integration release-check check-syntax check-boundary check-architecture check-binary-surfaces release-tag version clean lint bin-clear-quarantine check-docs-sync pre-push doctor plan deploy remove invoke logs inspect recover unlock inspect-remote lock-steal backend-migrate init mcp-install mcp-build mcp daemon-background daemon-stop docker-daemon-build docker-daemon-tag docker-daemon-push docker-daemon-run docker-daemon-up docker-daemon-down ghcr-login registry-api docker-registry-build docker-registry-run docker-registry-stop docker-registry-up docker-registry-down audit-gaps audit-unused audit

# UPX: compress binaries for smaller distribution. Override with e.g. make build-upx UPX="upx --best"
# On macOS, UPX requires --force-macos; compressed binaries may need re-signing for notarization.
UPX_BASE ?= upx --best --lzma
UPX_OPTS_DARWIN := --force-macos
UPX = $(UPX_BASE) $(if $(filter darwin,$(GOOS)),$(UPX_OPTS_DARWIN),)

# Platform for current machine (used by build-platform and SDK bin resolution)
GOOS   := $(shell go env GOOS)
GOARCH := $(shell go env GOARCH)
BIN_SUFFIX := $(if $(filter windows,$(GOOS)),.exe,)
PLATFORM_BIN := bin/$(APP)-$(GOOS)-$(GOARCH)$(BIN_SUFFIX)
PLATFORM_DAEMON_BIN := bin/$(DAEMON_APP)-$(GOOS)-$(GOARCH)$(BIN_SUFFIX)
PLATFORM_WORKER_BIN := bin/$(WORKER_APP)-$(GOOS)-$(GOARCH)$(BIN_SUFFIX)
EXAMPLE_CONFIG ?= examples/node/hello-aws/runfabric.yml
EXAMPLE_STAGE ?= dev
AUDIT_DIR ?= .runfabric/audit

# Default target
all: build

help:
	@echo "RunFabric Makefile targets:"
	@echo "  make              same as make build"
	@echo "  make build        build binaries to bin/runfabric, bin/runfabricd, and bin/runfabricw"
	@echo "  make build-daemon build daemon binary to bin/runfabricd"
	@echo "  make build-worker build worker binary to bin/runfabricw"
	@echo "  make build-platform  build CLI to bin/runfabric-$(GOOS)-$(GOARCH)$(BIN_SUFFIX) (for SDK)"
	@echo "  make build-daemon-platform  build daemon binary to bin/runfabricd-$(GOOS)-$(GOARCH)$(BIN_SUFFIX)"
	@echo "  make build-worker-platform  build worker binary to bin/runfabricw-$(GOOS)-$(GOARCH)$(BIN_SUFFIX)"
	@echo "  make build-all-platforms  build all GoReleaser targets into bin/"
	@echo "  make build-upx    build then compress bin/runfabric with UPX"
	@echo "  make build-platform-upx   build platform binary then compress with UPX"
	@echo "  make build-all-platforms-upx  build all platforms then compress each with UPX"
	@echo "  make build-provider-plugins  build external provider plugin binaries into bin/plugins/"
	@echo "  make install-provider-plugins  install provider plugin binaries + plugin.yaml into ~/.runfabric/plugins/provider/<id>/"
	@echo "  make build-secretmanager-plugins  build secret-manager plugin binaries into bin/plugins/"
	@echo "  make install-secretmanager-plugins  install secret-manager plugin binaries + plugin.yaml into ~/.runfabric/plugins/secret-managers/<id>/<ver>/"
	@echo "  make test         run tests"
	@echo "  make coverage     run tests with coverage report (engine)"
	@echo "  make coverage-gate [COVERAGE_THRESHOLD=N]  run coverage; fail if total below N%% (omit to report only)"
	@echo "  make lint         go vet / golangci-lint"
	@echo "  make release-check  format + vet + build + test -race + build CLI + UPX (CI gate)"
	@echo "  make check-syntax   go vet + go build + go test -count=1 (no -race); fast PR feedback"
	@echo "  make check-boundary  verify extension/packages/testdata stubs have no platform/ imports"
	@echo "  make check-architecture  enforce normalized architecture flow + anti-bridge rules"
	@echo "  make check-binary-surfaces  smoke-check runfabric/runfabricd/runfabricw command ownership"
	@echo "  make check-docs-sync  verify doc links and no outdated refs (packages/planner, packages/core)"
	@echo "  make pre-push       lint + validation (used by .githooks/pre-push)"
	@echo "  make release-tag   create and push tag v\$$(cat VERSION) to trigger CI release"
	@echo "  make bin-clear-quarantine  strip macOS quarantine from bin/ (fix 'killed' when running copied binaries)"
	@echo "  make clean        remove bin/ and go caches"
	@echo "  make version      show VERSION and binary -v"
	@echo "  make doctor plan deploy ...  run runfabric commands via go run (see Makefile)"
	@echo "  make mcp-install   npm install in packages/node/mcp (MCP server)"
	@echo "  make mcp-build     build MCP server (packages/node/mcp)"
	@echo "  make mcp           run MCP server (stdio; requires runfabric on PATH)"
	@echo "  make daemon-background  start daemon in background (logs: .runfabric/daemon.log)"
	@echo "  make daemon-stop   stop daemon started with daemon-background"
	@echo "  make docker-daemon-build  build daemon Docker image (infra/Dockerfile.daemon)"
	@echo "      overrides: DAEMON_GO_BUILDER_IMAGE=... DAEMON_RUNTIME_IMAGE=..."
	@echo "  make docker-daemon-tag    tag image as ghcr.io/runfabric/runfabric-daemon:latest"
	@echo "  make docker-daemon-push  build, tag, and push daemon image to ghcr.io"
	@echo "  make ghcr-login         login to ghcr.io (requires GHCR_USER and GHCR_PAT env vars)"
	@echo "  make docker-daemon-run   run daemon container (API on port 8766)"
	@echo "  make docker-daemon-up    docker compose up daemon + Redis (infra/docker-compose.daemon.yml)"
	@echo "  make docker-daemon-down  docker compose down daemon stack"
	@echo "  make registry-api    run local extension registry API (apps/registry)"
	@echo "  make docker-registry-build  build registry Docker image (apps/registry/Dockerfile)"
	@echo "      overrides: REGISTRY_WEB_BUILDER_IMAGE=... REGISTRY_GO_BUILDER_IMAGE=... REGISTRY_RUNTIME_IMAGE=..."
	@echo "  make docker-registry-run    run registry container (port 8787)"
	@echo "  make docker-registry-stop   stop registry container"
	@echo "  make docker-registry-up     docker compose up registry stack (infra/docker-compose.registry.yml)"
	@echo "  make docker-registry-down   docker compose down registry stack"
	@echo "  make audit-gaps      scan for TODO/stub/missing-implementation signals and write $(AUDIT_DIR)/gaps.txt"
	@echo "  make audit-unused    run dangling/unused checks and write $(AUDIT_DIR)/unused.txt"
	@echo "  make audit           run both audit-gaps and audit-unused"

VERSION_FILE := $(shell cat VERSION 2>/dev/null | tr -d '\n' || echo "0.0.0-dev")
PLATFORM_LDFLAGS := -s -w -X github.com/runfabric/runfabric/platform/core/model.Version=$(VERSION_FILE)

build:
	@mkdir -p bin
	go build -trimpath -ldflags "$(PLATFORM_LDFLAGS)" -o bin/$(APP) ./cmd/runfabric
	go build -trimpath -ldflags "$(PLATFORM_LDFLAGS)" -o bin/$(DAEMON_APP) ./cmd/runfabricd
	go build -trimpath -ldflags "$(PLATFORM_LDFLAGS)" -o bin/$(WORKER_APP) ./cmd/runfabricw

build-daemon:
	@mkdir -p bin
	go build -trimpath -ldflags "$(PLATFORM_LDFLAGS)" -o bin/$(DAEMON_APP) ./cmd/runfabricd

build-worker:
	@mkdir -p bin
	go build -trimpath -ldflags "$(PLATFORM_LDFLAGS)" -o bin/$(WORKER_APP) ./cmd/runfabricw

# Platform-specific binary (name matches SDK: runfabric-darwin-arm64, runfabric-windows-amd64.exe, etc.)
build-platform:
	@mkdir -p bin
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -trimpath -ldflags "$(PLATFORM_LDFLAGS)" -o $(PLATFORM_BIN) ./cmd/runfabric
	@echo "Built $(PLATFORM_BIN)"

build-daemon-platform:
	@mkdir -p bin
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -trimpath -ldflags "$(PLATFORM_LDFLAGS)" -o $(PLATFORM_DAEMON_BIN) ./cmd/runfabricd
	@echo "Built $(PLATFORM_DAEMON_BIN)"

build-worker-platform:
	@mkdir -p bin
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -trimpath -ldflags "$(PLATFORM_LDFLAGS)" -o $(PLATFORM_WORKER_BIN) ./cmd/runfabricw
	@echo "Built $(PLATFORM_WORKER_BIN)"

# Build all platforms (same matrix as .goreleaser.yaml) into bin/
build-all-platforms:
	@mkdir -p bin
	@for pair in darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64 windows/arm64; do \
		GOOS=$${pair%/*} GOARCH=$${pair#*/}; \
		SUF=; [ "$$GOOS" = "windows" ] && SUF=".exe"; \
		echo "Building $$GOOS-$$GOARCH..."; \
		CGO_ENABLED=0 GOOS=$$GOOS GOARCH=$$GOARCH go build -trimpath -ldflags "$(PLATFORM_LDFLAGS)" -o bin/$(APP)-$$GOOS-$$GOARCH$$SUF ./cmd/runfabric; \
		CGO_ENABLED=0 GOOS=$$GOOS GOARCH=$$GOARCH go build -trimpath -ldflags "$(PLATFORM_LDFLAGS)" -o bin/$(DAEMON_APP)-$$GOOS-$$GOARCH$$SUF ./cmd/runfabricd; \
		CGO_ENABLED=0 GOOS=$$GOOS GOARCH=$$GOARCH go build -trimpath -ldflags "$(PLATFORM_LDFLAGS)" -o bin/$(WORKER_APP)-$$GOOS-$$GOARCH$$SUF ./cmd/runfabricw; \
	done
	@echo "Built all platform binaries (runfabric + runfabricd + runfabricw) in bin/"

# Build then compress with UPX (requires: go build, upx). Use for smaller distribution size.
build-upx: build
	@command -v upx >/dev/null 2>&1 || { echo "upx not found; install e.g. brew install upx"; exit 1; }
	$(UPX) bin/$(APP)
	@echo "Compressed bin/$(APP) with UPX"

build-platform-upx: build-platform
	@command -v upx >/dev/null 2>&1 || { echo "upx not found; install e.g. brew install upx"; exit 1; }
	$(UPX) $(PLATFORM_BIN)
	@echo "Compressed $(PLATFORM_BIN) with UPX"

# Build all platform binaries then compress each with UPX (darwin gets --force-macos, others use base flags).
build-all-platforms-upx: build-all-platforms
	@command -v upx >/dev/null 2>&1 || { echo "upx not found; install e.g. brew install upx"; exit 1; }
	@for f in bin/$(APP)-darwin-amd64 bin/$(APP)-darwin-arm64 bin/$(APP)-linux-amd64 bin/$(APP)-linux-arm64 bin/$(APP)-windows-amd64.exe bin/$(APP)-windows-arm64.exe; do \
		[ -f "$$f" ] || continue; \
		case "$$f" in *darwin*) opts="$(UPX_BASE) $(UPX_OPTS_DARWIN)" ;; *) opts="$(UPX_BASE)" ;; esac; \
		$$opts "$$f" && echo "Compressed $$f"; \
	done
	@echo "Compressed all platform binaries in bin/"

# Build all built-in providers as external plugin binaries.
# Pairs: <output-name> <directory-name>
build-provider-plugins:
	@mkdir -p bin/plugins
	@set -e; \
	for pair in \
		"aws aws-lambda" \
		"gcp gcp-functions" \
		"azure azure-functions" \
		"alibaba alibaba-fc" \
		"cloudflare cloudflare-workers" \
		"digitalocean digitalocean-functions" \
		"fly fly-machines" \
		"ibm ibm-openwhisk" \
		"kubernetes kubernetes" \
		"netlify netlify" \
		"vercel vercel"; do \
		set -- $$pair; p=$$1; dir=$$2; \
		echo "Building provider plugin: $$p"; \
		go build -trimpath -ldflags "$(PLATFORM_LDFLAGS)" -o bin/plugins/$$p-plugin ./extensions/providers/$$dir/cmd; \
	done
	@echo "Building provider plugin: linode"
	@cd extensions/providers/linode && GOWORK=off go build -trimpath -ldflags "$(PLATFORM_LDFLAGS)" -o $(CURDIR)/bin/plugins/linode-plugin .

# Install provider plugins and manifests to local external plugin directory.
# This allows switching any provider to external-only mode by editing:
#   platform/extensions/providerpolicy/external_only.go
install-provider-plugins: build-provider-plugins
	@set -e; \
	for triple in \
		"aws aws-lambda aws-lambda" \
		"gcp gcp-functions gcp-functions" \
		"azure azure-functions azure-functions" \
		"alibaba alibaba-fc alibaba-fc" \
		"cloudflare cloudflare-workers cloudflare-workers" \
		"digitalocean digitalocean-functions digitalocean-functions" \
		"fly fly-machines fly-machines" \
		"ibm ibm-openwhisk ibm-openwhisk" \
		"kubernetes kubernetes kubernetes" \
		"netlify netlify netlify" \
		"vercel vercel vercel" \
		"linode linode linode"; do \
		set -- $$triple; p=$$1; id=$$2; srcdir=$$3; \
		dir="$$HOME/.runfabric/plugins/provider/$$id"; \
		mkdir -p "$$dir/bin"; \
		cp "bin/plugins/$$p-plugin" "$$dir/bin/$$p-plugin"; \
		cp "extensions/providers/$$srcdir/plugin.yaml" "$$dir/plugin.yaml"; \
		echo "Installed $$id -> $$dir"; \
	done

# Build secret-manager plugin binaries into bin/plugins/.
build-secretmanager-plugins:
	@mkdir -p bin/plugins
	@set -e; \
	for p in aws gcp azure vault; do \
		echo "Building secret-manager plugin: $$p"; \
		go build -trimpath -ldflags "$(PLATFORM_LDFLAGS)" -o bin/plugins/$$p-secret-manager ./extensions/secretmanagers/$$p; \
	done

# Install secret-manager plugins into ~/.runfabric/plugins/secret-managers/<id>/<version>/.
install-secretmanager-plugins: build-secretmanager-plugins
	@set -e; \
	for triple in \
		"aws aws-secret-manager 0.1.0" \
		"gcp gcp-secret-manager 0.1.0" \
		"azure azure-key-vault-secret-manager 0.1.0" \
		"vault vault-secret-manager 0.1.0"; do \
		set -- $$triple; p=$$1; id=$$2; ver=$$3; \
		dir="$$HOME/.runfabric/plugins/secret-managers/$$id/$$ver"; \
		mkdir -p "$$dir"; \
		cp "bin/plugins/$$p-secret-manager" "$$dir/$$p-secret-manager"; \
		cp "extensions/secretmanagers/$$p/plugin.yaml" "$$dir/plugin.yaml"; \
		echo "Installed $$id@$$ver -> $$dir"; \
	done

# Enforce extension boundary rules:
#   Rule 1: extensions/ must not import github.com/runfabric/runfabric/internal/ or /platform/
#   Rule 2: Shared types must be canonically in internal/<domain> (not duplicated/aliased)
#   Rule 3: No bridge, alias, or re-export file anywhere (all-alias type blocks)
#   Rule 4: At most one file in platform/extensions/ may import root extensions/
#   Legacy: packages/ and platform/extension/external/testdata/ must not import platform/
check-boundary:
	@echo "Checking extension boundary rules..."
	@FAILED=0; \
	\
	echo "  [Rule 1] extensions/ must not import internal/ or platform/..."; \
	if grep -rn '"github.com/runfabric/runfabric/internal/' extensions/ --include='*.go' 2>/dev/null | grep -v '_test.go'; then \
		echo "  ERROR [Rule 1]: extensions/ imports internal/ (see above)"; FAILED=1; \
	fi; \
	if grep -rn '"github.com/runfabric/runfabric/platform/' extensions/ --include='*.go' 2>/dev/null | grep -v '_test.go'; then \
		echo "  ERROR [Rule 1]: extensions/ imports platform/ (see above)"; FAILED=1; \
	fi; \
	\
	echo "  [Rule 3] No alias-only re-export files in extensions/..."; \
	for f in $$(find extensions/ -name '*.go' ! -name '*_test.go' 2>/dev/null); do \
		if [ -f "$$f" ] && grep -q '^\s*[A-Za-z].*=.*\.' "$$f" && ! grep -qE '^\s*(func|type [A-Za-z]+\s+(struct|interface)|var|const)\b' "$$f"; then \
			echo "  ERROR [Rule 3]: $$f appears to be an alias-only re-export file"; FAILED=1; \
		fi; \
	done; \
	\
	echo "  [Rule 4] At most one platform/extensions file may import root extensions/..."; \
	COUNT=$$(grep -rln '"github.com/runfabric/runfabric/extensions"' platform/extensions/ --include='*.go' 2>/dev/null | grep -v '_test.go' | wc -l | tr -d ' '); \
	if [ "$$COUNT" -gt "1" ]; then \
		echo "  ERROR [Rule 4]: $${COUNT} files in platform/extensions/ import root extensions/ (max 1):"; \
		grep -rln '"github.com/runfabric/runfabric/extensions"' platform/extensions/ --include='*.go' | grep -v '_test.go'; \
		FAILED=1; \
	fi; \
	\
	echo "  [Legacy] packages/ and testdata stubs must not import platform/..."; \
	if grep -rn '"github.com/runfabric/runfabric/platform/' packages/ platform/extensions/external/testdata/ 2>/dev/null | grep -v '_test.go'; then \
		echo "  ERROR [Legacy]: platform/ import found in packages/ or testdata (see above)"; FAILED=1; \
	fi; \
	\
	if [ "$$FAILED" -ne "0" ]; then exit 1; fi; \
	echo "check-boundary OK"

# Enforce normalized architecture rules beyond import boundary checks:
#   - internal/ must not import root extensions/
#   - internal/extensions/contracts must not be alias-only re-export layers
#   - internal/extensions/{routers,runtimes,simulators} must not bridge to root extensions/
#   - internal/extensions/builtins loader must not wire root extensions directly
check-architecture: check-boundary
	@echo "Checking normalized architecture rules..."
	@FAILED=0; \
	\
	echo "  [Flow] internal/ must not import root extensions/..."; \
	if grep -rn '"github.com/runfabric/runfabric/extensions/' internal/ --include='*.go' 2>/dev/null | grep -v '_test.go'; then \
		echo "  ERROR [Flow]: internal/ imports root extensions/ (see above)"; FAILED=1; \
	fi; \
	\
	echo "  [Rule 3] internal/extensions/contracts must not be alias-only re-export"; \
	if [ -f internal/extensions/contracts/types.go ] && grep -Eq '^\s*(type\s+[A-Za-z0-9_]+\s*=|[A-Za-z0-9_]+\s*=)' internal/extensions/contracts/types.go; then \
		echo "  ERROR [Rule 3]: internal/extensions/contracts/types.go contains alias/re-export type definitions"; FAILED=1; \
	fi; \
	\
	echo "  [Rule 3] internal bridge packages must not delegate to root extensions/..."; \
	if grep -rn '"github.com/runfabric/runfabric/extensions/' internal/extensions/routers/ internal/extensions/runtimes/ internal/extensions/simulators/ --include='*.go' 2>/dev/null | grep -v '_test.go'; then \
		echo "  ERROR [Rule 3]: bridge/delegator imports found in internal/extensions/{routers,runtimes,simulators}"; FAILED=1; \
	fi; \
	\
	echo "  [Flow] internal/extensions/builtins must not wire root extensions directly"; \
	if [ -f internal/extensions/builtins/loaders.go ] && grep -Eq '"github.com/runfabric/runfabric/extensions/' internal/extensions/builtins/loaders.go; then \
		echo "  ERROR [Flow]: internal/extensions/builtins/loaders.go imports root extensions/*"; FAILED=1; \
	fi; \
	\
	echo "  [Rule 4] go-level architecture policy tests"; \
	go test -count=1 ./platform/core/policy/architecture >/dev/null || FAILED=1; \
	\
	if [ "$$FAILED" -ne "0" ]; then exit 1; fi; \
	echo "check-architecture OK"

# Smoke-check binary command ownership surfaces to prevent drift:
#   - runfabric: control-plane commands, no daemon command
#   - runfabricd: daemon commands only (start/stop/restart/status), no deploy/workflow/daemon subcommand
#   - runfabricw: workflow command only, no deploy/start command
check-binary-surfaces:
	@echo "Checking binary command surfaces..."
	@RF_HELP="$$(go run ./cmd/runfabric --help 2>&1)"; \
	echo "$$RF_HELP" | grep -q '^  deploy[[:space:]]' || { echo "runfabric help missing deploy"; exit 1; }; \
	echo "$$RF_HELP" | grep -q '^  workflow[[:space:]]' || { echo "runfabric help missing workflow"; exit 1; }; \
	if echo "$$RF_HELP" | grep -q '^  daemon[[:space:]]'; then echo "runfabric must not expose daemon command"; exit 1; fi
	@RFD_HELP="$$(go run ./cmd/runfabricd --help 2>&1)"; \
	echo "$$RFD_HELP" | grep -q '^  start[[:space:]]' || { echo "runfabricd help missing start"; exit 1; }; \
	echo "$$RFD_HELP" | grep -q '^  stop[[:space:]]' || { echo "runfabricd help missing stop"; exit 1; }; \
	echo "$$RFD_HELP" | grep -q '^  restart[[:space:]]' || { echo "runfabricd help missing restart"; exit 1; }; \
	echo "$$RFD_HELP" | grep -q '^  status[[:space:]]' || { echo "runfabricd help missing status"; exit 1; }; \
	if echo "$$RFD_HELP" | grep -q '^  deploy[[:space:]]'; then echo "runfabricd must not expose deploy"; exit 1; fi; \
	if echo "$$RFD_HELP" | grep -q '^  workflow[[:space:]]'; then echo "runfabricd must not expose workflow"; exit 1; fi; \
	if echo "$$RFD_HELP" | grep -q '^  daemon[[:space:]]'; then echo "runfabricd must not expose daemon subcommand"; exit 1; fi
	@RFW_HELP="$$(go run ./cmd/runfabricw --help 2>&1)"; \
	echo "$$RFW_HELP" | grep -q '^  workflow[[:space:]]' || { echo "runfabricw help missing workflow"; exit 1; }; \
	if echo "$$RFW_HELP" | grep -q '^  deploy[[:space:]]'; then echo "runfabricw must not expose deploy"; exit 1; fi; \
	if echo "$$RFW_HELP" | grep -q '^  start[[:space:]]'; then echo "runfabricw must not expose start"; exit 1; fi
	@RF_DAEMON_OUT="$$(go run ./cmd/runfabric daemon status 2>&1)"; RF_DAEMON_CODE=$$?; \
	if [ $$RF_DAEMON_CODE -eq 0 ]; then echo "runfabric daemon status unexpectedly succeeded"; exit 1; fi; \
	echo "$$RF_DAEMON_OUT" | grep -q 'unknown command "daemon" for "runfabric"' || { echo "runfabric daemon status error did not mention unknown command"; echo "$$RF_DAEMON_OUT"; exit 1; }
	@RFD_DAEMON_OUT="$$(go run ./cmd/runfabricd daemon status 2>&1)"; RFD_DAEMON_CODE=$$?; \
	if [ $$RFD_DAEMON_CODE -eq 0 ]; then echo "runfabricd daemon status unexpectedly succeeded"; exit 1; fi; \
	echo "$$RFD_DAEMON_OUT" | grep -q 'unknown command "daemon" for "runfabricd"' || { echo "runfabricd daemon status error did not mention unknown command"; echo "$$RFD_DAEMON_OUT"; exit 1; }
	@echo "check-binary-surfaces OK"

# Fast CI check: vet, build, test without -race. Use for quick PR feedback before full release-check.
check-syntax: check-architecture check-binary-surfaces
	@echo "Running go vet..."
	@go vet ./...
	@echo "Building all packages..."
	@go build -v ./...
	@echo "Running tests (no -race)..."
	@go test -count=1 ./...
	@echo "check-syntax OK"

# Pre-release validation: format, vet, build all, test with race, build CLI, then compress with UPX if available (matches CI; AGENTS.md default gate).
release-check: check-architecture check-binary-surfaces
	@echo "Checking format (gofmt)..."
	@test -z "$$(gofmt -l .)" || { echo "Go code is not formatted. Run: gofmt -w ."; gofmt -d .; exit 1; }
	@echo "Running go vet..."
	@go vet ./...
	@echo "Building all packages..."
	@go build -v ./...
	@echo "Running tests (with -race)..."
	@go test -v -race ./...
	@echo "Building CLI binary..."
	@$(MAKE) build
	@if command -v upx >/dev/null 2>&1; then echo "Compressing with UPX..."; $(UPX) bin/$(APP); else echo "UPX not found; skipping compression"; fi
	@echo "release-check OK"

# Verify docs: (1) relative .md links in docs/ resolve to existing files; (2) no outdated refs to packages/planner, packages/core.
# Validates docs/*.md, docs/developer/**/*.md, and apps/registry/docs/**/*.md.
check-docs-sync:
	@echo "Checking doc links..."
	@tmp=$$(mktemp); \
	for f in $$(find docs -name '*.md' 2>/dev/null); do \
	  [ -f "$$f" ] || continue; \
	  base=$$(dirname "$$f"); \
	  grep -hoE '\]\([^)]+\)' "$$f" 2>/dev/null | sed 's/](\(.*\))/\1/' | while read link; do \
	    path=$${link%%#*}; \
	    case "$$link" in http*) continue;; esac; \
	    [ -z "$$path" ] && continue; \
	    res_base="$$base"; res_path="$$path"; \
	    while echo "$$res_path" | grep -q '^\.\./'; do res_base=$$(dirname "$$res_base"); res_path=$$(echo "$$res_path" | sed 's|^\.\./||'); done; \
	    target="$$res_base/$$res_path"; \
	    [ -f "$$target" ] || [ -d "$$target" ] || echo "  Broken: $$f -> $$link ($$target)" >> "$$tmp"; \
	  done; \
	done; \
	outdated=$$(grep -rE 'packages/planner|packages/core' docs/ 2>/dev/null | grep -v ROADMAP_PHASES || true); \
	if [ -n "$$outdated" ]; then echo "  Outdated refs (packages/planner or packages/core) in docs/" >> "$$tmp"; fi; \
	if ! grep -q '"integrations"' schemas/runfabric.schema.json; then echo "  Missing schema key: integrations" >> "$$tmp"; fi; \
	if ! grep -q '"policies"' schemas/runfabric.schema.json; then echo "  Missing schema key: policies" >> "$$tmp"; fi; \
	if ! grep -q '"human-approval"' schemas/runfabric.schema.json; then echo "  Missing workflow step kind in schema: human-approval" >> "$$tmp"; fi; \
	if ! grep -q '"ai-structured"' schemas/runfabric.schema.json; then echo "  Missing workflow step kind in schema: ai-structured" >> "$$tmp"; fi; \
	if ! grep -q '"stepFunctions"' schemas/runfabric.schema.json; then echo "  Missing Step Functions schema section under extensions.aws-lambda" >> "$$tmp"; fi; \
	if ! grep -q '`integrations`' docs/RUNFABRIC_YML_REFERENCE.md; then echo "  Missing docs section/key: integrations" >> "$$tmp"; fi; \
	if ! grep -q '`policies`' docs/RUNFABRIC_YML_REFERENCE.md; then echo "  Missing docs section/key: policies" >> "$$tmp"; fi; \
	if ! grep -q 'human-approval' docs/RUNFABRIC_YML_REFERENCE.md; then echo "  Missing docs section: human-approval workflow lifecycle" >> "$$tmp"; fi; \
	if ! grep -q 'ai-structured' docs/RUNFABRIC_YML_REFERENCE.md; then echo "  Missing docs section: typed workflow step kinds" >> "$$tmp"; fi; \
	if ! grep -q 'MCP' docs/RUNFABRIC_YML_REFERENCE.md; then echo "  Missing docs mention: MCP integration config" >> "$$tmp"; fi; \
	if [ -s "$$tmp" ]; then cat "$$tmp"; rm -f "$$tmp"; exit 1; fi; \
	rm -f "$$tmp"; echo "check-docs-sync OK"

# Pre-push: linting and validation (format, vet, build, test, docs). Used by .githooks/pre-push. Skips UPX.
pre-push:
	@$(MAKE) check-architecture
	@echo "Checking format (gofmt)..."
	@test -z "$$(gofmt -l .)" || { echo "Go code is not formatted. Run: gofmt -w ."; gofmt -d .; exit 1; }
	@echo "Running go vet..."
	@go vet ./...
	@echo "Building all packages..."
	@go build -v ./...
	@echo "Running tests (with -race)..."
	@go test -v -race ./...
	@echo "Building CLI binary..."
	@$(MAKE) build
	@$(MAKE) check-docs-sync
	@echo "pre-push OK"

# Create and push version tag to trigger CI release (goreleaser + npm). Run after release-check.
release-tag:
	@tag="v$$(cat VERSION | tr -d '\n')"; \
	echo "Creating and pushing $$tag..."; \
	git tag "$$tag"; \
	git push origin "$$tag"

# Strip macOS quarantine so binaries in bin/ run without being killed (e.g. after copying from CI).
bin-clear-quarantine:
	@if [ -d bin ]; then xattr -cr bin 2>/dev/null || true; echo "Cleared quarantine on bin/"; fi

version:
	@echo "VERSION file: $$(cat VERSION 2>/dev/null || echo '0.0.0-dev')"
	@([ -f bin/$(APP) ] && ./bin/$(APP) -v) || (go run ./cmd/runfabric -v 2>/dev/null) || true

clean:
	rm -rf bin/
	go clean -cache -testcache 2>/dev/null || true

lint:
	@(command -v golangci-lint >/dev/null 2>&1 && golangci-lint run ./... || go vet ./...)

test:
	go test ./...

# Scan potential feature gaps/stubs and write a report that can be converted into TODOs.
audit-gaps:
	@mkdir -p $(AUDIT_DIR)
	@out="$(AUDIT_DIR)/gaps.txt"; \
	{ \
		echo "RunFabric Gap Scan"; \
		echo "Generated: $$(date -u +'%Y-%m-%dT%H:%M:%SZ')"; \
		echo; \
		echo "[1] TODO/FIXME/XXX markers"; \
		rg -n --glob '*.go' 'TODO|FIXME|XXX' cmd internal platform apps 2>/dev/null || true; \
		echo; \
		echo "[2] Stub/no-op indicators (high-signal)"; \
		rg -n --glob '*.go' 'currently returns empty|not implemented for this provider|not yet supported|panic\("TODO|panic\("not implemented|func main\(\) \{\s*\}' cmd internal platform apps 2>/dev/null || true; \
		echo; \
		echo "[3] Informational: legacy alias wording"; \
		rg -n --glob '*.go' 'legacy alias' cmd internal platform apps 2>/dev/null || true; \
	} > "$$out"; \
	echo "Wrote $$out"; \
	echo "Review and add actionable items to your task TODO list."

# Run unused/dangling checks (compiler+vet baseline, optional staticcheck U1000, module drift).
audit-unused:
	@mkdir -p $(AUDIT_DIR)
	@out="$(AUDIT_DIR)/unused.txt"; \
	{ \
		echo "RunFabric Unused/Dangling Scan"; \
		echo "Generated: $$(date -u +'%Y-%m-%dT%H:%M:%SZ')"; \
		echo; \
		echo "[1] go vet ./..."; \
		go vet ./... 2>&1 || true; \
		echo; \
		echo "[2] staticcheck U1000"; \
		go run honnef.co/go/tools/cmd/staticcheck@v0.6.1 -checks U1000 ./... 2>&1 || true; \
		echo; \
		echo "[3] go mod tidy -diff"; \
		go mod tidy -diff 2>&1 || true; \
	} > "$$out"; \
	echo "Wrote $$out"

audit: audit-gaps audit-unused

# Local extension registry API (development scaffold).
# Override listen address: make registry-api REGISTRY_LISTEN=0.0.0.0:8787
REGISTRY_LISTEN ?= 127.0.0.1:8787
registry-api:
	cd apps/registry && go run ./cmd/registry --listen $(REGISTRY_LISTEN)

# Build registry Docker image. Image name: make docker-registry-build REGISTRY_IMAGE=myorg/runfabric-registry
REGISTRY_IMAGE ?= runfabric-registry:latest
REGISTRY_CONTAINER ?= runfabric-registry
REGISTRY_COMPOSE ?= infra/docker-compose.registry.yml
REGISTRY_WEB_BUILDER_IMAGE ?= public.ecr.aws/docker/library/node:24-alpine
REGISTRY_GO_BUILDER_IMAGE ?= public.ecr.aws/docker/library/golang:1.25-alpine
REGISTRY_RUNTIME_IMAGE ?= public.ecr.aws/docker/library/alpine:3.21

docker-registry-build:
	docker build -t $(REGISTRY_IMAGE) -f apps/registry/Dockerfile \
		--build-arg WEB_BUILDER_IMAGE=$(REGISTRY_WEB_BUILDER_IMAGE) \
		--build-arg GO_BUILDER_IMAGE=$(REGISTRY_GO_BUILDER_IMAGE) \
		--build-arg RUNTIME_IMAGE=$(REGISTRY_RUNTIME_IMAGE) \
		.

docker-registry-run: docker-registry-build
	docker run -d --name $(REGISTRY_CONTAINER) -p 8787:8787 $(REGISTRY_IMAGE)

docker-registry-stop:
	docker stop $(REGISTRY_CONTAINER) 2>/dev/null || true
	docker rm $(REGISTRY_CONTAINER) 2>/dev/null || true

docker-registry-up:
	docker rm -f $(REGISTRY_CONTAINER) 2>/dev/null || true
	docker compose -f $(REGISTRY_COMPOSE) up -d --build

docker-registry-down:
	docker compose -f $(REGISTRY_COMPOSE) down
	docker rm -f $(REGISTRY_CONTAINER) 2>/dev/null || true

# Test coverage for workspace packages. View: go tool cover -html=coverage.out
coverage:
	@go test -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out | tail -1

# Coverage gate: fail if total coverage is below COVERAGE_THRESHOLD (e.g. make coverage-gate COVERAGE_THRESHOLD=10). Omit to only report.
coverage-gate:
	@go test -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out | tail -1
	@if [ -n "$(COVERAGE_THRESHOLD)" ]; then \
		go tool cover -func=coverage.out | tail -1 | awk -v t=$(COVERAGE_THRESHOLD) '{gsub(/%/,""); if ($$NF+0 < t) { print "Coverage " $$NF "% is below threshold " t "%"; exit 1 } }'; \
		echo "Coverage gate passed (threshold $(COVERAGE_THRESHOLD)%)"; \
	fi

test-integration:
	RUNFABRIC_AWS_INTEGRATION=1 go test ./platform/test/integration -v

# CLI command shortcuts (root layout).
# Override with: make <target> EXAMPLE_CONFIG=path/to/runfabric.yml EXAMPLE_STAGE=dev
CLI_RUN := go run ./cmd/runfabric

doctor:
	$(CLI_RUN) doctor --config $(EXAMPLE_CONFIG) --stage $(EXAMPLE_STAGE)

# Init: without DIR, creates a folder named after the service in engine/; with DIR uses that path.
# e.g. make init DIR=./test-exmp  or  make init INIT_ARGS="--no-interactive --service my-api --provider aws-lambda"
init:
	$(CLI_RUN) init $(if $(DIR),--dir $(DIR),) $(INIT_ARGS)

plan:
	$(CLI_RUN) plan --config $(EXAMPLE_CONFIG) --stage $(EXAMPLE_STAGE)

deploy:
	$(CLI_RUN) deploy --config $(EXAMPLE_CONFIG) --stage $(EXAMPLE_STAGE)

remove:
	$(CLI_RUN) remove --config $(EXAMPLE_CONFIG) --stage $(EXAMPLE_STAGE)

invoke:
	$(CLI_RUN) invoke --config $(EXAMPLE_CONFIG) --stage $(EXAMPLE_STAGE) --function hello --payload '{"name":"Yogesh"}'

logs:
	$(CLI_RUN) logs --config $(EXAMPLE_CONFIG) --stage $(EXAMPLE_STAGE) --function hello

inspect:
	$(CLI_RUN) inspect --config $(EXAMPLE_CONFIG) --stage $(EXAMPLE_STAGE)

recover:
	$(CLI_RUN) recover --config $(EXAMPLE_CONFIG) --stage $(EXAMPLE_STAGE)

unlock:
	$(CLI_RUN) unlock --config $(EXAMPLE_CONFIG) --stage $(EXAMPLE_STAGE) --force

inspect-remote:
	RUNFABRIC_BACKEND=s3 \
	RUNFABRIC_S3_BUCKET=$(RUNFABRIC_S3_BUCKET) \
	RUNFABRIC_S3_PREFIX=$(RUNFABRIC_S3_PREFIX) \
	RUNFABRIC_DYNAMODB_TABLE=$(RUNFABRIC_DYNAMODB_TABLE) \
	$(CLI_RUN) inspect --config $(EXAMPLE_CONFIG) --stage $(EXAMPLE_STAGE)

lock-steal:
	$(CLI_RUN) lock-steal --config $(EXAMPLE_CONFIG) --stage $(EXAMPLE_STAGE)

backend-migrate:
	$(CLI_RUN) backend-migrate --config $(EXAMPLE_CONFIG) --stage $(EXAMPLE_STAGE) --target s3

# MCP server (packages/node/mcp). Requires Node.js. Use with Cursor/IDE MCP client (stdio).
mcp-install:
	cd packages/node/mcp && npm install

mcp-build: mcp-install
	cd packages/node/mcp && npm run build

mcp: mcp-build
	cd packages/node/mcp && npm start

# Daemon: run in background (does not hold terminal). Uses bin/runfabricd; logs in .runfabric/daemon.log
daemon-background:
	@mkdir -p .runfabric
	@([ -f bin/runfabricd ] && nohup ./bin/runfabricd >> .runfabric/daemon.log 2>&1 & echo $$! > .runfabric/daemon.pid && echo "Daemon started (PID $$(cat .runfabric/daemon.pid)). Logs: .runfabric/daemon.log") || (echo "Run 'make build' first to create bin/runfabricd" && exit 1)

daemon-stop:
	@[ -f .runfabric/daemon.pid ] && kill $$(cat .runfabric/daemon.pid) 2>/dev/null && rm -f .runfabric/daemon.pid && echo "Daemon stopped." || echo "No daemon PID file (.runfabric/daemon.pid)."

# Daemon Docker image (see docs/DAEMON.md). Image name override: make docker-daemon-build DAEMON_IMAGE=my-registry/runfabric-daemon
DAEMON_IMAGE ?= runfabric-daemon
DAEMON_COMPOSE ?= infra/docker-compose.daemon.yml
DAEMON_GO_BUILDER_IMAGE ?= public.ecr.aws/docker/library/golang:1.25-alpine
DAEMON_RUNTIME_IMAGE ?= public.ecr.aws/docker/library/alpine:3.21

docker-daemon-build:
	docker build -f infra/Dockerfile.daemon \
		--build-arg GO_BUILDER_IMAGE=$(DAEMON_GO_BUILDER_IMAGE) \
		--build-arg RUNTIME_IMAGE=$(DAEMON_RUNTIME_IMAGE) \
		-t $(DAEMON_IMAGE) .

# Tag and push daemon image to GitHub Container Registry (ghcr.io/runfabric/runfabric-daemon:latest)
GHCRIO_DAEMON_IMAGE ?= ghcr.io/runfabric/runfabric-daemon:latest
GHCR_REGISTRY ?= ghcr.io
GHCR_USER ?=
GHCR_PAT ?=
ghcr-login:
	@if [ -z "$(GHCR_USER)" ]; then echo "GHCR_USER is required (example: GHCR_USER=<github-username> make ghcr-login)"; exit 1; fi
	@if [ -z "$(GHCR_PAT)" ]; then echo "GHCR_PAT is required (token with write:packages)"; exit 1; fi
	@echo "$(GHCR_PAT)" | docker login $(GHCR_REGISTRY) -u "$(GHCR_USER)" --password-stdin
	@echo "Logged into $(GHCR_REGISTRY) as $(GHCR_USER)"
docker-daemon-tag: docker-daemon-build
	docker tag $(DAEMON_IMAGE) $(GHCRIO_DAEMON_IMAGE)
docker-daemon-push: docker-daemon-tag
	docker push $(GHCRIO_DAEMON_IMAGE)

docker-daemon-run: docker-daemon-build
	docker run -d --name runfabric-daemon -p 8766:8766 $(DAEMON_IMAGE)

docker-daemon-up:
	docker compose -f $(DAEMON_COMPOSE) up -d

docker-daemon-down:
	docker compose -f $(DAEMON_COMPOSE) down
