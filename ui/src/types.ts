export enum ResourceStatus {
  Unknown = 'Unknown',
  Ready = 'Ready',
  Warning = 'Warning',
  Error = 'Error',
  Terminating = 'Terminating'
}

export interface K8sEvent {
  id: string;
  timestamp: Date;
  reason: string;
  message: string;
  type: 'Normal' | 'Warning' | string;
  count: number;
  source?: string;
  firstTimestamp?: Date;
  lastTimestamp?: Date;
}

export interface ResourceStatusSegment {
  start: Date;
  end: Date;
  status: ResourceStatus;
  message?: string;
  resourceData?: Record<string, any>;
}

export interface K8sResource {
  id: string; // unique key (e.g., uid)
  group: string;
  version: string;
  kind: string;
  namespace: string;
  name: string;
  // A simplified history of status changes for the timeline
  statusSegments: ResourceStatusSegment[];
  // Discrete audit events associated with this resource
  events: K8sEvent[];
}

export interface FilterState {
  kinds: string[];
  namespaces: string[];
  search: string;
}

export interface SelectedPoint {
  resourceId: string;
  index: number;
}

export interface TimeRange {
  start: Date;
  end: Date;
}