# syntax=docker/dockerfile:1.7

# docs-sign ships as a single static binary with the React frontend embedded via
# //go:embed and the PDFium engine embedded as WebAssembly. The runtime image follows the
# LinuxServer.io conventions: it is built on their s6-overlay baseimage, runs the server
# as an unprivileged user whose uid/gid are set at runtime from PUID/PGID, honours TZ and
# UMASK, and keeps all state under a /config volume.
#
# Multi-arch note: the frontend (JavaScript) and the Go compile are both run on the
# native BUILDPLATFORM and the Go binary is *cross-compiled* to TARGETARCH. Because the
# whole program is pure Go (CGO disabled), this avoids slow QEMU emulation entirely — only
# the trivial final-stage assembly is per-architecture.

# ---- Stage 1: build the embedded frontend ------------------------------------------
FROM --platform=$BUILDPLATFORM node:22-alpine AS web
WORKDIR /src/web
# Install deps first (cached unless the lockfile changes).
COPY web/package.json web/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci
# Build the SPA. vite.config.ts writes the output to ../internal/web/dist so it lands at
# /src/internal/web/dist, ready to be embedded by the Go build.
COPY web/ ./
RUN npm run build

# ---- Stage 2: cross-compile the Go binary ------------------------------------------
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
# Module download is arch-independent; cache it across builds.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
# Source, then overlay the freshly built frontend into the embed directory.
COPY . .
COPY --from=web /src/internal/web/dist ./internal/web/dist
# Static binary: CGO off (everything is pure Go), VCS stamping disabled (.git is not in
# the build context), symbols stripped.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -buildvcs=false -ldflags "-s -w" -o /out/docs-sign ./cmd/docs-sign

# ---- Stage 3: LinuxServer.io-style runtime image -----------------------------------
# The baseimage provides s6-overlay (PID 1 + service supervision), the `abc` runtime user,
# and the PUID/PGID/TZ/UMASK handling. Our s6 service (in root/) starts the server.
FROM ghcr.io/linuxserver/baseimage-alpine:3.22

# PORT selects the in-container listen port; PUID/PGID/TZ/UMASK are consumed by the
# baseimage init and can all be overridden at runtime.
ENV PORT=8080

COPY --from=build /out/docs-sign /usr/local/bin/docs-sign
# Redistribution attribution must travel with the binaries.
COPY LICENSE THIRD_PARTY_LICENSES /usr/local/share/doc/docs-sign/
# s6-overlay service definitions: a config-ownership oneshot + the long-running server.
COPY root/ /

EXPOSE 8080
VOLUME /config

# The server has no external dependencies, so a simple liveness probe on /api/health is
# enough. wget comes from the baseimage's busybox.
HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD wget -qO- "http://127.0.0.1:${PORT:-8080}/api/health" >/dev/null 2>&1 || exit 1

# No ENTRYPOINT/CMD: the baseimage's /init (s6-overlay) is the entrypoint and launches the
# service defined under /etc/s6-overlay/s6-rc.d.
