/**
 * API Client Service
 * Communicates with the backend API at /v1
 */

import {
  SearchResponse,
  Resource,
  StatusSegment,
  AuditEvent,
  MetadataResponse,
  EventsResponse,
  SegmentsResponse,
} from './apiTypes';
import { K8sResource, K8sEvent, ResourceStatusSegment } from '../types';
import {
  transformSearchResponse,
  transformAuditEvent,
  transformStatusSegment,
  transformAuditEventsWithErrorHandling,
  transformStatusSegmentsWithErrorHandling,
} from './dataTransformer';

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
  user: string;
  details?: string;
}

interface ApiClientConfig {
  baseUrl: string;
  timeout?: number;
}

class ApiClient {
  private baseUrl: string;
  private timeout: number = 30000; // 30 seconds default

  constructor(config: ApiClientConfig) {
    this.baseUrl = config.baseUrl;
    if (config.timeout) {
      this.timeout = config.timeout;
    }
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
        const errorBody = await response.text().catch(() => '');
        throw new Error(
          `API Error: ${response.status} ${response.statusText}${errorBody ? ` - ${errorBody}` : ''}`
        );
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
   * Fetch metadata (namespaces, kinds, resource counts)
   */
  async getMetadata(): Promise<ApiMetadata> {
    return this.request<ApiMetadata>('/api/metadata');
  }

  /**
   * Fetch all resources
   */
  async getResources(
    namespace?: string,
    kind?: string
  ): Promise<ApiResource[]> {
    const params = new URLSearchParams();
    if (namespace) params.append('namespace', namespace);
    if (kind) params.append('kind', kind);

    const query = params.toString();
    const endpoint = query ? `/api/resources?${query}` : '/api/resources';

    return this.request<ApiResource[]>(endpoint);
  }

  /**
   * Fetch events for a specific resource
   */
  async getEvents(
    resourceId: string,
    startTime?: string,
    endTime?: string
  ): Promise<ApiEvent[]> {
    const params = new URLSearchParams();
    if (startTime) params.append('startTime', startTime);
    if (endTime) params.append('endTime', endTime);

    const query = params.toString();
    const endpoint = query
      ? `/api/events/${resourceId}?${query}`
      : `/api/events/${resourceId}`;

    return this.request<ApiEvent[]>(endpoint);
  }

  /**
   * Fetch segments for a specific resource
   */
  async getSegments(resourceId: string): Promise<ApiSegment[]> {
    return this.request<ApiSegment[]>(`/api/segments/${resourceId}`);
  }

  /**
   * Search resources using /v1/search endpoint
   * Returns transformed K8sResource[] for timeline view
   */
  async searchResources(
    startTime: string | number,
    endTime: string | number,
    filters?: {
      namespace?: string;
      kind?: string;
      group?: string;
      version?: string;
    }
  ): Promise<K8sResource[]> {
    // Convert milliseconds to Unix seconds if needed
    const startSeconds = typeof startTime === 'number'
      ? Math.floor(startTime / 1000)
      : startTime;
    const endSeconds = typeof endTime === 'number'
      ? Math.floor(endTime / 1000)
      : endTime;

    const params = new URLSearchParams();
    params.append('start', startSeconds.toString());
    params.append('end', endSeconds.toString());

    if (filters?.namespace) params.append('namespace', filters.namespace);
    if (filters?.kind) params.append('kind', filters.kind);
    if (filters?.group) params.append('group', filters.group);
    if (filters?.version) params.append('version', filters.version);

    const endpoint = `/v1/search?${params.toString()}`;
    const response = await this.request<SearchResponse>(endpoint);
    return transformSearchResponse(response);
  }

  /**
   * Get metadata for filters
   */
  async getMetadataV1(
    startTime?: string | number,
    endTime?: string | number
  ): Promise<MetadataResponse> {
    const params = new URLSearchParams();

    if (startTime !== undefined) {
      const startSeconds = typeof startTime === 'number'
        ? Math.floor(startTime / 1000)
        : startTime;
      params.append('start', startSeconds.toString());
    }

    if (endTime !== undefined) {
      const endSeconds = typeof endTime === 'number'
        ? Math.floor(endTime / 1000)
        : endTime;
      params.append('end', endSeconds.toString());
    }

    const endpoint = params.toString()
      ? `/v1/metadata?${params.toString()}`
      : '/v1/metadata';

    return this.request<MetadataResponse>(endpoint);
  }

  /**
   * Get status segments for a resource
   */
  async getResourceSegments(
    resourceId: string,
    startTime?: string | number,
    endTime?: string | number
  ): Promise<ResourceStatusSegment[]> {
    const params = new URLSearchParams();

    if (startTime !== undefined) {
      const startSeconds = typeof startTime === 'number'
        ? Math.floor(startTime / 1000)
        : startTime;
      params.append('start', startSeconds.toString());
    }

    if (endTime !== undefined) {
      const endSeconds = typeof endTime === 'number'
        ? Math.floor(endTime / 1000)
        : endTime;
      params.append('end', endSeconds.toString());
    }

    const endpoint = params.toString()
      ? `/v1/resources/${resourceId}/segments?${params.toString()}`
      : `/v1/resources/${resourceId}/segments`;

    const response = await this.request<SegmentsResponse>(endpoint);
    return transformStatusSegmentsWithErrorHandling(response.segments);
  }

  /**
   * Get audit events for a resource
   */
  async getResourceEvents(
    resourceId: string,
    startTime?: string | number,
    endTime?: string | number,
    limit?: number
  ): Promise<K8sEvent[]> {
    const params = new URLSearchParams();

    if (startTime !== undefined) {
      const startSeconds = typeof startTime === 'number'
        ? Math.floor(startTime / 1000)
        : startTime;
      params.append('start', startSeconds.toString());
    }

    if (endTime !== undefined) {
      const endSeconds = typeof endTime === 'number'
        ? Math.floor(endTime / 1000)
        : endTime;
      params.append('end', endSeconds.toString());
    }

    if (limit !== undefined) {
      params.append('limit', limit.toString());
    }

    const endpoint = params.toString()
      ? `/v1/resources/${resourceId}/events?${params.toString()}`
      : `/v1/resources/${resourceId}/events`;

    const response = await this.request<EventsResponse>(endpoint);
    return transformAuditEventsWithErrorHandling(response.events);
  }

  /**
   * Health check endpoint
   */
  async healthCheck(): Promise<{ status: string }> {
    return this.request<{ status: string }>('/api/health');
  }
}

// Create singleton instance with environment-based configuration
const baseUrl =
  typeof process !== 'undefined' && process.env.VITE_API_BASE
    ? process.env.VITE_API_BASE
    : '/v1';

export const apiClient = new ApiClient({
  baseUrl,
  timeout: 30000,
});

// Export for testing/mocking
export { ApiClient };
