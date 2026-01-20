/**
 * Namespace Graph API Types
 * Matches backend schema from internal/analysis/namespace_graph/types.go
 */

import * as d3 from 'd3';
import { Anomaly, CausalPath } from './rootCause';

// Re-export for convenience
export type { Anomaly, CausalPath };

/**
 * API Request parameters for namespace graph
 */
export interface NamespaceGraphRequest {
  /** Required: Kubernetes namespace */
  namespace: string;
  /** Required: Point in time (Unix nanoseconds) */
  timestamp: number;
  /** Optional: Include anomaly detection results */
  includeAnomalies?: boolean;
  /** Optional: Include causal path analysis */
  includeCausalPaths?: boolean;
  /** Optional: Lookback duration string (e.g., "10m", "1h") */
  lookback?: string;
  /** Optional: Max relationship traversal depth (default 3) */
  maxDepth?: number;
  /** Optional: Pagination limit (default 100, max 500) */
  limit?: number;
  /** Optional: Pagination cursor */
  cursor?: string;
}

/**
 * API Response structure for namespace graph
 */
export interface NamespaceGraphResponse {
  graph: Graph;
  anomalies?: Anomaly[];
  causalPaths?: CausalPath[];
  metadata: GraphMetadata;
}

/**
 * Graph contains nodes and edges
 */
export interface Graph {
  nodes: GraphNode[];
  edges: GraphEdge[];
}

/**
 * Node represents a Kubernetes resource in the graph
 */
export interface GraphNode {
  uid: string;
  kind: string;
  apiGroup?: string;
  namespace: string; // Empty string for cluster-scoped resources
  name: string;
  status: string; // "Ready" | "Warning" | "Error" | "Terminating" | "Unknown"
  latestEvent?: ChangeEventInfo;
  labels?: Record<string, string>;
}

/**
 * Information about a resource change event
 */
export interface ChangeEventInfo {
  timestamp: number; // Unix nanoseconds
  eventType: string; // "ADDED" | "MODIFIED" | "DELETED"
  description?: string;
  status?: string; // "Ready" | "Warning" | "Error" | "Terminating" | "Unknown"
  errorMessage?: string; // Human-readable error description
  containerIssues?: string[]; // CrashLoopBackOff, ImagePullBackOff, OOMKilled
  impactScore?: number; // 0.0-1.0 severity score
  specChanges?: string; // Git-style unified diff of spec changes
}

/**
 * Edge represents a relationship between resources
 */
export interface GraphEdge {
  id: string;
  source: string; // Source node UID
  target: string; // Target node UID
  relationshipType: string; // "OWNS" | "SELECTS" | "REFERENCES" | etc.
}

/**
 * Response metadata and pagination info
 */
export interface GraphMetadata {
  namespace: string;
  timestamp: number; // Unix nanoseconds
  nodeCount: number;
  edgeCount: number;
  queryExecutionMs: number;
  hasMore: boolean;
  nextCursor?: string;
}

// ============================================================================
// D3 Simulation Types
// ============================================================================

/**
 * D3-compatible node type extending SimulationNodeDatum
 */
export interface D3GraphNode extends d3.SimulationNodeDatum {
  // Original properties from GraphNode
  uid: string;
  kind: string;
  apiGroup?: string;
  namespace: string;
  name: string;
  status: string;
  latestEvent?: ChangeEventInfo;
  labels?: Record<string, string>;
  
  // Computed properties for visualization
  anomalyCount: number;
  isClusterScoped: boolean;
  
  // D3 simulation adds these (optional since they're set during simulation)
  x?: number;
  y?: number;
  vx?: number;
  vy?: number;
  fx?: number | null;
  fy?: number | null;
}

/**
 * D3-compatible link type extending SimulationLinkDatum
 */
export interface D3GraphLink extends d3.SimulationLinkDatum<D3GraphNode> {
  id: string;
  relationshipType: string;
  // source and target are inherited from SimulationLinkDatum
  // They start as string UIDs but D3 replaces them with node references
}

/**
 * Status colors for node visualization
 */
export const NODE_STATUS_COLORS: Record<string, string> = {
  Ready: '#10b981',      // emerald-500
  Warning: '#f59e0b',    // amber-500
  Error: '#ef4444',      // red-500
  Terminating: '#6b7280', // gray-500
  Unknown: '#374151',    // gray-700
};

/**
 * Relationship type display names
 */
export const RELATIONSHIP_LABELS: Record<string, string> = {
  OWNS: 'Owns',
  SELECTS: 'Selects',
  REFERENCES: 'References',
  MANAGES: 'Manages',
  SCHEDULED_ON: 'Scheduled On',
  MEMBER_OF: 'Member Of',
};

/**
 * Convert API GraphNode to D3GraphNode
 */
export function toD3Node(node: GraphNode, anomalyCount: number = 0): D3GraphNode {
  return {
    ...node,
    anomalyCount,
    isClusterScoped: node.namespace === '',
  };
}

/**
 * Convert API GraphEdge to D3GraphLink
 */
export function toD3Link(edge: GraphEdge): D3GraphLink {
  return {
    id: edge.id,
    source: edge.source,
    target: edge.target,
    relationshipType: edge.relationshipType,
  };
}

/**
 * Transform API response to D3-compatible format
 */
export function transformToD3Graph(
  response: NamespaceGraphResponse
): { nodes: D3GraphNode[]; links: D3GraphLink[] } {
  // Count anomalies per node
  const anomalyCounts = new Map<string, number>();
  if (response.anomalies) {
    for (const anomaly of response.anomalies) {
      const uid = anomaly.node.uid;
      anomalyCounts.set(uid, (anomalyCounts.get(uid) || 0) + 1);
    }
  }

  const nodes = response.graph.nodes.map(node => 
    toD3Node(node, anomalyCounts.get(node.uid) || 0)
  );

  const links = response.graph.edges.map(toD3Link);

  return { nodes, links };
}

/**
 * Merge two namespace graph responses (for pagination)
 * Appends new nodes/edges/anomalies without duplicates
 */
export function mergeNamespaceGraphResponses(
  existing: NamespaceGraphResponse,
  incoming: NamespaceGraphResponse
): NamespaceGraphResponse {
  // Create sets of existing UIDs for deduplication
  const existingNodeUids = new Set(existing.graph.nodes.map(n => n.uid));
  const existingEdgeIds = new Set(existing.graph.edges.map(e => e.id));

  // Filter out duplicates from incoming data
  const newNodes = incoming.graph.nodes.filter(n => !existingNodeUids.has(n.uid));
  const newEdges = incoming.graph.edges.filter(e => !existingEdgeIds.has(e.id));

  // Merge anomalies (dedupe by composite key)
  const existingAnomalyKeys = new Set(
    (existing.anomalies || []).map(a => `${a.node.uid}:${a.category}:${a.type}`)
  );
  const newAnomalies = (incoming.anomalies || []).filter(
    a => !existingAnomalyKeys.has(`${a.node.uid}:${a.category}:${a.type}`)
  );

  // Merge causal paths (dedupe by ID)
  const existingPathIds = new Set((existing.causalPaths || []).map(p => p.id));
  const newPaths = (incoming.causalPaths || []).filter(p => !existingPathIds.has(p.id));

  return {
    graph: {
      nodes: [...existing.graph.nodes, ...newNodes],
      edges: [...existing.graph.edges, ...newEdges],
    },
    anomalies: [...(existing.anomalies || []), ...newAnomalies],
    causalPaths: [...(existing.causalPaths || []), ...newPaths],
    metadata: {
      ...incoming.metadata,
      nodeCount: existing.graph.nodes.length + newNodes.length,
      edgeCount: existing.graph.edges.length + newEdges.length,
      // Keep query time as cumulative
      queryExecutionMs: existing.metadata.queryExecutionMs + incoming.metadata.queryExecutionMs,
    },
  };
}
