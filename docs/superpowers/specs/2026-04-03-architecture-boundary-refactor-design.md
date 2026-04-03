# Architecture Boundary Refactor Design

**Date:** 2026-04-03

## Goal

Refactor the current server, HTTP, and MCP architecture to establish a clean dependency direction:

`cmd` -> transport adapters (`apiserver`, `api`, `mcp`) -> application services (`internal/app/...`) -> domain/storage/integration packages.

This first slice is intentionally narrow. It fixes the highest-value boundary problems without attempting a full repo-wide redesign of large subsystems such as `internal/integration/grafana` or `internal/graph`.

## Scope

### In scope

- Introduce an application-layer package family under `internal/app/`.
- Move timeline query/ingest ports out of `internal/api`.
- Move timeline and graph orchestration services out of `internal/api`.
- Convert HTTP handlers and MCP server code to depend on application services instead of transport-owned services.
- Move integration admin behavior out of HTTP handlers into an application service.
- Reduce responsibility inside `internal/apiserver` so it assembles adapters around already-created services instead of building those services internally.
- Reduce the size of the most overloaded transport/composition files as part of the boundary changes.

### Out of scope

- Full decomposition of `internal/integration/grafana` into subpackages.
- Refactoring `internal/graph/sync/builder.go`, `internal/graph/schema.go`, or other graph internals beyond any compatibility changes needed for the new seams.
- Changing external API behavior or MCP tool behavior.
- Renaming public HTTP endpoints, CLI flags, or integration config structures.

## Current Problems

### `internal/api` mixes transport and application concerns

`internal/api` currently owns both transport-facing code and core orchestration:

- Query and ingest ports
- Timeline business logic
- Graph orchestration logic
- Protobuf projection helpers

That makes transport packages and non-transport packages depend on `internal/api` as a pseudo-core package, which inverts the intended direction.

### HTTP handlers perform runtime composition and plugin-specific logic

`internal/api/handlers/register.go` creates services and chooses executors instead of only wiring handlers to already-defined dependencies. `integration_config_handler.go` also contains runtime-specific operations such as integration testing, sync triggering, SSE status polling, and Grafana-specific signal validation paths.

### MCP depends on transport-owned services

`internal/mcp/server.go` depends directly on `internal/api` service types. MCP should consume application services, not services defined in another transport-adjacent package.

### Composition responsibilities are spread across too many layers

`cmd/spectre/commands/server.go`, `internal/apiserver`, and `internal/api/handlers` all perform some mixture of wiring, policy decisions, and concrete service construction. This makes lifecycle and dependency relationships harder to reason about.

## Target Architecture

### Application layer

Introduce focused application packages:

- `internal/app/timeline`
  - timeline query ports
  - ingest ports
  - timeline service
  - request parsing helpers that are domain/application oriented only
- `internal/app/graph`
  - graph analysis service
  - dependencies on analysis store and graph-backed analyzers
- `internal/app/integrationadmin`
  - integration config CRUD orchestration
  - connection testing
  - runtime status lookup
  - sync and signal-validation orchestration

These packages own use-case orchestration and ports. They do not import HTTP, MCP, protobuf, or path-based router concerns.

### Transport adapters

- `internal/api`
  - request parsing
  - response writing
  - protobuf and Connect/gRPC projections
  - HTTP handler types that delegate into `internal/app/...`
- `internal/mcp`
  - MCP schema definitions
  - tool registration
  - adapter glue between MCP requests and `internal/app/...`
- `internal/apiserver`
  - HTTP server setup
  - middleware
  - router registration
  - startup/shutdown of caches and the HTTP server

### Composition root

`cmd/spectre/commands/server.go` remains the main composition root for runtime mode selection. It will construct:

- concrete storage/query implementations
- application services
- transport adapters using those services
- lifecycle-managed runtime components

This keeps policy at the edge while preventing lower transport packages from creating core services on their own.

## Detailed Design

### 1. Move ports from `internal/api` to `internal/app/timeline`

Move these abstractions out of `internal/api`:

- `QueryExecutor`
- `EventIngestor`
- `BatchIngestor`
- `TimelineQuerySource`

Consumers such as watcher, handlers, apiserver, and MCP will be updated to use the application package definitions.

The key design rule is that transports can depend on these ports, but the ports cannot depend on transports.

### 2. Move timeline orchestration to `internal/app/timeline`

`TimelineService` becomes an application service. It keeps query orchestration, validation flow, and timeline response building, but it stops owning protobuf conversion.

Planned split:

- `internal/app/timeline/service.go`
  - service struct and main query orchestration
- `internal/app/timeline/query.go`
  - query parsing and pagination helpers
- `internal/app/timeline/response.go`
  - domain-level response construction
- `internal/api/timeline_proto.go` or similar
  - protobuf projection previously embedded in the service

This removes the current cross-layer leak where a framework-agnostic service imports protobuf packages.

### 3. Move graph orchestration to `internal/app/graph`

`GraphService` moves to an application package. It remains a thin orchestrator over analyzers, but it should be owned by the application layer rather than the HTTP package family.

The service may continue to expose observatory graph support via explicit methods, but transports should see it as an application dependency rather than a type from `internal/api`.

### 4. Add an integration admin application service

Create `internal/app/integrationadmin` to own the non-transport behavior currently embedded in `integration_config_handler.go`.

Responsibilities:

- load and validate integration config files
- enrich config data with runtime health and sync status
- create/update/delete integration definitions
- perform test-connection workflow
- trigger sync for integrations that support it
- trigger signal validation for integrations that support it
- provide status snapshot data for SSE polling

The HTTP handler becomes a thin adapter that:

- parses URL and request body
- calls the admin service
- maps result/errors to HTTP responses

This also provides a future seam for non-HTTP management surfaces.

### 5. Remove plugin loading from HTTP handlers

The blank import of Grafana inside `integration_config_handler.go` will be removed. Integration registration belongs at the composition edge, not in transport handlers.

The CLI composition root already imports integration implementations for registration. That should remain the authoritative registration location for this slice.

### 6. Simplify `internal/apiserver`

`internal/apiserver` should stop building timeline/metadata/graph services internally. Instead, it should accept already-created service dependencies via a dedicated constructor input struct.

Proposed direction:

- add a `Dependencies` or `ServerDependencies` struct
- pass in application services and optional caches
- keep cache lifecycle local to `apiserver`
- keep route registration local
- remove executor-selection and service-construction logic from `apiserver`

This makes server assembly easier to test and shrinks the constructor signature.

### 7. Keep behavior stable during migration

This refactor should preserve:

- existing REST endpoints
- existing Connect/gRPC behavior
- existing MCP tool names and schemas
- integration config file format
- embedded, audit-only, and graph runtime modes

Only package ownership and dependency flow change.

## File Plan

Expected new or heavily changed files in this slice:

### New packages

- `internal/app/timeline/*.go`
- `internal/app/graph/*.go`
- `internal/app/integrationadmin/*.go`

### Existing files to shrink or adapt

- `internal/api/interfaces.go`
- `internal/api/timeline_service.go`
- `internal/api/graph_service.go`
- `internal/api/handlers/register.go`
- `internal/api/handlers/integration_config_handler.go`
- `internal/apiserver/server.go`
- `internal/mcp/server.go`
- `internal/watcher/event_handler.go`
- `cmd/spectre/commands/server.go`

## Error Handling

- Application services return typed or wrapped Go errors without HTTP-specific response behavior.
- HTTP handlers remain responsible for status-code mapping.
- MCP remains responsible for tool-result formatting.
- Existing logging should remain near adapter boundaries and lifecycle boundaries, not duplicated in every layer.

## Testing Strategy

### New tests

- unit tests for `internal/app/timeline`
- unit tests for `internal/app/graph`
- unit tests for `internal/app/integrationadmin`
- handler tests verifying delegation and response mapping still work
- MCP tests verifying tool registration still delegates correctly

### Existing tests to keep green

- timeline handler tests
- graph service related tests after package relocation
- MCP tool tests
- integration config handler tests or replacements

### Verification approach

Because the repo already has some unrelated failing tests, verification for this slice should focus on:

- targeted package tests for the modified packages
- build/test of the updated server wiring path
- any existing tests directly covering the moved seams

## Risks and Mitigations

### Risk: large import churn

Moving service types across packages will touch many imports.

Mitigation:

- perform the move in small steps
- keep package names explicit
- run targeted package builds frequently

### Risk: accidental behavior changes in handlers

Transport logic can drift while extracting services.

Mitigation:

- keep handler surface identical
- add delegation-focused tests before moving logic
- move behavior first, optimize later

### Risk: over-reaching into Grafana internals

The Grafana package is already a large bounded context and should not be redesigned as part of this slice.

Mitigation:

- treat Grafana as an existing integration implementation
- only change the integration admin seam around it

## Success Criteria

This slice is successful when:

- transport packages no longer define core timeline/query/graph ports
- HTTP and MCP depend on application services rather than on `internal/api` service types
- integration admin behavior is moved out of HTTP handlers
- `internal/apiserver` accepts service dependencies rather than creating them internally
- the modified code paths preserve existing behavior under targeted tests
- the worst transport/composition files are materially smaller and more focused

## Follow-Up Work

Not part of this implementation, but made easier by it:

- split `internal/integration/grafana` into subpackages
- decompose `internal/graph/sync/builder.go`
- decompose `internal/graph/schema.go`
- rationalize remaining large files in `internal/analysis`
