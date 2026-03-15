APP=runfabric

.PHONY: build test test-integration release-check version clean lint doctor plan deploy remove invoke logs inspect recover unlock

# Default target
all: build

VERSION_FILE := $(shell cat VERSION 2>/dev/null | tr -d '\n' || echo "0.0.0-dev")

build:
	@mkdir -p bin
	go build -trimpath -ldflags "-s -w -X github.com/runfabric/runfabric/internal/runtime.Version=$(VERSION_FILE) -X github.com/runfabric/runfabric/internal/runtime.ProtocolVersion=1" -o bin/$(APP) ./cmd/runfabric

# Pre-release validation: build everything and run tests (AGENTS.md default gate).
release-check: build
	go build ./...
	go test ./...

version:
	@echo "VERSION file: $$(cat VERSION 2>/dev/null || echo '0.0.0-dev')"
	@([ -f bin/$(APP) ] && ./bin/$(APP) -v) || (go run ./cmd/runfabric -v 2>/dev/null) || true

clean:
	rm -rf bin/
	go clean -cache -testcache 2>/dev/null || true

lint:
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run ./... || go vet ./...

build-upx:
	upx --best --lzma --force-macos bin/$(APP) ./cmd/runfabric

test:
	go test ./...

test-integration:
	RUNFABRIC_AWS_INTEGRATION=1 go test ./test/integration -v

doctor:
	go run ./cmd/runfabric doctor --config examples/hello-aws/runfabric.yml --stage dev

plan:
	go run ./cmd/runfabric plan --config examples/hello-aws/runfabric.yml --stage dev

deploy:
	go run ./cmd/runfabric deploy --config examples/hello-aws/runfabric.yml --stage dev

remove:
	go run ./cmd/runfabric remove --config examples/hello-aws/runfabric.yml --stage dev

invoke:
	go run ./cmd/runfabric invoke --config examples/hello-aws/runfabric.yml --stage dev --function hello --payload '{"name":"Yogesh"}'

logs:
	go run ./cmd/runfabric logs --config examples/hello-aws/runfabric.yml --stage dev --function hello

inspect:
	go run ./cmd/runfabric inspect --config examples/hello-aws/runfabric.yml --stage dev

recover:
	go run ./cmd/runfabric recover --config examples/hello-aws/runfabric.yml --stage dev

unlock:
	go run ./cmd/runfabric unlock --config examples/hello-aws/runfabric.yml --stage dev --force

inspect-remote:
	RUNFABRIC_BACKEND=aws-remote \
	RUNFABRIC_S3_BUCKET=$(RUNFABRIC_S3_BUCKET) \
	RUNFABRIC_S3_PREFIX=$(RUNFABRIC_S3_PREFIX) \
	RUNFABRIC_DYNAMODB_TABLE=$(RUNFABRIC_DYNAMODB_TABLE) \
	go run ./cmd/runfabric inspect --config examples/hello-aws/runfabric.yml --stage dev

lock-steal:
	go run ./cmd/runfabric lock-steal --config examples/hello-aws/runfabric.yml --stage dev

backend-migrate:
	go run ./cmd/runfabric backend-migrate --config examples/hello-aws/runfabric.yml --stage dev --target aws-remote