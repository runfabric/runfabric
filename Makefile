APP=runfabric

.PHONY: all help build build-platform build-all-platforms build-upx build-platform-upx build-all-platforms-upx test test-integration release-check check-syntax release-tag version clean lint bin-clear-quarantine check-docs-sync pre-push doctor plan deploy remove invoke logs inspect recover unlock mcp-install mcp-build mcp daemon-background daemon-stop docker-daemon-build docker-daemon-run docker-daemon-up docker-daemon-down

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

# Default target
all: build

help:
	@echo "RunFabric Makefile targets:"
	@echo "  make              same as make build"
	@echo "  make build        build CLI to bin/runfabric"
	@echo "  make build-platform  build CLI to bin/runfabric-$(GOOS)-$(GOARCH)$(BIN_SUFFIX) (for SDK)"
	@echo "  make build-all-platforms  build all GoReleaser targets into bin/"
	@echo "  make build-upx    build then compress bin/runfabric with UPX"
	@echo "  make build-platform-upx   build platform binary then compress with UPX"
	@echo "  make build-all-platforms-upx  build all platforms then compress each with UPX"
	@echo "  make test         run tests"
	@echo "  make coverage     run tests with coverage report (engine)"
	@echo "  make coverage-gate [COVERAGE_THRESHOLD=N]  run coverage; fail if total below N%% (omit to report only)"
	@echo "  make lint         go vet / golangci-lint"
	@echo "  make release-check  format + vet + build + test -race + build CLI + UPX (CI gate)"
	@echo "  make check-syntax   go vet + go build + go test -count=1 (no -race); fast PR feedback"
	@echo "  make check-docs-sync  verify doc links and no outdated refs (packages/planner, packages/core)"
	@echo "  make pre-push       lint + validation (used by .githooks/pre-push)"
	@echo "  make release-tag   create and push tag v\$$(cat VERSION) to trigger CI release"
	@echo "  make bin-clear-quarantine  strip macOS quarantine from bin/ (fix 'killed' when running copied binaries)"
	@echo "  make clean        remove bin/ and go caches"
	@echo "  make version      show VERSION and binary -v"
	@echo "  make doctor plan deploy ...  run runfabric commands via go run (see Makefile)"
	@echo "  make mcp-install   npm install in protocol/mcp (MCP server)"
	@echo "  make mcp-build     build MCP server (protocol/mcp)"
	@echo "  make mcp           run MCP server (stdio; requires runfabric on PATH)"
	@echo "  make daemon-background  start daemon in background (logs: .runfabric/daemon.log)"
	@echo "  make daemon-stop   stop daemon started with daemon-background"
	@echo "  make docker-daemon-build  build daemon Docker image (Dockerfile.daemon)"
	@echo "  make docker-daemon-run    run daemon container (API on port 8766)"
	@echo "  make docker-daemon-up    docker compose up daemon + Redis (docker-compose.daemon.yml)"
	@echo "  make docker-daemon-down  docker compose down daemon stack"

VERSION_FILE := $(shell cat VERSION 2>/dev/null | tr -d '\n' || echo "0.0.0-dev")
ENGINE_LDFLAGS := -s -w -X github.com/runfabric/runfabric/engine/internal/runtime.Version=$(VERSION_FILE) -X github.com/runfabric/runfabric/engine/internal/runtime.ProtocolVersion=1

build:
	@mkdir -p bin
	cd engine && go build -trimpath -ldflags "$(ENGINE_LDFLAGS)" -o ../bin/$(APP) ./cmd/runfabric

# Platform-specific binary (name matches SDK: runfabric-darwin-arm64, runfabric-windows-amd64.exe, etc.)
build-platform:
	@mkdir -p bin
	cd engine && CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) go build -trimpath -ldflags "$(ENGINE_LDFLAGS)" -o ../$(PLATFORM_BIN) ./cmd/runfabric
	@echo "Built $(PLATFORM_BIN)"

# Build all platforms (same matrix as .goreleaser.yaml) into bin/
build-all-platforms:
	@mkdir -p bin
	@for pair in darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64 windows/arm64; do \
		GOOS=$${pair%/*} GOARCH=$${pair#*/}; \
		SUF=; [ "$$GOOS" = "windows" ] && SUF=".exe"; \
		echo "Building $$GOOS-$$GOARCH..."; \
		cd engine && CGO_ENABLED=0 GOOS=$$GOOS GOARCH=$$GOARCH go build -trimpath -ldflags "$(ENGINE_LDFLAGS)" -o ../bin/$(APP)-$$GOOS-$$GOARCH$$SUF ./cmd/runfabric; \
		cd ..; \
	done
	@echo "Built all platform binaries in bin/"

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

# Fast CI check: vet, build, test without -race. Use for quick PR feedback before full release-check.
check-syntax:
	@echo "Running go vet..."
	@cd engine && go vet ./...
	@echo "Building all packages..."
	@cd engine && go build -v ./...
	@echo "Running tests (no -race)..."
	@cd engine && go test -count=1 ./...
	@echo "check-syntax OK"

# Pre-release validation: format, vet, build all, test with race, build CLI, then compress with UPX if available (matches CI; AGENTS.md default gate).
release-check:
	@echo "Checking format (gofmt)..."
	@test -z "$$(cd engine && gofmt -l .)" || { echo "Go code is not formatted. Run: cd engine && gofmt -w ."; cd engine && gofmt -d .; exit 1; }
	@echo "Running go vet..."
	@cd engine && go vet ./...
	@echo "Building all packages..."
	@cd engine && go build -v ./...
	@echo "Running tests (with -race)..."
	@cd engine && go test -v -race ./...
	@echo "Building CLI binary..."
	@$(MAKE) build
	@if command -v upx >/dev/null 2>&1; then echo "Compressing with UPX..."; $(UPX) bin/$(APP); else echo "UPX not found; skipping compression"; fi
	@echo "release-check OK"

# Verify docs: (1) relative .md links in docs/ resolve to existing files; (2) no outdated refs to packages/planner, packages/core.
check-docs-sync:
	@echo "Checking doc links..."
	@tmp=$$(mktemp); \
	for f in docs/*.md; do \
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
	outdated=$$(grep -rE 'packages/planner|packages/core' docs/*.md 2>/dev/null | grep -v ROADMAP_PHASES || true); \
	if [ -n "$$outdated" ]; then echo "  Outdated refs (packages/planner or packages/core) in docs/" >> "$$tmp"; fi; \
	if [ -s "$$tmp" ]; then cat "$$tmp"; rm -f "$$tmp"; exit 1; fi; \
	rm -f "$$tmp"; echo "check-docs-sync OK"

# Pre-push: linting and validation (format, vet, build, test, docs). Used by .githooks/pre-push. Skips UPX.
pre-push:
	@echo "Checking format (gofmt)..."
	@test -z "$$(cd engine && gofmt -l .)" || { echo "Go code is not formatted. Run: cd engine && gofmt -w ."; cd engine && gofmt -d .; exit 1; }
	@echo "Running go vet..."
	@cd engine && go vet ./...
	@echo "Building all packages..."
	@cd engine && go build -v ./...
	@echo "Running tests (with -race)..."
	@cd engine && go test -v -race ./...
	@echo "Building CLI binary..."
	@$(MAKE) build
	@$(MAKE) check-docs-sync
	@echo "pre-push OK"

# Create and push version tag to trigger CI release (goreleaser + npm). Run after release-check.
release-tag:
	@./scripts/release.sh tag

# Strip macOS quarantine so binaries in bin/ run without being killed (e.g. after copying from CI).
bin-clear-quarantine:
	@if [ -d bin ]; then xattr -cr bin 2>/dev/null || true; echo "Cleared quarantine on bin/"; fi

version:
	@echo "VERSION file: $$(cat VERSION 2>/dev/null || echo '0.0.0-dev')"
	@([ -f bin/$(APP) ] && ./bin/$(APP) -v) || (go run ./cmd/runfabric -v 2>/dev/null) || true

clean:
	rm -rf bin/
	cd engine && go clean -cache -testcache 2>/dev/null || true

lint:
	@cd engine && (command -v golangci-lint >/dev/null 2>&1 && golangci-lint run ./... || go vet ./...)

test:
	cd engine && go test ./...

# Test coverage for engine. Target: 95%% for critical packages (internal/config, internal/planner). View: cd engine && go tool cover -html=coverage.out
coverage:
	@cd engine && go test -coverprofile=coverage.out ./...
	@cd engine && go tool cover -func=coverage.out | tail -1

# Coverage gate: fail if total coverage is below COVERAGE_THRESHOLD (e.g. make coverage-gate COVERAGE_THRESHOLD=10). Omit to only report.
coverage-gate:
	@cd engine && go test -coverprofile=coverage.out ./...
	@cd engine && go tool cover -func=coverage.out | tail -1
	@if [ -n "$(COVERAGE_THRESHOLD)" ]; then \
		cd engine && go tool cover -func=coverage.out | tail -1 | awk -v t=$(COVERAGE_THRESHOLD) '{gsub(/%/,""); if ($$NF+0 < t) { print "Coverage " $$NF "% is below threshold " t "%"; exit 1 } }'; \
		echo "Coverage gate passed (threshold $(COVERAGE_THRESHOLD)%)"; \
	fi

test-integration:
	cd engine && RUNFABRIC_AWS_INTEGRATION=1 go test ./test/integration -v

doctor:
	cd engine && go run ./cmd/runfabric doctor --config ../examples/node/hello-aws/runfabric.yml --stage dev

# Init: without DIR, creates a folder named after the service in engine/; with DIR uses that path.
# e.g. make init DIR=./test-exmp  or  make init INIT_ARGS="--no-interactive --service my-api --provider aws-lambda"
init:
	cd engine && go run ./cmd/runfabric init $(if $(DIR),--dir $(DIR),) $(INIT_ARGS)

plan:
	cd engine && go run ./cmd/runfabric plan --config ../examples/node/hello-aws/runfabric.yml --stage dev

deploy:
	cd engine && go run ./cmd/runfabric deploy --config ../examples/node/hello-aws/runfabric.yml --stage dev

remove:
	cd engine && go run ./cmd/runfabric remove --config ../examples/node/hello-aws/runfabric.yml --stage dev

invoke:
	cd engine && go run ./cmd/runfabric invoke --config ../examples/node/hello-aws/runfabric.yml --stage dev --function hello --payload '{"name":"Yogesh"}'

logs:
	cd engine && go run ./cmd/runfabric logs --config ../examples/node/hello-aws/runfabric.yml --stage dev --function hello

inspect:
	cd engine && go run ./cmd/runfabric inspect --config ../examples/node/hello-aws/runfabric.yml --stage dev

recover:
	cd engine && go run ./cmd/runfabric recover --config ../examples/node/hello-aws/runfabric.yml --stage dev

unlock:
	cd engine && go run ./cmd/runfabric unlock --config ../examples/node/hello-aws/runfabric.yml --stage dev --force

inspect-remote:
	cd engine && RUNFABRIC_BACKEND=aws-remote \
	RUNFABRIC_S3_BUCKET=$(RUNFABRIC_S3_BUCKET) \
	RUNFABRIC_S3_PREFIX=$(RUNFABRIC_S3_PREFIX) \
	RUNFABRIC_DYNAMODB_TABLE=$(RUNFABRIC_DYNAMODB_TABLE) \
	go run ./cmd/runfabric inspect --config ../examples/node/hello-aws/runfabric.yml --stage dev

lock-steal:
	cd engine && go run ./cmd/runfabric lock-steal --config ../examples/node/hello-aws/runfabric.yml --stage dev

backend-migrate:
	cd engine && go run ./cmd/runfabric backend-migrate --config ../examples/node/hello-aws/runfabric.yml --stage dev --target aws-remote

# MCP server (protocol/mcp). Requires Node.js. Use with Cursor/IDE MCP client (stdio).
mcp-install:
	cd protocol/mcp && npm install

mcp-build: mcp-install
	cd protocol/mcp && npm run build

mcp: mcp-build
	cd protocol/mcp && npm start

# Daemon: run in background (does not hold terminal). Uses bin/runfabric; logs in .runfabric/daemon.log
daemon-background:
	@mkdir -p .runfabric
	@([ -f bin/runfabric ] && nohup ./bin/runfabric daemon >> .runfabric/daemon.log 2>&1 & echo $$! > .runfabric/daemon.pid && echo "Daemon started (PID $$(cat .runfabric/daemon.pid)). Logs: .runfabric/daemon.log") || (echo "Run 'make build' first to create bin/runfabric" && exit 1)

daemon-stop:
	@[ -f .runfabric/daemon.pid ] && kill $$(cat .runfabric/daemon.pid) 2>/dev/null && rm -f .runfabric/daemon.pid && echo "Daemon stopped." || echo "No daemon PID file (.runfabric/daemon.pid)."

# Daemon Docker image (see docs/DAEMON.md). Image name override: make docker-daemon-build DAEMON_IMAGE=my-registry/runfabric-daemon
DAEMON_IMAGE ?= runfabric-daemon
DAEMON_COMPOSE ?= docker-compose.daemon.yml

docker-daemon-build:
	docker build -f Dockerfile.daemon -t $(DAEMON_IMAGE) .

docker-daemon-run: docker-daemon-build
	docker run -p 8766:8766 $(DAEMON_IMAGE)

docker-daemon-up:
	docker compose -f $(DAEMON_COMPOSE) up -d

docker-daemon-down:
	docker compose -f $(DAEMON_COMPOSE) down