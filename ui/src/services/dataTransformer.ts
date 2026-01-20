/**
 * Data Transformation Service
 * Converts backend API responses to frontend K8sResource format
 *
 * This service handles the conversion between:
 * - Backend API models (with Unix timestamp seconds, string enums)
 * - Frontend models (with JavaScript Date objects, typed enums)
 *
 * Key conversions:
 * - Timestamps: Unix seconds (backend) → JavaScript Date (frontend) by multiplying by 1000
 * - Status strings: Mapped to ResourceStatus enum values
 * - Events and segments: Filtered and validated with error handling
 */

import { K8sResource, ResourceStatusSegment, K8sEvent } from '../types';
import { SearchResponse, StatusSegment, K8sEventDTO } from './apiTypes';

/**
 * Transforms backend SearchResponse to frontend K8sResource array
 */
export function transformSearchResponse(response: SearchResponse): K8sResource[] {
  if (!response || !Array.isArray(response.resources)) {
    console.warn('Invalid SearchResponse format', response);
    return [];
  }

  return response.resources
    .filter(resource => {
      // Skip resources with missing required fields
      // Note: namespace can be empty for cluster-scoped resources like Nodes
      if (!resource.id || !resource.kind || !resource.name) {
        console.warn('Skipping resource with missing fields', resource);
        return false;
      }
      return true;
    })
    .map(resource => transformResource(resource));
}

/**
 * Transforms a single Resource with segments and events
 */
export function transformResourceDetail(resource: any): K8sResource {
  return transformResource(resource);
}

/**
 * Internal function to transform resource object
 */
function transformResource(resource: any): K8sResource {
  const events = resource.events?.map(transformK8sEvent) || [];
  
  // Debug: Log resources with events
  if (events.length > 0) {
    console.log(`[dataTransformer] Resource ${resource.kind}/${resource.name} has ${events.length} events`);
  }
  
  return {
    id: resource.id,
    group: resource.group || '',
    version: resource.version || '',
    kind: resource.kind,
    namespace: resource.namespace || '(cluster)', // Cluster-scoped resources have no namespace
    name: resource.name,
    statusSegments: resource.statusSegments?.map(transformStatusSegment) || [],
    events,
  };
}

/**
 * Transforms a StatusSegment from backend to frontend format
 */
export function transformStatusSegment(segment: StatusSegment): ResourceStatusSegment {
  try {
    // Map status string to ResourceStatus enum
    const statusMap: Record<string, any> = {
      'Ready': 'Ready',
      'Warning': 'Warning',
      'Error': 'Error',
      'Terminating': 'Terminating',
      'Unknown': 'Unknown',
    };

    return {
      status: statusMap[segment.status] || 'Unknown',
      start: new Date(segment.startTime * 1000), // Convert Unix seconds to Date
      end: new Date(segment.endTime * 1000),
      message: segment.message || '',
      resourceData: segment.resourceData,
    };
  } catch (error) {
    console.error('Error transforming status segment', segment, error);
    throw new Error('Failed to transform status segment: invalid timestamp format');
  }
}

/**
 * Transforms a K8sEvent from backend to frontend format
 */
export function transformK8sEvent(event: K8sEventDTO): K8sEvent {
  try {
    return {
      id: event.id,
      timestamp: new Date(event.timestamp * 1000), // Convert Unix seconds to Date
      reason: event.reason || 'Unknown',
      message: event.message,
      type: event.type || 'Normal',
      count: event.count ?? 0,
      source: event.source,
      firstTimestamp: event.firstTimestamp ? new Date(event.firstTimestamp * 1000) : undefined,
      lastTimestamp: event.lastTimestamp ? new Date(event.lastTimestamp * 1000) : undefined,
    };
  } catch (error) {
    console.error('Error transforming Kubernetes event', event, error);
    throw new Error('Failed to transform event: invalid timestamp format');
  }
}

/**
 * Safely transforms events with error handling
 */
export function transformK8sEventsWithErrorHandling(events: K8sEventDTO[]): K8sEvent[] {
  return events.map(event => {
    try {
      return transformK8sEvent(event);
    } catch (error) {
      console.warn('Failed to transform individual event, skipping', event, error);
      // Return a placeholder event so we don't break the entire list
      return {
        id: event.id,
        timestamp: new Date(),
        reason: 'Unknown',
        message: `Failed to parse event: ${error instanceof Error ? error.message : 'unknown error'}`,
        type: 'Normal',
        count: 0,
      };
    }
  });
}

/**
 * Safely transforms segments with error handling
 */
export function transformStatusSegmentsWithErrorHandling(segments: StatusSegment[]): ResourceStatusSegment[] {
  return segments.map(segment => {
    try {
      return transformStatusSegment(segment);
    } catch (error) {
      console.warn('Failed to transform individual segment, skipping', segment, error);
      // Return a placeholder segment
      return {
        status: 'Unknown',
        start: new Date(),
        end: new Date(),
        message: `Failed to parse segment: ${error instanceof Error ? error.message : 'unknown error'}`,
        config: {},
      };
    }
  });
}
