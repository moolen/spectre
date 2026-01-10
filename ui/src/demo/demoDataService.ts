import rawDataset from './demo-data.json';
import { K8sEventDTO, MetadataResponse, Resource, SearchResponse, StatusSegment } from '../services/apiTypes';
import { NamespaceGraphResponse, GraphNode, GraphEdge } from '../types/namespaceGraph';
import { Anomaly, CausalPath, PathNode, PathStep } from '../types/rootCause';

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

/**
 * Builds a demo NamespaceGraphResponse with realistic K8s resource relationships.
 * Creates a typical microservices topology with:
 * - Deployments -> ReplicaSets -> Pods
 * - Services -> Endpoints selecting Pods
 * - ConfigMaps/Secrets referenced by Pods
 * - Some resources in Warning/Error state with anomalies
 */
export function buildDemoNamespaceGraphResponse(
  namespace: string,
  _timestampNanos: number
): NamespaceGraphResponse {
  const now = new Date().toISOString();
  const timestampNanos = Date.now() * 1_000_000;

  // Generate UIDs based on namespace for consistency
  const uid = (kind: string, name: string) => `${namespace}-${kind.toLowerCase()}-${name}`;

  const nodes: GraphNode[] = [
    // === Web App Stack ===
    {
      uid: uid('Deployment', 'web-app'),
      kind: 'Deployment',
      apiGroup: 'apps',
      namespace,
      name: 'web-app',
      status: 'Ready',
      labels: { app: 'web-app', tier: 'frontend' },
    },
    {
      uid: uid('ReplicaSet', 'web-app-7d9f8b6c5'),
      kind: 'ReplicaSet',
      apiGroup: 'apps',
      namespace,
      name: 'web-app-7d9f8b6c5',
      status: 'Ready',
      labels: { app: 'web-app', 'pod-template-hash': '7d9f8b6c5' },
    },
    {
      uid: uid('Pod', 'web-app-7d9f8b6c5-abc12'),
      kind: 'Pod',
      namespace,
      name: 'web-app-7d9f8b6c5-abc12',
      status: 'Ready',
      labels: { app: 'web-app', 'pod-template-hash': '7d9f8b6c5' },
    },
    {
      uid: uid('Pod', 'web-app-7d9f8b6c5-def34'),
      kind: 'Pod',
      namespace,
      name: 'web-app-7d9f8b6c5-def34',
      status: 'Error', // Failing pod
      labels: { app: 'web-app', 'pod-template-hash': '7d9f8b6c5' },
      latestEvent: {
        timestamp: timestampNanos - 60_000_000_000, // 1 min ago
        eventType: 'MODIFIED',
        description: 'Container crashed with exit code 137',
      },
    },
    {
      uid: uid('Service', 'web-app'),
      kind: 'Service',
      namespace,
      name: 'web-app',
      status: 'Ready',
      labels: { app: 'web-app' },
    },
    {
      uid: uid('ConfigMap', 'web-app-config'),
      kind: 'ConfigMap',
      namespace,
      name: 'web-app-config',
      status: 'Ready',
      labels: { app: 'web-app' },
    },

    // === API Backend Stack ===
    {
      uid: uid('Deployment', 'api-server'),
      kind: 'Deployment',
      apiGroup: 'apps',
      namespace,
      name: 'api-server',
      status: 'Warning', // Degraded
      labels: { app: 'api-server', tier: 'backend' },
    },
    {
      uid: uid('ReplicaSet', 'api-server-5c7d8e9f1'),
      kind: 'ReplicaSet',
      apiGroup: 'apps',
      namespace,
      name: 'api-server-5c7d8e9f1',
      status: 'Warning',
      labels: { app: 'api-server', 'pod-template-hash': '5c7d8e9f1' },
    },
    {
      uid: uid('Pod', 'api-server-5c7d8e9f1-xyz99'),
      kind: 'Pod',
      namespace,
      name: 'api-server-5c7d8e9f1-xyz99',
      status: 'Ready',
      labels: { app: 'api-server', 'pod-template-hash': '5c7d8e9f1' },
    },
    {
      uid: uid('Service', 'api-server'),
      kind: 'Service',
      namespace,
      name: 'api-server',
      status: 'Ready',
      labels: { app: 'api-server' },
    },
    {
      uid: uid('Secret', 'api-server-creds'),
      kind: 'Secret',
      namespace,
      name: 'api-server-creds',
      status: 'Ready',
      labels: { app: 'api-server' },
    },

    // === Database Stack ===
    {
      uid: uid('StatefulSet', 'postgres'),
      kind: 'StatefulSet',
      apiGroup: 'apps',
      namespace,
      name: 'postgres',
      status: 'Ready',
      labels: { app: 'postgres', tier: 'database' },
    },
    {
      uid: uid('Pod', 'postgres-0'),
      kind: 'Pod',
      namespace,
      name: 'postgres-0',
      status: 'Ready',
      labels: { app: 'postgres', 'statefulset.kubernetes.io/pod-name': 'postgres-0' },
    },
    {
      uid: uid('Service', 'postgres'),
      kind: 'Service',
      namespace,
      name: 'postgres',
      status: 'Ready',
      labels: { app: 'postgres' },
    },
    {
      uid: uid('PersistentVolumeClaim', 'postgres-data'),
      kind: 'PersistentVolumeClaim',
      namespace,
      name: 'postgres-data',
      status: 'Ready',
      labels: { app: 'postgres' },
    },

    // === Ingress ===
    {
      uid: uid('Ingress', 'main-ingress'),
      kind: 'Ingress',
      apiGroup: 'networking.k8s.io',
      namespace,
      name: 'main-ingress',
      status: 'Ready',
      labels: { app: 'ingress' },
    },

    // === Cluster-scoped resource (empty namespace) ===
    {
      uid: `cluster-clusterrole-${namespace}-reader`,
      kind: 'ClusterRole',
      apiGroup: 'rbac.authorization.k8s.io',
      namespace: '', // Cluster-scoped
      name: `${namespace}-reader`,
      status: 'Ready',
      labels: { 'rbac.authorization.k8s.io/aggregate-to-view': 'true' },
    },
  ];

  const edges: GraphEdge[] = [
    // Web App ownership chain
    { id: 'e1', source: uid('Deployment', 'web-app'), target: uid('ReplicaSet', 'web-app-7d9f8b6c5'), relationshipType: 'OWNS' },
    { id: 'e2', source: uid('ReplicaSet', 'web-app-7d9f8b6c5'), target: uid('Pod', 'web-app-7d9f8b6c5-abc12'), relationshipType: 'OWNS' },
    { id: 'e3', source: uid('ReplicaSet', 'web-app-7d9f8b6c5'), target: uid('Pod', 'web-app-7d9f8b6c5-def34'), relationshipType: 'OWNS' },
    
    // Web App Service selects pods
    { id: 'e4', source: uid('Service', 'web-app'), target: uid('Pod', 'web-app-7d9f8b6c5-abc12'), relationshipType: 'SELECTS' },
    { id: 'e5', source: uid('Service', 'web-app'), target: uid('Pod', 'web-app-7d9f8b6c5-def34'), relationshipType: 'SELECTS' },
    
    // Web App ConfigMap reference
    { id: 'e6', source: uid('Pod', 'web-app-7d9f8b6c5-abc12'), target: uid('ConfigMap', 'web-app-config'), relationshipType: 'REFERENCES' },
    { id: 'e7', source: uid('Pod', 'web-app-7d9f8b6c5-def34'), target: uid('ConfigMap', 'web-app-config'), relationshipType: 'REFERENCES' },

    // API Server ownership chain
    { id: 'e8', source: uid('Deployment', 'api-server'), target: uid('ReplicaSet', 'api-server-5c7d8e9f1'), relationshipType: 'OWNS' },
    { id: 'e9', source: uid('ReplicaSet', 'api-server-5c7d8e9f1'), target: uid('Pod', 'api-server-5c7d8e9f1-xyz99'), relationshipType: 'OWNS' },
    
    // API Service selects pods
    { id: 'e10', source: uid('Service', 'api-server'), target: uid('Pod', 'api-server-5c7d8e9f1-xyz99'), relationshipType: 'SELECTS' },
    
    // API Secret reference
    { id: 'e11', source: uid('Pod', 'api-server-5c7d8e9f1-xyz99'), target: uid('Secret', 'api-server-creds'), relationshipType: 'REFERENCES' },

    // Database ownership
    { id: 'e12', source: uid('StatefulSet', 'postgres'), target: uid('Pod', 'postgres-0'), relationshipType: 'OWNS' },
    { id: 'e13', source: uid('Service', 'postgres'), target: uid('Pod', 'postgres-0'), relationshipType: 'SELECTS' },
    { id: 'e14', source: uid('Pod', 'postgres-0'), target: uid('PersistentVolumeClaim', 'postgres-data'), relationshipType: 'REFERENCES' },

    // Ingress routes to services
    { id: 'e15', source: uid('Ingress', 'main-ingress'), target: uid('Service', 'web-app'), relationshipType: 'ROUTES_TO' },
    { id: 'e16', source: uid('Ingress', 'main-ingress'), target: uid('Service', 'api-server'), relationshipType: 'ROUTES_TO' },

    // Cross-service dependencies (web-app -> api-server, api-server -> postgres)
    { id: 'e17', source: uid('Pod', 'web-app-7d9f8b6c5-abc12'), target: uid('Service', 'api-server'), relationshipType: 'CONNECTS_TO' },
    { id: 'e18', source: uid('Pod', 'api-server-5c7d8e9f1-xyz99'), target: uid('Service', 'postgres'), relationshipType: 'CONNECTS_TO' },
  ];

  // Anomalies for the failing pod and degraded deployment
  const anomalies: Anomaly[] = [
    {
      node: {
        uid: uid('Pod', 'web-app-7d9f8b6c5-def34'),
        kind: 'Pod',
        namespace,
        name: 'web-app-7d9f8b6c5-def34',
      },
      category: 'state',
      type: 'CrashLoopBackOff',
      severity: 'high',
      timestamp: now,
      summary: 'Pod is in CrashLoopBackOff state with 5 restarts',
      details: {
        restartCount: 5,
        lastState: 'OOMKilled',
        exitCode: 137,
      },
    },
    {
      node: {
        uid: uid('Pod', 'web-app-7d9f8b6c5-def34'),
        kind: 'Pod',
        namespace,
        name: 'web-app-7d9f8b6c5-def34',
      },
      category: 'event',
      type: 'BackOff',
      severity: 'medium',
      timestamp: now,
      summary: 'Back-off restarting failed container',
      details: {
        reason: 'BackOff',
        message: 'Back-off 5m0s restarting failed container',
      },
    },
    {
      node: {
        uid: uid('Deployment', 'api-server'),
        kind: 'Deployment',
        namespace,
        name: 'api-server',
      },
      category: 'state',
      type: 'InsufficientReplicas',
      severity: 'medium',
      timestamp: now,
      summary: 'Deployment has fewer ready replicas than desired',
      details: {
        desiredReplicas: 3,
        readyReplicas: 1,
        availableReplicas: 1,
      },
    },
    {
      node: {
        uid: uid('ConfigMap', 'web-app-config'),
        kind: 'ConfigMap',
        namespace,
        name: 'web-app-config',
      },
      category: 'change',
      type: 'ConfigChange',
      severity: 'medium',
      timestamp: new Date(Date.now() - 10 * 60 * 1000).toISOString(), // 10 min ago
      summary: 'Configuration changed: memory limit reduced',
      details: {
        changedKeys: ['MEMORY_LIMIT'],
        previousValue: '512Mi',
        newValue: '128Mi',
      },
    },
  ];

  // Build causal paths - demonstrating root cause analysis
  // Path 1: ConfigMap change caused Pod crash (high confidence)
  const configMapNode: PathNode = {
    id: uid('ConfigMap', 'web-app-config'),
    resource: {
      uid: uid('ConfigMap', 'web-app-config'),
      kind: 'ConfigMap',
      namespace,
      name: 'web-app-config',
    },
    anomalies: [anomalies[3]], // ConfigChange anomaly
    primaryEvent: {
      eventId: 'evt-config-change-1',
      timestamp: new Date(Date.now() - 10 * 60 * 1000).toISOString(),
      eventType: 'UPDATE',
      configChanged: true,
      description: 'ConfigMap updated: MEMORY_LIMIT changed from 512Mi to 128Mi',
    },
  };

  const replicaSetNode: PathNode = {
    id: uid('ReplicaSet', 'web-app-7d9f8b6c5'),
    resource: {
      uid: uid('ReplicaSet', 'web-app-7d9f8b6c5'),
      kind: 'ReplicaSet',
      namespace,
      name: 'web-app-7d9f8b6c5',
    },
    anomalies: [],
  };

  const crashingPodNode: PathNode = {
    id: uid('Pod', 'web-app-7d9f8b6c5-def34'),
    resource: {
      uid: uid('Pod', 'web-app-7d9f8b6c5-def34'),
      kind: 'Pod',
      namespace,
      name: 'web-app-7d9f8b6c5-def34',
    },
    anomalies: [anomalies[0], anomalies[1]], // CrashLoopBackOff and BackOff
    primaryEvent: {
      eventId: 'evt-pod-crash-1',
      timestamp: now,
      eventType: 'UPDATE',
      statusChanged: true,
      description: 'Pod entered CrashLoopBackOff state',
    },
  };

  const causalPath1: CausalPath = {
    id: 'path-1-configmap-to-pod-crash',
    candidateRoot: configMapNode,
    firstAnomalyAt: new Date(Date.now() - 10 * 60 * 1000).toISOString(),
    steps: [
      {
        node: configMapNode,
        // No edge for root node
      },
      {
        node: replicaSetNode,
        edge: {
          id: 'edge-config-to-rs',
          relationshipType: 'REFERENCED_BY',
          edgeCategory: 'MATERIALIZATION',
          causalWeight: 0.0,
        },
      },
      {
        node: crashingPodNode,
        edge: {
          id: 'edge-rs-to-pod',
          relationshipType: 'OWNS',
          edgeCategory: 'CAUSE_INTRODUCING',
          causalWeight: 1.0,
        },
      },
    ],
    confidenceScore: 0.87,
    explanation: 'ConfigMap web-app-config was updated 10 minutes ago, reducing MEMORY_LIMIT from 512Mi to 128Mi. This change propagated to Pod web-app-7d9f8b6c5-def34 which subsequently entered CrashLoopBackOff due to OOMKilled events.',
    ranking: {
      temporalScore: 0.92,
      effectiveCausalDistance: 2,
      maxAnomalySeverity: 'high',
      severityScore: 0.85,
    },
  };

  // Path 2: Deployment scaling issue (medium confidence)
  const deploymentNode: PathNode = {
    id: uid('Deployment', 'api-server'),
    resource: {
      uid: uid('Deployment', 'api-server'),
      kind: 'Deployment',
      namespace,
      name: 'api-server',
    },
    anomalies: [anomalies[2]], // InsufficientReplicas
    primaryEvent: {
      eventId: 'evt-deploy-scale-1',
      timestamp: new Date(Date.now() - 5 * 60 * 1000).toISOString(),
      eventType: 'UPDATE',
      configChanged: true,
      description: 'Deployment replicas scaled from 3 to 1',
    },
  };

  const apiReplicaSetNode: PathNode = {
    id: uid('ReplicaSet', 'api-server-5c7d8e9f1'),
    resource: {
      uid: uid('ReplicaSet', 'api-server-5c7d8e9f1'),
      kind: 'ReplicaSet',
      namespace,
      name: 'api-server-5c7d8e9f1',
    },
    anomalies: [],
  };

  const causalPath2: CausalPath = {
    id: 'path-2-deployment-scaling',
    candidateRoot: deploymentNode,
    firstAnomalyAt: new Date(Date.now() - 5 * 60 * 1000).toISOString(),
    steps: [
      {
        node: deploymentNode,
      },
      {
        node: apiReplicaSetNode,
        edge: {
          id: 'edge-deploy-to-rs',
          relationshipType: 'OWNS',
          edgeCategory: 'CAUSE_INTRODUCING',
          causalWeight: 1.0,
        },
      },
    ],
    confidenceScore: 0.65,
    explanation: 'Deployment api-server was scaled down from 3 to 1 replica 5 minutes ago, causing an InsufficientReplicas warning. Service degradation may occur under load.',
    ranking: {
      temporalScore: 0.78,
      effectiveCausalDistance: 1,
      maxAnomalySeverity: 'medium',
      severityScore: 0.60,
    },
  };

  // Path 3: Potential resource contention (lower confidence)
  const webDeploymentNode: PathNode = {
    id: uid('Deployment', 'web-app'),
    resource: {
      uid: uid('Deployment', 'web-app'),
      kind: 'Deployment',
      namespace,
      name: 'web-app',
    },
    anomalies: [],
    primaryEvent: {
      eventId: 'evt-web-deploy-1',
      timestamp: new Date(Date.now() - 15 * 60 * 1000).toISOString(),
      eventType: 'UPDATE',
      configChanged: true,
      description: 'Deployment image updated to v2.1.0',
    },
  };

  const causalPath3: CausalPath = {
    id: 'path-3-image-update',
    candidateRoot: webDeploymentNode,
    firstAnomalyAt: new Date(Date.now() - 15 * 60 * 1000).toISOString(),
    steps: [
      {
        node: webDeploymentNode,
      },
      {
        node: replicaSetNode,
        edge: {
          id: 'edge-webdeploy-to-rs',
          relationshipType: 'OWNS',
          edgeCategory: 'CAUSE_INTRODUCING',
          causalWeight: 1.0,
        },
      },
      {
        node: crashingPodNode,
        edge: {
          id: 'edge-rs-to-crashpod',
          relationshipType: 'OWNS',
          edgeCategory: 'CAUSE_INTRODUCING',
          causalWeight: 1.0,
        },
      },
    ],
    confidenceScore: 0.42,
    explanation: 'Deployment web-app was updated to image v2.1.0 approximately 15 minutes ago. The new version may have introduced a memory leak causing OOMKilled events in the pod.',
    ranking: {
      temporalScore: 0.55,
      effectiveCausalDistance: 2,
      maxAnomalySeverity: 'high',
      severityScore: 0.85,
    },
  };

  const causalPaths: CausalPath[] = [causalPath1, causalPath2, causalPath3];

  return {
    graph: { nodes, edges },
    anomalies,
    causalPaths,
    metadata: {
      namespace,
      timestamp: timestampNanos,
      nodeCount: nodes.length,
      edgeCount: edges.length,
      queryExecutionMs: 42,
      hasMore: false,
    },
  };
}
