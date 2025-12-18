import { TimelineServiceClientImpl, TimelineRequest, TimelineChunk, TimelineResource, TimelineMetadata } from '../generated/timeline';
import { GrpcWebTransport } from './grpc-transport';
import { K8sResource, ResourceStatus } from '../types';
import { parseTimeExpression } from '../utils/timeParsing';

export interface TimelineStreamResult {
  metadata?: TimelineMetadata;
  resources: TimelineResource[];
  isComplete: boolean;
}

export interface StreamCallbacks {
  onMetadata: (count: number) => void;
  onBatch: (resources: K8sResource[]) => void;
  onComplete: () => void;
  onError: (error: Error) => void;
}

/**
 * Parse a timestamp (string or number) to Unix timestamp in seconds.
 * Supports:
 * - Human-friendly strings: "now", "now-24h", "now-2h", "now-30m"
 * - Numbers: treated as milliseconds, converted to seconds
 *
 * @param timestamp - String expression or number (milliseconds)
 * @returns Unix timestamp in seconds (number)
 */
function parseTimestampToSeconds(timestamp: string | number): number {
  if (typeof timestamp === 'string') {
    // Try parsing as human-friendly expression (e.g., "now", "now-24h")
    const parsedDate = parseTimeExpression(timestamp);
    if (parsedDate) {
      // Convert Date to Unix seconds
      return Math.floor(parsedDate.getTime() / 1000);
    }

    // If parsing failed, try parsing as numeric string (Unix seconds)
    const numeric = Number(timestamp);
    if (!isNaN(numeric)) {
      // If the number is very large (> year 2100 in seconds), assume it's milliseconds
      if (numeric > 4102444800) { // Jan 1, 2100 in seconds
        return Math.floor(numeric / 1000);
      }
      return Math.floor(numeric);
    }

    throw new Error(`Unable to parse timestamp: ${timestamp}`);
  }

  // Number input: treat as milliseconds and convert to seconds
  return Math.floor(timestamp / 1000);
}

export class TimelineGrpcService {
  private client: TimelineServiceClientImpl;

  constructor(baseUrl: string) {
    // gRPC-Web is served on the HTTP port, not the gRPC port
    // Use the baseUrl passed in (should be HTTP port like http://localhost:8080)
    const transport = new GrpcWebTransport(baseUrl);
    this.client = new TimelineServiceClientImpl(transport as any);
  }

  /**
   * Fetches timeline data with streaming support and intelligent batching.
   *
   * @param request Timeline query parameters
   * @param onChunk Callback invoked for each batch of resources
   * @param visibleCount Number of resources initially visible (for optimal first render)
   * @returns Promise that resolves when streaming is complete
   */
  async fetchTimeline(
    request: TimelineRequest,
    onChunk: (result: TimelineStreamResult) => void,
    visibleCount: number = 50
  ): Promise<void> {
    return new Promise((resolve, reject) => {
      const allResources: TimelineResource[] = [];
      let metadata: TimelineMetadata | undefined;
      let sentFirstBatch = false;

      this.client.GetTimeline(request).subscribe({
        next: (chunk: TimelineChunk) => {
          if (chunk.metadata) {
            // First message contains metadata
            metadata = chunk.metadata;
            onChunk({
              metadata,
              resources: [],
              isComplete: false,
            });
          }

          if (chunk.batch) {
            // Accumulate resources from this batch
            allResources.push(...chunk.batch.resources);

            // Send first batch immediately when we have enough for visible area
            if (!sentFirstBatch && allResources.length >= visibleCount) {
              sentFirstBatch = true;
              onChunk({
                metadata,
                resources: allResources.slice(0, visibleCount),
                isComplete: false,
              });
            }

            // If this is the final batch, send everything
            if (chunk.batch.isFinalBatch) {
              if (sentFirstBatch) {
                // Send remaining resources (everything after the first visible batch)
                const remaining = allResources.slice(visibleCount);
                if (remaining.length > 0) {
                  onChunk({
                    metadata,
                    resources: remaining,
                    isComplete: true,
                  });
                } else {
                  // All resources were already sent in first batch
                  onChunk({
                    metadata,
                    resources: [],
                    isComplete: true,
                  });
                }
              } else {
                // Small dataset - send everything at once
                onChunk({
                  metadata,
                  resources: allResources,
                  isComplete: true,
                });
              }
            }
          }
        },
        error: (err) => {
          reject(err);
        },
        complete: () => {
          resolve();
        },
      });
    });
  }

  /**
   * Fetches timeline data without streaming (waits for all data).
   * Useful for backward compatibility or when streaming is not needed.
   */
  async fetchTimelineComplete(request: TimelineRequest): Promise<{
    metadata: TimelineMetadata;
    resources: TimelineResource[];
  }> {
    return new Promise((resolve, reject) => {
      const allResources: TimelineResource[] = [];
      let metadata: TimelineMetadata | undefined;

      this.client.GetTimeline(request).subscribe({
        next: (chunk: TimelineChunk) => {
          if (chunk.metadata) {
            metadata = chunk.metadata;
          }
          if (chunk.batch) {
            allResources.push(...chunk.batch.resources);
          }
        },
        error: (err) => {
          reject(err);
        },
        complete: () => {
          if (!metadata) {
            reject(new Error('No metadata received'));
            return;
          }
          resolve({ metadata, resources: allResources });
        },
      });
    });
  }

  /**
   * Stream timeline data with progress callbacks for React hooks.
   * Converts gRPC protobuf types to application K8sResource types.
   */
  async streamTimeline(
    start: string | number,
    end: string | number,
    filters?: { namespace?: string; kind?: string; group?: string; version?: string },
    callbacks?: StreamCallbacks,
    signal?: AbortSignal
  ): Promise<void> {
    // Parse timestamps to Unix seconds
    const startTimestamp = parseTimestampToSeconds(start);
    const endTimestamp = parseTimestampToSeconds(end);

    const request: TimelineRequest = {
      startTimestamp,
      endTimestamp,
      namespace: filters?.namespace || '',
      kind: filters?.kind || '',
      name: '',
      labelSelector: '',
    };

    return new Promise((resolve, reject) => {
      if (signal?.aborted) {
        reject(new Error('AbortError'));
        return;
      }

      const abortHandler = () => {
        reject(new Error('AbortError'));
      };
      signal?.addEventListener('abort', abortHandler);

      this.client.GetTimeline(request).subscribe({
        next: (chunk: TimelineChunk) => {
          try {
            if (chunk.metadata) {
              callbacks?.onMetadata(chunk.metadata.totalCount);
            }

            if (chunk.batch && chunk.batch.resources.length > 0) {
              // Convert protobuf resources to K8sResource format
              // gRPC timestamps are in seconds (Unix timestamp), convert to Date objects
              const resources: K8sResource[] = chunk.batch.resources.map(r => {
                // Parse apiVersion to extract group and version
                const [groupVersion, version] = r.apiVersion.includes('/')
                  ? r.apiVersion.split('/')
                  : ['', r.apiVersion];

                return {
                  id: r.id,
                  name: r.name,
                  kind: r.kind,
                  group: groupVersion,
                  version: version || groupVersion,
                  namespace: r.namespace,
                  statusSegments: r.statusSegments.map(s => ({
                    status: s.status as ResourceStatus,
                    start: new Date(Number(s.start) * 1000),
                    end: new Date(Number(s.end) * 1000),
                    message: s.message || undefined,
                    resourceData: s.config ? JSON.parse(s.config) : undefined,
                  })),
                  events: r.events.map(e => ({
                    id: e.id,
                    timestamp: new Date(Number(e.timestamp) * 1000),
                    reason: e.reason,
                    message: e.message,
                    type: e.type,
                    count: e.count,
                    source: e.source,
                    firstTimestamp: e.firstTimestamp ? new Date(Number(e.firstTimestamp) * 1000) : undefined,
                    lastTimestamp: e.lastTimestamp ? new Date(Number(e.lastTimestamp) * 1000) : undefined,
                  })),
                };
              });

              callbacks?.onBatch(resources);
            }
          } catch (err) {
            callbacks?.onError(err instanceof Error ? err : new Error('Processing error'));
          }
        },
        error: (err) => {
          signal?.removeEventListener('abort', abortHandler);
          callbacks?.onError(err instanceof Error ? err : new Error('gRPC error'));
          reject(err);
        },
        complete: () => {
          signal?.removeEventListener('abort', abortHandler);
          callbacks?.onComplete();
          resolve();
        },
      });
    });
  }
}

// Export singleton instance
export const timelineGrpcClient = new TimelineGrpcService(
  import.meta.env.VITE_API_URL || window.location.origin
);
