/**
 * Data Transformation Service
 * Converts backend API responses to frontend K8sResource format
 */

import { K8sResource, ResourceStatusSegment, K8sEvent } from '../types';
import { SearchResponse, StatusSegment, AuditEvent } from './apiTypes';

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
      if (!resource.id || !resource.kind || !resource.namespace || !resource.name) {
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
  return {
    id: resource.id,
    group: resource.group,
    version: resource.version,
    kind: resource.kind,
    namespace: resource.namespace,
    name: resource.name,
    statusSegments: resource.statusSegments?.map(transformStatusSegment) || [],
    events: resource.events?.map(transformAuditEvent) || [],
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
      config: segment.config || {},
    };
  } catch (error) {
    console.error('Error transforming status segment', segment, error);
    throw new Error('Failed to transform status segment: invalid timestamp format');
  }
}

/**
 * Transforms an AuditEvent from backend to frontend format
 */
export function transformAuditEvent(event: AuditEvent): K8sEvent {
  try {
    return {
      id: event.id,
      timestamp: new Date(event.timestamp * 1000), // Convert Unix seconds to Date
      verb: event.verb,
      user: event.user,
      message: event.message,
      details: event.details || '',
    };
  } catch (error) {
    console.error('Error transforming audit event', event, error);
    throw new Error('Failed to transform event: invalid timestamp format');
  }
}

/**
 * Safely transforms events with error handling
 */
export function transformAuditEventsWithErrorHandling(events: AuditEvent[]): K8sEvent[] {
  return events.map(event => {
    try {
      return transformAuditEvent(event);
    } catch (error) {
      console.warn('Failed to transform individual event, skipping', event, error);
      // Return a placeholder event so we don't break the entire list
      return {
        id: event.id,
        timestamp: new Date(),
        verb: event.verb || 'unknown' as any,
        user: event.user || 'unknown',
        message: `Failed to parse event: ${error instanceof Error ? error.message : 'unknown error'}`,
        details: '',
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
        startTime: new Date(),
        endTime: new Date(),
        message: `Failed to parse segment: ${error instanceof Error ? error.message : 'unknown error'}`,
        config: {},
      };
    }
  });
}
