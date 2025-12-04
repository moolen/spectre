FROM node:25-alpine AS ui-builder
WORKDIR /ui-build
COPY ui/package*.json ./
RUN npm ci
COPY ui/src ./src
COPY ui/public ./public
COPY ui/index.html ui/vite.config.ts ui/tsconfig.json ./
RUN npm run build

FROM golang:1.25-alpine AS builder
WORKDIR /build
RUN apk add --no-cache git make
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ cmd/
COPY internal/ internal/
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o spectre ./cmd/main.go

FROM alpine:3.18
WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /build/spectre .
COPY --from=ui-builder /ui-build/dist ./ui
RUN mkdir -p /data
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD ["/app/spectre", "--health-check"] || exit 1

ENTRYPOINT ["/app/spectre"]
