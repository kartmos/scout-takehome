# syntax=docker/dockerfile:1
# Build context: repository root (docker build -f frontend/Dockerfile .)

FROM --platform=$BUILDPLATFORM node:24-bookworm-slim AS builder

WORKDIR /app

# Enable pnpm via corepack (matches packageManager in package.json).
RUN corepack enable && corepack prepare pnpm@10.34.4 --activate

# Install dependencies first for cache efficiency.
COPY frontend/package.json frontend/pnpm-lock.yaml frontend/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile

# openapi.yaml lives at the repo root; generate:api resolves it as ../openapi.yaml from /app.
COPY openapi.yaml /openapi.yaml

# Copy remaining source files.
COPY frontend/ ./

# Generate OpenAPI types, typecheck, and build.
# VITE_SCOUT_API_KEY ends up in the browser bundle; it is a demo limitation (static key).
ARG VITE_SCOUT_API_KEY
RUN pnpm generate:api && \
    pnpm typecheck && \
    VITE_SCOUT_API_BASE_URL=/api \
    VITE_SCOUT_API_KEY=${VITE_SCOUT_API_KEY} \
    pnpm build

# ─── runtime ────────────────────────────────────────────────────────────────
FROM nginxinc/nginx-unprivileged:1.27-alpine

COPY --from=builder /app/dist     /usr/share/nginx/html
COPY frontend/nginx.conf          /etc/nginx/nginx.conf

EXPOSE 8080
