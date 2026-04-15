# gRPC Server Architecture

## Overview

Spectre runs **two separate servers** for different client types:

1. **HTTP + gRPC-Web Server** (port 8080)
2. **Native gRPC Server** (port 50051)

## 1. HTTP + gRPC-Web Server (Port 8080)

**File:** `internal/api/server.go`

**Purpose:** Serves browser clients with REST API, gRPC-Web, and static UI

**Capabilities:**
- REST API endpoints (`/v1/timeline`, `/v1/metadata`, etc.)
- gRPC-Web (wrapped gRPC accessible from browsers)
- Static UI file serving
- CORS support for development

**Architecture:**
```
Browser Request
    ↓
HTTP Server (port 8080)
    ↓
Router checks if gRPC-Web request
    ↓
YES → grpcWebWrapper.ServeHTTP()
NO  → Regular HTTP handlers
```

**Key Code:**
- Line 78: Creates gRPC server
- Lines 93-99: Wraps gRPC with gRPC-Web middleware
- Lines 105-114: Routes gRPC-Web requests to wrapper, others to HTTP router

## 2. Native gRPC Server (Port 50051)

**File:** `internal/api/grpc_server.go`

**Purpose:** Serves native gRPC clients (Go apps, grpcurl, etc.)

**Capabilities:**
- Pure gRPC protocol (not gRPC-Web)
- For non-browser clients only
- Better performance for server-to-server communication

## Frontend Configuration

**The frontend MUST use the HTTP port (8080) for gRPC-Web requests.**

**File:** `ui/src/services/api.ts`

**Correct Configuration:**
```typescript
// Line 89: Use baseUrl (HTTP origin) for gRPC-Web
this.grpcService = new TimelineGrpcService(this.baseUrl);
```

**Why?**
- Browsers cannot use native gRPC (requires HTTP/2 bidirectional streaming)
- gRPC-Web is a compatible protocol that works over HTTP/1.1 and HTTP/2
- The HTTP server wraps the gRPC server with gRPC-Web middleware
- All gRPC-Web requests go to port 8080, not 50051

## Port Summary

| Port | Server | Protocol | Clients |
|------|--------|----------|---------|
| 8080 | HTTP + gRPC-Web | HTTP/1.1, HTTP/2, gRPC-Web | Browsers, REST clients |
| 8081 | Native gRPC (via HTTP server) | gRPC | Currently unused (wrapped by gRPC-Web) |
| 50051 | Native gRPC (standalone) | gRPC | Native gRPC clients (Go, grpcurl) |

## Recent Changes

**Updated:** `ui/src/hooks/useTimeline.ts`

- Changed from REST API (`getTimeline`) to gRPC streaming (`getTimelineGrpc`)
- Added progressive rendering support via streaming chunks
- Resources load incrementally as they arrive from the server
- Improves perceived performance for large datasets

**Benefits:**
- Faster initial render (show first batch while loading rest)
- Better performance with protobuf binary encoding
- Server can prioritize visible resources
- Reduced memory overhead with chunked processing
