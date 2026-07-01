# syntax=docker/dockerfile:1
# Build context: repository root (docker build -f backend/Dockerfile .)

FROM golang:1.26-bookworm AS builder

WORKDIR /build

# Download modules before source copy for layer caching.
COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend/ ./

ARG TARGETOS=linux
ARG TARGETARCH=amd64

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/scout-api ./cmd/api && \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-s -w" -o /out/scout-seed ./cmd/seed

# ─── runtime ────────────────────────────────────────────────────────────────
FROM debian:bookworm-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends ca-certificates wget && \
    rm -rf /var/lib/apt/lists/*

# Fixed non-root UID/GID.
RUN groupadd -g 10001 scout && \
    useradd -u 10001 -g scout -s /sbin/nologin -M scout

# Pre-create thumbnail cache directory with correct ownership.
RUN mkdir -p /var/cache/scout-thumbnails && \
    chown scout:scout /var/cache/scout-thumbnails

COPY --from=builder /out/scout-api  /usr/local/bin/scout-api
COPY --from=builder /out/scout-seed /usr/local/bin/scout-seed

USER 10001:10001

EXPOSE 8080

ENTRYPOINT ["scout-api"]
