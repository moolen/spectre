/**
 * API Client Service
 * Communicates with the backend API at /internal/api
 */

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
        throw new Error(
          `API Error: ${response.status} ${response.statusText}`
        );
      }

      return await response.json();
    } catch (error) {
      if (error instanceof Error) {
        if (error.name === 'AbortError') {
          throw new Error(`Request timeout (${this.timeout}ms)`);
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
    : '/api';

export const apiClient = new ApiClient({
  baseUrl,
  timeout: 30000,
});

// Export for testing/mocking
export { ApiClient };
