# syntax=docker/dockerfile:1.7

FROM node:22-alpine AS admin-builder

WORKDIR /src/market_registry/admin

COPY admin/package.json ./
RUN npm install --no-audit --no-fund

COPY admin ./ 
COPY internal/adminui ../internal/adminui

RUN npm run build

FROM golang:1.25-alpine AS go-builder

RUN apk add --no-cache ca-certificates git

WORKDIR /src/market_registry

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=admin-builder /src/market_registry/internal/adminui/dist ./internal/adminui/dist

ARG BUILD_VERSION=dev

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/market-registry-api ./cmd/market-registry-api && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/market-registry-cli ./cmd/market-registry-cli

FROM alpine:3.20 AS runtime

ARG BUILD_VERSION=dev

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

ENV MARKET_REGISTRY_ADDR=:18080 \
    MARKET_REGISTRY_DATA_DIR=/app/data/data \
    MARKET_REGISTRY_KEY_DIR=/app/data/keys

LABEL org.opencontainers.image.title="AuraLogic Market Registry" \
      org.opencontainers.image.version="${BUILD_VERSION}"

COPY docker-build/entrypoint.sh /usr/local/bin/market-registry-entrypoint
COPY --from=go-builder /out/market-registry-api /usr/local/bin/market-registry-api
COPY --from=go-builder /out/market-registry-cli /usr/local/bin/market-registry-cli

RUN chmod +x /usr/local/bin/market-registry-entrypoint \
    /usr/local/bin/market-registry-api \
    /usr/local/bin/market-registry-cli && \
    mkdir -p /app/data

VOLUME ["/app/data"]

EXPOSE 18080

ENTRYPOINT ["/usr/local/bin/market-registry-entrypoint"]
