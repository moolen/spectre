# Research: UI-API Integration

**Phase**: 0 - Research & Unknowns Resolution
**Date**: 2025-11-26

## Summary

This document consolidates research findings and design decisions for integrating the React UI with the Go backend API, replacing mock data with real API calls.

## Research Topics

### 1. Backend Response Format Design

**Topic**: How to structure the `/v1/search` API response to include resources, status segments, and events

**Research Findings**:
- The existing `QueryExecutor` in internal/storage is designed to query event data
- The API currently supports filtering by timestamp range and resource attributes
- Current response is a generic query result - needs to be enriched with resource-specific data

**Decision**: Extend the `/v1/search` response to wrap resources with their full audit histories
- Resources will be the primary entity in the response
- Each resource includes embedded statusSegments array (computed from events)
- Each resource includes embedded events array (the audit events)
- Response includes metadata: count of resources and query execution time

**Rationale**:
- Eliminates need for multiple API calls (one for resources, one for events)
- Matches UI's data model expectations (K8sResource contains both segments and events)
- Improves performance by reducing network round-trips

**Alternatives Considered**:
- Separate endpoints for resources, segments, and events - rejected because it requires 3x API calls
- GraphQL with nested queries - rejected because Go backend is already HTTP/JSON based
- Streaming response - rejected because data volumes are manageable with single response

### 2. Data Transformation Strategy

**Topic**: Converting Go backend response format to TypeScript K8sResource type

**Research Findings**:
- Frontend types are already well-defined in ui/src/types.ts
- K8sResource expects timestamps as Date objects; backend will return Unix seconds
- Status values already match enumeration expectations

**Decision**: Implement a transformation layer in the API service
- Function: `transformSearchResponse(response: any): K8sResource[]`
- Converts Unix timestamps (seconds) to JavaScript Date objects
- Maps backend field names to UI field names if they differ
- Validates required fields and provides defaults for optional ones

**Rationale**:
- Isolation principle: API service handles backend format, components handle UI format
- Easier to test and debug timestamp conversions
- Handles API schema changes without modifying components

**Alternatives Considered**:
- Direct assignment without transformation - rejected because timestamp conversion is necessary
- Transformation at component level - rejected because it violates separation of concerns

### 3. Error Handling Patterns

**Topic**: User-friendly error handling for API failures

**Research Findings**:
- Existing error handling in apiClient uses try/catch with timeout detection
- ErrorBoundary component exists in UI for error display
- Four main error categories: timeout, network, validation, server errors

**Decision**: Implement tiered error handling
1. **Network/Timeout Errors**: "Service is temporarily unavailable. Please try again."
2. **Validation Errors** (400): "Invalid search parameters. Check your filters and try again."
3. **Server Errors** (500): "An error occurred while fetching data. Please try again."
4. **Parsing Errors**: "Received unexpected data format from server."

**Rationale**:
- Non-technical language appropriate for all users
- Clear actionability (what user should do)
- Consistent with existing error handling patterns

**Alternatives Considered**:
- Showing raw API errors - rejected because too technical
- Silent failure with retry - rejected because user should be aware of issues
- Detailed error codes - rejected because users won't understand them

### 4. Default Time Range Strategy

**Topic**: What time range to use when user doesn't specify dates

**Research Findings**:
- Timeline UI currently uses last 2 hours in mock data generation
- Most Kubernetes event audits are recent
- Users typically audit recent changes

**Decision**: Default to last 2 hours from current time
- Start: `Date.now() - 2 * 60 * 60 * 1000` (milliseconds)
- End: `Date.now()`
- User can override by explicitly setting filters

**Rationale**:
- Aligns with existing mock data patterns
- Matches user expectations for "recent" events
- Provides reasonable default without requiring user input
- Reduces API load by limiting historical data fetched

**Alternatives Considered**:
- Last 24 hours - too broad, slower queries
- No default (require user input) - poor UX, adds friction
- Configurable defaults - premature complexity

### 5. API Response Timeout Configuration

**Topic**: How long to wait for API responses before timing out

**Research Findings**:
- Existing apiClient has 30-second default timeout
- Frontend performance goal is 3 seconds for typical queries
- Some large queries may take up to 30 seconds

**Decision**: Keep existing 30-second timeout in apiClient
- Reasonable upper bound for backend query execution
- Allows for larger historical queries without failing
- User sees timeout error if exceeded

**Rationale**:
- Prevents hanging requests
- 30 seconds is standard for backend operations
- Better to wait longer than timeout prematurely

**Alternatives Considered**:
- 5 seconds - too aggressive for large queries
- 60+ seconds - could appear to user as hung
- 3 seconds - matches performance goal but too strict for all scenarios

## Implementation Decisions Summary

| Area | Decision | Impact |
|------|----------|--------|
| Response Format | Embed resources with statusSegments and events | Simplifies frontend, reduces API calls |
| Transformation | Dedicated transformer function | Maintainability, testability |
| Error Messages | User-friendly, actionable text | Better UX |
| Default Time Range | Last 2 hours from now | Matches expectations |
| Timeout | 30 seconds | Balances UX and reliability |

## Next Steps

- Proceed to Phase 1: Design API contracts and data models
- Confirm backend response format matches this specification
- Implement API contracts in contracts/ directory
