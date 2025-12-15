# Multi-stage Dockerfile for JASM (Just Another Secret Manager)
# Supports multi-architecture builds
#
# Build targets:
#   --target=minimal   - Controller only (default, distroless)
#   --target=bitwarden - Controller + Bitwarden CLI

# Stage 1: Build Go binary
FROM --platform=$BUILDPLATFORM golang:1.24-alpine AS builder

RUN apk add --no-cache git make

WORKDIR /workspace

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ cmd/
COPY internal/ internal/

ARG TARGETOS=linux
ARG TARGETARCH=amd64

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -trimpath -ldflags="-w -s" -o controller ./cmd/controller/main.go

# Stage 2: Build Bitwarden CLI from source (native arch)
FROM node:22 AS bw-builder

RUN git clone --depth 1 https://github.com/bitwarden/clients.git /clients

WORKDIR /clients

RUN npm ci

WORKDIR /clients/apps/cli

# Build and package for the native architecture
RUN ARCH=$(uname -m) && \
    if [ "$ARCH" = "aarch64" ] || [ "$ARCH" = "arm64" ]; then \
        npm run dist:bit:lin-arm64 && \
        cp dist/bit/linux-arm64/bw /bw; \
    else \
        npm run dist:bit:lin && \
        cp dist/bit/linux/bw /bw; \
    fi

# Stage 3a: Runtime without Bitwarden CLI (distroless, minimal)
FROM gcr.io/distroless/base-debian12:nonroot AS minimal

WORKDIR /
COPY --from=builder --chmod=755 /workspace/controller /controller
USER 65532:65532
ENTRYPOINT ["/controller"]

# Stage 3b: Runtime with Bitwarden CLI
FROM debian:12-slim AS bitwarden

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /

COPY --from=builder --chmod=755 /workspace/controller /controller
COPY --from=bw-builder --chmod=755 /bw /usr/local/bin/bw

RUN groupadd -g 65532 nonroot && \
    useradd -u 65532 -g nonroot -s /bin/false -m nonroot

USER 65532:65532
ENV HOME=/home/nonroot

ENTRYPOINT ["/controller"]
