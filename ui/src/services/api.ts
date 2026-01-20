/**
 * API Client Service
 * Communicates with the backend API at /v1
 */

import {
  SearchResponse,
  Resource,
  StatusSegment,
  MetadataResponse,
  EventsResponse,
  SegmentsResponse,
} from './apiTypes';
import { K8sResource, K8sEvent, ResourceStatusSegment, ResourceStatus } from '../types';
import {
  transformSearchResponse,
  transformStatusSegment,
  transformK8sEventsWithErrorHandling,
  transformStatusSegmentsWithErrorHandling,
} from './dataTransformer';
import { NamespaceGraphRequest, NamespaceGraphResponse } from '../types/namespaceGraph';
import { isHumanFriendlyExpression, parseTimeExpression } from '../utils/timeParsing';
import { TimelineGrpcService, TimelineStreamResult as GrpcStreamResult } from './timeline-grpc';
import { TimelineResource as GrpcTimelineResource, TimelineMetadata } from '../generated/timeline';

export interface ApiTimelineStreamResult {
  metadata?: TimelineMetadata;
  resources: K8sResource[];
  isComplete: boolean;
}

export interface TimelineFilters {
  namespace?: string;
  namespaces?: string[];
  kind?: string;
  kinds?: string[];
  group?: string;
  version?: string;
  pageSize?: number;
  cursor?: string;
}

export interface ApiMetadata {
  namespaces: string[];
  kinds: string[];
  resourceCounts: Record<string, number>;
}

export interface ApiResource {
  id: string;
  name: string;
  kind: string;
  apiVersion: string;
  namespace: string;
  createdAt: string;
  deletedAt?: string;
  labels?: Record<string, string>;
}

export interface ApiSegment {
  id: string;
  resourceId: string;
  status: 'Ready' | 'Warning' | 'Error' | 'Terminating' | 'Unknown';
  startTime: string;
  endTime: string;
  message?: string;
  configuration: Record<string, any>;
}

export interface ApiEvent {
  id: string;
  timestamp: string;
  verb: 'create' | 'update' | 'patch' | 'delete' | 'get' | 'list';
  message: string;
  details?: string;
}

interface ApiClientConfig {
  baseUrl: string;
  timeout?: number;
}

class ApiClient {
  private baseUrl: string;
  private timeout: number = 180000; // 3 minutes default for resource-constrained environments
  private grpcService: TimelineGrpcService;

  constructor(config: ApiClientConfig) {
    this.baseUrl = config.baseUrl;
    if (config.timeout) {
      this.timeout = config.timeout;
    }
    // gRPC-Web is served on the HTTP port, not the gRPC port
    this.grpcService = new TimelineGrpcService(this.baseUrl);
  }

  /**
   * Make a fetch request with error handling
   */
  private async request<T>(
    endpoint: string,
    options?: RequestInit
  ): Promise<T> {
    const url = `${this.baseUrl}${endpoint}`;
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), this.timeout);

    try {
      const response = await fetch(url, {
        ...options,
        signal: controller.signal,
        headers: {
          'Content-Type': 'application/json',
          ...options?.headers,
        },
      });

      if (!response.ok) {
        // Try to parse structured error response
        try {
          const errorData = await response.json();
          if (errorData.error && errorData.message) {
            throw new Error(`API error (${errorData.error}): ${errorData.message}`);
          }
          throw new Error(`API Error: ${response.status} ${response.statusText} - ${JSON.stringify(errorData)}`);
        } catch (jsonError) {
          // If JSON parsing fails, fall back to text
          const errorBody = await response.text().catch(() => '');
          throw new Error(
            `API Error: ${response.status} ${response.statusText}${errorBody ? ` - ${errorBody}` : ''}`
          );
        }
      }

      return await response.json();
    } catch (error) {
      if (error instanceof Error) {
        if (error.name === 'AbortError') {
          throw new Error(`Request timeout (${this.timeout}ms) - Backend server may be unavailable`);
        }
        if (error.message.includes('Failed to fetch')) {
          throw new Error('Network error - Unable to connect to backend server. Please check that the server is running.');
        }
        throw error;
      }
      throw new Error('Unknown error occurred');
    } finally {
      clearTimeout(timeoutId);
    }
  }

  /**
   * Get timeline data using /v1/timeline endpoint
   * Returns full resource data with statusSegments and events for timeline visualization
   */
  async getTimeline(
    startTime: string | number,
    endTime: string | number,
    filters?: TimelineFilters
  ): Promise<K8sResource[]> {
    const params = new URLSearchParams();

    // If startTime/endTime are strings that look like human-friendly expressions, pass them through
    // Otherwise normalize to seconds
    if (typeof startTime === 'string' && isHumanFriendlyExpression(startTime)) {
      params.append('start', startTime);
    } else {
      const startSeconds = normalizeToSeconds(startTime);
      params.append('start', startSeconds.toString());
    }

    if (typeof endTime === 'string' && isHumanFriendlyExpression(endTime)) {
      params.append('end', endTime);
    } else {
      const endSeconds = normalizeToSeconds(endTime);
      params.append('end', endSeconds.toString());
    }

    if (filters?.namespace) params.append('namespace', filters.namespace);
    if (filters?.kind) params.append('kind', filters.kind);
    if (filters?.group) params.append('group', filters.group);
    if (filters?.version) params.append('version', filters.version);

    const endpoint = `/v1/timeline?${params.toString()}`;
    const response = await this.request<SearchResponse>(endpoint);
    return transformSearchResponse(response);
  }

  /**
   * Get timeline data using gRPC streaming
   * Returns timeline data in batches for progressive rendering
   */
  async getTimelineGrpc(
    startTime: string | number,
    endTime: string | number,
    filters?: TimelineFilters,
    onChunk?: (result: ApiTimelineStreamResult) => void,
    visibleCount: number = 50
  ): Promise<K8sResource[]> {
    // Normalize timestamps to seconds
    const startSeconds = typeof startTime === 'string' && isHumanFriendlyExpression(startTime)
      ? Math.floor((parseTimeExpression(startTime)?.getTime() ?? Date.now()) / 1000)
      : normalizeToSeconds(startTime);

    const endSeconds = typeof endTime === 'string' && isHumanFriendlyExpression(endTime)
      ? Math.floor((parseTimeExpression(endTime)?.getTime() ?? Date.now()) / 1000)
      : normalizeToSeconds(endTime);

    const request = {
      startTimestamp: startSeconds,
      endTimestamp: endSeconds,
      namespace: filters?.namespace ?? '',
      kind: filters?.kind ?? '',
      namespaces: filters?.namespaces ?? [],
      kinds: filters?.kinds ?? [],
      name: '',
      labelSelector: '',
      pageSize: filters?.pageSize ?? 0,
      cursor: filters?.cursor ?? '',
    };

    const allResources: K8sResource[] = [];

    if (onChunk) {
      // Streaming mode - invoke callback for each chunk
      await this.grpcService.fetchTimeline(request, (result) => {
        // Transform gRPC resources to K8sResource format
        const transformed = result.resources.map(r => this.transformGrpcResource(r));
        allResources.push(...transformed);
        console.log(result, transformed)

        // Forward to caller with transformed data
        onChunk({
          metadata: result.metadata,
          resources: transformed,
          isComplete: result.isComplete,
        });
      }, visibleCount);
    } else {
      // Non-streaming mode - wait for all data
      const result = await this.grpcService.fetchTimelineComplete(request);
      allResources.push(...result.resources.map(r => this.transformGrpcResource(r)));
    }

    return allResources;
  }

  /**
   * Transform gRPC TimelineResource to K8sResource format
   */
  private transformGrpcResource(grpcResource: GrpcTimelineResource): K8sResource {
    // Parse apiVersion to extract group and version
    const [groupVersion, version] = grpcResource.apiVersion.includes('/')
      ? grpcResource.apiVersion.split('/')
      : ['', grpcResource.apiVersion];

    return {
      id: grpcResource.id,
      name: grpcResource.name,
      kind: grpcResource.kind,
      group: groupVersion,
      version: version || groupVersion,
      namespace: grpcResource.namespace,
      preExisting: grpcResource.preExisting,
      deletedAt: grpcResource.deletedAt && grpcResource.deletedAt > 0
        ? new Date(grpcResource.deletedAt * 1000)
        : undefined,
      statusSegments: grpcResource.statusSegments.map(seg => ({
        start: new Date(seg.startTime * 1000),
        end: new Date(seg.endTime * 1000),
        status: seg.status as any as ResourceStatus,
        message: seg.message || undefined,
        resourceData: seg.resourceData ? this.decodeResourceData(seg.resourceData) : undefined,
      })),
      events: grpcResource.events.map((evt, idx) => ({
        id: evt.uid || `evt-${idx}`,
        timestamp: new Date(evt.timestamp * 1000),
        type: evt.type,
        reason: evt.reason,
        message: evt.message,
        count: 1, // gRPC events don't have count, default to 1
      })),
    };
  }

  /**
   * Decode resource data from gRPC bytes to JSON object
   */
  private decodeResourceData(data: Uint8Array): any {
    try {
      // Convert Uint8Array to string
      const decoder = new TextDecoder('utf-8');
      const jsonString = decoder.decode(data);
      // Parse JSON string to object
      return JSON.parse(jsonString);
    } catch (error) {
      console.error('Failed to decode resourceData:', error);
      return undefined;
    }
  }

  /**
   * Get metadata for filters
   */
  async getMetadata(
    startTime?: string | number,
    endTime?: string | number
  ): Promise<MetadataResponse> {
    const params = new URLSearchParams();

    if (startTime !== undefined) {
      if (typeof startTime === 'string' && isHumanFriendlyExpression(startTime)) {
        params.append('start', startTime);
      } else {
        const normalizedStart = normalizeToSeconds(startTime);
        params.append('start', normalizedStart.toString());
      }
    }

    if (endTime !== undefined) {
      if (typeof endTime === 'string' && isHumanFriendlyExpression(endTime)) {
        params.append('end', endTime);
      } else {
        const normalizedEnd = normalizeToSeconds(endTime);
        params.append('end', normalizedEnd.toString());
      }
    }

    const endpoint = params.toString()
      ? `/v1/metadata?${params.toString()}`
      : '/v1/metadata';

    return await this.request<MetadataResponse>(endpoint);
  }

  /**
   * Export storage data
   * Returns a Blob that can be downloaded (gzipped JSON)
   */
  async exportData(options: {
    from: string;
    to: string;
    clusterId?: string;
    instanceId?: string;
  }): Promise<Blob> {
    // Convert human-readable time strings to Unix timestamps (seconds)
    const fromDate = parseTimeExpression(options.from);
    const toDate = parseTimeExpression(options.to);
    
    if (!fromDate || !toDate) {
      throw new Error('Invalid time range: could not parse start or end time');
    }
    
    const fromTimestamp = Math.floor(fromDate.getTime() / 1000);
    const toTimestamp = Math.floor(toDate.getTime() / 1000);
    
    const params = new URLSearchParams();
    params.append('from', fromTimestamp.toString());
    params.append('to', toTimestamp.toString());
    if (options.clusterId) params.append('cluster_id', options.clusterId);
    if (options.instanceId) params.append('instance_id', options.instanceId);

    const url = `${this.baseUrl}/v1/storage/export?${params.toString()}`;
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), this.timeout);

    try {
      const response = await fetch(url, {
        signal: controller.signal,
        headers: {
          'Content-Type': 'application/json',
        },
      });

      if (!response.ok) {
        // Try to parse structured error response
        try {
          const errorData = await response.json();
          if (errorData.error && errorData.message) {
            throw new Error(`API error (${errorData.error}): ${errorData.message}`);
          }
          throw new Error(`API Error: ${response.status} ${response.statusText} - ${JSON.stringify(errorData)}`);
        } catch (jsonError) {
          // If JSON parsing fails, fall back to text
          const errorBody = await response.text().catch(() => '');
          throw new Error(
            `API Error: ${response.status} ${response.statusText}${errorBody ? ` - ${errorBody}` : ''}`
          );
        }
      }

      return await response.blob();
    } catch (error) {
      if (error instanceof Error) {
        if (error.name === 'AbortError') {
          throw new Error(`Request timeout (${this.timeout}ms) - Backend server may be unavailable`);
        }
        if (error.message.includes('Failed to fetch')) {
          throw new Error('Network error - Unable to connect to backend server. Please check that the server is running.');
        }
        throw error;
      }
      throw new Error('Unknown error occurred');
    } finally {
      clearTimeout(timeoutId);
    }
  }

  /**
   * Import storage data from a file
   */
  async importData(
    file: File,
    options?: {
      validate?: boolean;
      overwrite?: boolean;
    }
  ): Promise<{ total_events: number; imported_files: number }> {
    const params = new URLSearchParams();
    if (options?.validate !== undefined) {
      params.append('validate', options.validate.toString());
    } else {
      params.append('validate', 'true');
    }
    if (options?.overwrite !== undefined) {
      params.append('overwrite', options.overwrite.toString());
    } else {
      params.append('overwrite', 'true');
    }

    const url = `${this.baseUrl}/v1/storage/import?${params.toString()}`;
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), this.timeout);

    try {
      const response = await fetch(url, {
        method: 'POST',
        body: file,
        signal: controller.signal,
        headers: {
          'Content-Type': 'application/vnd.spectre.events.v1+bin',
        },
      });

      if (!response.ok) {
        // Try to parse structured error response
        try {
          const errorData = await response.json();
          if (errorData.error && errorData.message) {
            throw new Error(`API error (${errorData.error}): ${errorData.message}`);
          }
          throw new Error(`API Error: ${response.status} ${response.statusText} - ${JSON.stringify(errorData)}`);
        } catch (jsonError) {
          // If JSON parsing fails, fall back to text
          const errorText = await response.text().catch(() => '');
          throw new Error(`Import failed: ${response.status}${errorText ? ` - ${errorText}` : ''}`);
        }
      }

      return await response.json();
    } catch (error) {
      if (error instanceof Error) {
        if (error.name === 'AbortError') {
          throw new Error(`Request timeout (${this.timeout}ms) - Backend server may be unavailable`);
        }
        if (error.message.includes('Failed to fetch')) {
          throw new Error('Network error - Unable to connect to backend server. Please check that the server is running.');
        }
        throw error;
      }
      throw new Error('Unknown error occurred');
    } finally {
      clearTimeout(timeoutId);
    }
  }

  /**
   * Get namespace graph data for visualization
   * Returns resource graph with optional anomalies and causal paths
   */
  async getNamespaceGraph(params: NamespaceGraphRequest): Promise<NamespaceGraphResponse> {
    const queryParams = new URLSearchParams();
    queryParams.append('namespace', params.namespace);
    queryParams.append('timestamp', params.timestamp.toString());
    
    if (params.includeAnomalies) {
      queryParams.append('includeAnomalies', 'true');
    }
    if (params.includeCausalPaths) {
      queryParams.append('includeCausalPaths', 'true');
    }
    if (params.lookback) {
      queryParams.append('lookback', params.lookback);
    }
    if (params.maxDepth !== undefined) {
      queryParams.append('maxDepth', params.maxDepth.toString());
    }
    if (params.limit !== undefined) {
      queryParams.append('limit', params.limit.toString());
    }
    if (params.cursor) {
      queryParams.append('cursor', params.cursor);
    }

    const endpoint = `/v1/namespace-graph?${queryParams.toString()}`;
    return this.request<NamespaceGraphResponse>(endpoint);
  }
}

// Create singleton instance with environment-based configuration
const baseUrl =
  (typeof window !== 'undefined' ? window.location.origin : 'http://localhost:8080');

export const apiClient = new ApiClient({
  baseUrl,
  timeout: 180000, // 3 minutes for slow environments
});

// Export for testing/mocking
export { ApiClient };

function normalizeToSeconds(value: string | number): number {
  if (typeof value === 'number') {
    return Math.floor(value / 1000);
  }

  const numeric = Number(value);
  if (!Number.isNaN(numeric)) {
    return Math.floor(numeric);
  }

  const parsedDate = new Date(value);
  if (!Number.isNaN(parsedDate.getTime())) {
    return Math.floor(parsedDate.getTime() / 1000);
  }

  throw new Error(`Unable to parse timestamp value: ${value}`);
}
