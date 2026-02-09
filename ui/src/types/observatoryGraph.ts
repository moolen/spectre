/**
 * Observatory Graph API Types
 * Matches backend schema from internal/analysis/observatory_graph/types.go
 */

import * as d3 from 'd3';

/**
 * API Request parameters for observatory graph
 */
export interface ObservatoryGraphRequest {
  /** Optional: Integration name to filter */
  integration?: string;
  /** Optional: Kubernetes namespace to filter SignalAnchors by workload */
  namespace?: string;
  /** Optional: Workload name to filter SignalAnchors */
  workload?: string;
  /** Optional: Include SignalBaseline nodes (default false) */
  includeBaselines?: boolean;
  /** Optional: Maximum number of SignalAnchor nodes (default 100, max 500) */
  limit?: number;
}

/**
 * API Response structure for observatory graph
 */
export interface ObservatoryGraphResponse {
  graph: ObservatoryGraph;
  metadata: ObservatoryGraphMetadata;
}

/**
 * Graph contains nodes and edges
 */
export interface ObservatoryGraph {
  nodes: ObservatoryNode[];
  edges: ObservatoryEdge[];
}

/**
 * Node represents a node in the observatory graph
 */
export interface ObservatoryNode {
  id: string;
  type: ObservatoryNodeType;
  label: string;
  properties?: Record<string, any>;
}

/**
 * Node types for observatory visualization
 */
export type ObservatoryNodeType =
  | 'SignalAnchor'
  | 'SignalBaseline'
  | 'Alert'
  | 'Dashboard'
  | 'Panel'
  | 'Query'
  | 'Metric'
  | 'Service'
  | 'Workload';

/**
 * Edge represents an edge in the observatory graph
 */
export interface ObservatoryEdge {
  id: string;
  source: string;
  target: string;
  relationshipType: ObservatoryEdgeType;
  properties?: Record<string, any>;
}

/**
 * Edge types for observatory visualization
 */
export type ObservatoryEdgeType =
  | 'MONITORS_WORKLOAD'
  | 'CORRELATES_WITH'
  | 'EXTRACTED_FROM'
  | 'HAS_BASELINE'
  | 'CONTAINS'
  | 'HAS'
  | 'USES'
  | 'TRACKS'
  | 'MONITORS';

/**
 * Response metadata
 */
export interface ObservatoryGraphMetadata {
  nodeCount: number;
  edgeCount: number;
  queryExecutionMs: number;
}

// ============================================================================
// D3 Simulation Types
// ============================================================================

/**
 * D3-compatible node type extending SimulationNodeDatum
 */
export interface D3ObservatoryNode extends d3.SimulationNodeDatum {
  // Original properties from ObservatoryNode
  id: string;
  type: ObservatoryNodeType;
  label: string;
  properties?: Record<string, any>;

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
export interface D3ObservatoryLink extends d3.SimulationLinkDatum<D3ObservatoryNode> {
  id: string;
  relationshipType: ObservatoryEdgeType;
  // source and target are inherited from SimulationLinkDatum
  // They start as string IDs but D3 replaces them with node references
}

/**
 * Node type colors for visualization
 */
export const NODE_TYPE_COLORS: Record<ObservatoryNodeType, string> = {
  SignalAnchor: '#a855f7',    // purple-500 - main observatory entity
  SignalBaseline: '#8b5cf6',  // violet-500
  Alert: '#ef4444',           // red-500 - alerts are important
  Dashboard: '#3b82f6',       // blue-500
  Panel: '#60a5fa',           // blue-400
  Query: '#06b6d4',           // cyan-500
  Metric: '#10b981',          // emerald-500
  Service: '#f59e0b',         // amber-500
  Workload: '#22c55e',        // green-500
};

/**
 * Node type icons (emoji for quick identification)
 */
export const NODE_TYPE_ICONS: Record<ObservatoryNodeType, string> = {
  SignalAnchor: '📡',
  SignalBaseline: '📊',
  Alert: '🚨',
  Dashboard: '📋',
  Panel: '📈',
  Query: '🔍',
  Metric: '📉',
  Service: '⚙️',
  Workload: '🔧',
};

/**
 * Edge type colors for visualization
 */
export const EDGE_TYPE_COLORS: Record<ObservatoryEdgeType, string> = {
  MONITORS_WORKLOAD: '#22c55e', // green
  CORRELATES_WITH: '#ef4444',   // red
  EXTRACTED_FROM: '#3b82f6',    // blue
  HAS_BASELINE: '#8b5cf6',      // violet
  CONTAINS: '#6b7280',          // gray
  HAS: '#6b7280',               // gray
  USES: '#06b6d4',              // cyan
  TRACKS: '#f59e0b',            // amber
  MONITORS: '#ef4444',          // red
};

/**
 * Relationship type display names
 */
export const EDGE_TYPE_LABELS: Record<ObservatoryEdgeType, string> = {
  MONITORS_WORKLOAD: 'Monitors',
  CORRELATES_WITH: 'Correlates With',
  EXTRACTED_FROM: 'Extracted From',
  HAS_BASELINE: 'Has Baseline',
  CONTAINS: 'Contains',
  HAS: 'Has',
  USES: 'Uses',
  TRACKS: 'Tracks',
  MONITORS: 'Monitors',
};

/**
 * Convert API ObservatoryNode to D3ObservatoryNode
 */
export function toD3Node(node: ObservatoryNode): D3ObservatoryNode {
  return {
    ...node,
  };
}

/**
 * Convert API ObservatoryEdge to D3ObservatoryLink
 */
export function toD3Link(edge: ObservatoryEdge): D3ObservatoryLink {
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
  response: ObservatoryGraphResponse
): { nodes: D3ObservatoryNode[]; links: D3ObservatoryLink[] } {
  const nodes = response.graph.nodes.map(toD3Node);
  const links = response.graph.edges.map(toD3Link);
  return { nodes, links };
}
