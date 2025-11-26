/**
 * API Response Types for Backend Communication
 * These interfaces match the backend API response formats
 */

// Search endpoint response
export interface SearchResponse {
  resources: Resource[];
  count: number;
  executionTimeMs: number;
}

// Minimal resource data for list views
export interface Resource {
  id: string;
  group: string;
  version: string;
  kind: string;
  namespace: string;
  name: string;
  statusSegments?: StatusSegment[];
  events?: AuditEvent[];
}

// Status segment for timeline visualization
export interface StatusSegment {
  startTime: number;  // Unix seconds
  endTime: number;    // Unix seconds
  status: 'Ready' | 'Warning' | 'Error' | 'Terminating' | 'Unknown';
  message?: string;
  config: Record<string, any>;
}

// Audit event for resource trail
export interface AuditEvent {
  id: string;
  timestamp: number;  // Unix seconds
  verb: 'create' | 'update' | 'patch' | 'delete' | 'get' | 'list';
  user: string;
  message: string;
  details?: string;
}

// Metadata response for filters
export interface MetadataResponse {
  namespaces: string[];
  kinds: string[];
  groups: string[];
  resourceCounts: Record<string, number>;
  totalEvents: number;
  timeRange: TimeRangeInfo;
}

// Time range information
export interface TimeRangeInfo {
  earliest: number;  // Unix seconds
  latest: number;    // Unix seconds
}

// Events response for resource audit trail
export interface EventsResponse {
  events: AuditEvent[];
  count: number;
  resourceId: string;
}

// Segments response for status timeline
export interface SegmentsResponse {
  segments: StatusSegment[];
  resourceId: string;
  count: number;
}
