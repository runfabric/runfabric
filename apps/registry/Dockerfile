FROM node:24-alpine AS web-builder
WORKDIR /workspace

COPY docs ./docs
COPY apps/registry-api/web ./registry/web
RUN cd registry/web && npm run build

FROM golang:1.25-alpine AS go-builder
WORKDIR /workspace/registry

COPY apps/registry-api/go.mod apps/registry-api/go.sum ./
RUN go mod download

COPY apps/registry-api ./
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /registry ./cmd/registry

FROM alpine:3.20
RUN apk add --no-cache ca-certificates
WORKDIR /app

COPY --from=go-builder /registry /app/registry
COPY --from=go-builder /workspace/registry/configs/config.yaml /app/configs/config.yaml
COPY --from=go-builder /workspace/registry/internal/adapter/policy/casbin /app/internal/adapter/policy/casbin
COPY --from=web-builder /workspace/registry/web/dist /app/web/dist

ENV REGISTRY_LISTEN=0.0.0.0:8787
ENV REGISTRY_WEB_DIR=/app/web/dist
EXPOSE 8787

ENTRYPOINT ["/app/registry"]
CMD ["-config", "/app/configs/config.yaml"]
