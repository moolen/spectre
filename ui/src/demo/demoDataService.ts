import rawDataset from './demo-data.json';
import { K8sEventDTO, MetadataResponse, Resource, SearchResponse, StatusSegment } from '../services/apiTypes';

type DemoDataset = {
  version: number;
  timeRange: {
    earliestOffsetSec: number;
    latestOffsetSec: number;
  };
  resources: DemoResource[];
  metadata: {
    namespaces: string[];
    kinds: string[];
    groups: string[];
    resourceCounts: Record<string, number>;
    totalEvents: number;
  };
};

type DemoResource = {
  id: string;
  group: string;
  version: string;
  kind: string;
  namespace: string;
  name: string;
  statusSegments: DemoStatusSegment[];
  events?: DemoEvent[];
};

type DemoStatusSegment = {
  startOffsetSec: number;
  endOffsetSec: number;
  status: string;
  message?: string;
  resourceData?: Record<string, any>;
};

type DemoEvent = {
  id: string;
  timestampOffsetSec: number;
  reason: string;
  message: string;
  type: string;
  count: number;
  source?: string;
  firstTimestampOffsetSec?: number;
  lastTimestampOffsetSec?: number;
};

export type TimelineFilters = {
  namespace?: string;
  kind?: string;
  namespaces?: string[];
  kinds?: string[];
  group?: string;
  version?: string;
  pageSize?: number;
  cursor?: string;
};

const dataset: DemoDataset = rawDataset as DemoDataset;

/**
 * Builds a SearchResponse equivalent using the embedded demo dataset.
 * Offsets from the JSON payload are re-anchored to the requested start time,
 * mimicking the backend behavior that seeds demo data relative to the query window.
 */
export function buildDemoTimelineResponse(
  startSeconds: number,
  filters?: TimelineFilters
): SearchResponse {
  const filteredResources = dataset.resources.filter(resource => matchesFilters(resource, filters));

  const resources: Resource[] = filteredResources.map(resource => ({
    id: resource.id,
    group: resource.group,
    version: resource.version,
    kind: resource.kind,
    namespace: resource.namespace,
    name: resource.name,
    statusSegments: resource.statusSegments.map<StatusSegment>(segment => ({
      status: segment.status as StatusSegment['status'],
      startTime: startSeconds + segment.startOffsetSec,
      endTime: startSeconds + segment.endOffsetSec,
      message: segment.message,
      resourceData: segment.resourceData,
    })),
    events: resource.events?.map<K8sEventDTO>(event => ({
      id: event.id,
      timestamp: startSeconds + event.timestampOffsetSec,
      reason: event.reason,
      message: event.message,
      type: event.type,
      count: event.count,
      source: event.source,
      firstTimestamp: event.firstTimestampOffsetSec !== undefined
        ? startSeconds + event.firstTimestampOffsetSec
        : undefined,
      lastTimestamp: event.lastTimestampOffsetSec !== undefined
        ? startSeconds + event.lastTimestampOffsetSec
        : undefined,
    })),
  }));

  return {
    resources,
    count: resources.length,
    executionTimeMs: 1,
  };
}

/**
 * Builds a MetadataResponse using the embedded dataset and anchors the
 * relative offsets to the caller's provided time window.
 */
export function buildDemoMetadata(
  startSeconds: number,
  endSeconds: number
): MetadataResponse {
  const { metadata, timeRange } = dataset;

  return {
    namespaces: metadata.namespaces,
    kinds: metadata.kinds,
    groups: metadata.groups,
    resourceCounts: metadata.resourceCounts,
    totalEvents: metadata.totalEvents,
    timeRange: {
      earliest: startSeconds + timeRange.earliestOffsetSec,
      latest: Math.max(endSeconds, startSeconds + timeRange.latestOffsetSec),
    },
  };
}

function matchesFilters(resource: DemoResource, filters?: TimelineFilters): boolean {
  if (!filters) {
    return true;
  }

  // Single-value filters (backward compatibility)
  if (filters.namespace && resource.namespace !== filters.namespace) {
    return false;
  }
  if (filters.kind && resource.kind !== filters.kind) {
    return false;
  }

  // Multi-value filters (take precedence over single-value if both provided)
  if (filters.namespaces && filters.namespaces.length > 0) {
    if (!filters.namespaces.includes(resource.namespace)) {
      return false;
    }
  }
  if (filters.kinds && filters.kinds.length > 0) {
    if (!filters.kinds.includes(resource.kind)) {
      return false;
    }
  }

  if (filters.group && resource.group !== filters.group) {
    return false;
  }
  if (filters.version && resource.version !== filters.version) {
    return false;
  }
  return true;
}
