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
import { K8sResource, K8sEvent, ResourceStatusSegment } from '../types';
import {
  transformSearchResponse,
  transformStatusSegment,
  transformK8sEventsWithErrorHandling,
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
   * Get timeline data using /v1/timeline endpoint
   * Returns full resource data with statusSegments and events for timeline visualization
   */
  async getTimeline(
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

    const endpoint = `/v1/timeline?${params.toString()}`;
    const response = await this.request<SearchResponse>(endpoint);
    return transformSearchResponse(response);
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
}

// Create singleton instance with environment-based configuration
const baseUrl =
  (typeof window !== 'undefined' ? window.location.origin : 'http://localhost:8080');

export const apiClient = new ApiClient({
  baseUrl,
  timeout: 30000,
});

// Export for testing/mocking
export { ApiClient };
