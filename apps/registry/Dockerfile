ARG WEB_BUILDER_IMAGE=public.ecr.aws/docker/library/node:24-alpine
ARG GO_BUILDER_IMAGE=public.ecr.aws/docker/library/golang:1.25-alpine
ARG RUNTIME_IMAGE=public.ecr.aws/docker/library/alpine:3.21

FROM ${WEB_BUILDER_IMAGE} AS web-builder
WORKDIR /workspace

COPY docs ./docs
COPY apps/registry ./apps/registry
RUN cd apps/registry/web && npm run build

FROM ${GO_BUILDER_IMAGE} AS go-builder
WORKDIR /workspace/apps/registry

COPY apps/registry/go.mod apps/registry/go.sum ./
RUN go mod download

COPY apps/registry ./
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /registry ./cmd/registry

FROM ${RUNTIME_IMAGE}
RUN apk add --no-cache ca-certificates
WORKDIR /app

COPY --from=go-builder /registry /app/registry
COPY --from=go-builder /workspace/apps/registry/configs/config.yaml /app/configs/config.yaml
COPY --from=go-builder /workspace/apps/registry/internal/adapter/policy/casbin /app/internal/adapter/policy/casbin
COPY --from=web-builder /workspace/apps/registry/web/dist /app/web/dist

ENV REGISTRY_LISTEN=0.0.0.0:8787
ENV REGISTRY_WEB_DIR=/app/web/dist
EXPOSE 8787

ENTRYPOINT ["/app/registry"]
CMD ["-config", "/app/configs/config.yaml"]
