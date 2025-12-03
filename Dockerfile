# Multi-stage build for Kubernetes Event Monitor
#
# Usage:
#   Normal mode (monitor Kubernetes events):
#     docker run -v /data:/data rpk:latest
#
#   Demo mode (serve preset static data):
#     docker run rpk:latest -- --demo
#   Or with custom port:
#     docker run -p 8080:8080 rpk:latest -- --demo --api-port 8080
#

# Stage 1: Build UI
FROM node:25-alpine AS ui-builder

WORKDIR /ui-build

# Copy UI source
COPY ui/package*.json ./

# Install dependencies
RUN npm ci

# Copy UI source code
COPY ui/src ./src
COPY ui/index.html ui/vite.config.ts ui/tsconfig.json ./

# Build UI
RUN npm run build

# Stage 2: Build Go binary
FROM golang:1.25-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git make

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY cmd/ cmd/
COPY internal/ internal/

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o spectre ./cmd/main.go

# Stage 3: Runtime
FROM alpine:3.18

WORKDIR /app

# Install runtime dependencies
RUN apk add --no-cache ca-certificates tzdata

# Copy binary from builder
COPY --from=builder /build/spectre .

# Copy built UI from ui-builder
COPY --from=ui-builder /ui-build/dist ./ui

# Create data directory
RUN mkdir -p /data

# Expose API port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD ["/app/spectre", "--health-check"] || exit 1

# Run the application
ENTRYPOINT ["/app/spectre"]
