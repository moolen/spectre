/**
 * Graph layout utilities for RootCauseView
 * Uses dagre for hierarchical DAG layout
 */

import dagre from 'dagre';
import { RootCauseAnalysisV2, CausalStep, RelatedResource } from '../types/rootCause';

export interface LayoutNode {
  id: string;
  width: number;
  height: number;
  kind: 'main' | 'related';
  stepRef?: CausalStep;
  relatedRef?: RelatedResource;
  parentId?: string; // For related resources, the parent main node ID
}

export interface LayoutEdge {
  from: string;
  to: string;
  kind: 'main' | 'related';
  relationshipType?: string;
}

export interface LayoutResult {
  nodes: Array<LayoutNode & { x: number; y: number }>;
  edges: Array<LayoutEdge & { points: Array<{ x: number; y: number }> }>;
  width: number;
  height: number;
}

// Card dimensions (matching RootCauseView)
const CARD_WIDTH = 280;
const CARD_HEIGHT = 180;

// Compact spacing parameters
const RANK_SEP = 100; // Vertical spacing between layers (causal steps) - reduced for compactness
const NODE_SEP = 50;  // Horizontal spacing between nodes in same layer - reduced for compactness
const EDGE_SEP = 15;  // Edge separation - reduced for compactness

// Related resource edge parameters (keep them close to parent)
const RELATED_MIN_LEN = 1; // Minimum rank distance for related resources (keep at 1 for tight packing)
const RELATED_WEIGHT = 0.05; // Lower weight = shorter preferred distance (reduced for tighter packing)

/**
 * Build a normalized graph model from root cause analysis
 */
export function buildLayoutGraph(analysis: RootCauseAnalysisV2): {
  nodes: LayoutNode[];
  edges: LayoutEdge[];
} {
  if (!analysis?.incident?.causalChain || analysis.incident.causalChain.length === 0) {
    return { nodes: [], edges: [] };
  }

  const steps = [...analysis.incident.causalChain].sort((a, b) => a.stepNumber - b.stepNumber);
  const nodes: LayoutNode[] = [];
  const edges: LayoutEdge[] = [];

  // Create main causal step nodes
  steps.forEach((step, idx) => {
    const mainNodeId = `main-${step.resource.uid}`;
    nodes.push({
      id: mainNodeId,
      width: CARD_WIDTH,
      height: CARD_HEIGHT,
      kind: 'main',
      stepRef: step,
    });

    // Create edge to next step in causal chain
    if (idx < steps.length - 1) {
      const nextStep = steps[idx + 1];
      const nextNodeId = `main-${nextStep.resource.uid}`;
      edges.push({
        from: mainNodeId,
        to: nextNodeId,
        kind: 'main',
        relationshipType: step.relationshipType,
      });
    }

    // Create related resource nodes and edges
    if (step.relatedResources && step.relatedResources.length > 0) {
      // Sort related resources for stable ordering
      const sortedRelated = [...step.relatedResources].sort((a, b) => {
        // Sort by relationship type, then resource name
        const typeCompare = (a.relationshipType || '').localeCompare(b.relationshipType || '');
        if (typeCompare !== 0) return typeCompare;
        return (a.resource.name || '').localeCompare(b.resource.name || '');
      });

      sortedRelated.forEach((related) => {
        const relatedNodeId = `related-${step.resource.uid}-${related.resource.uid}`;
        nodes.push({
          id: relatedNodeId,
          width: CARD_WIDTH,
          height: CARD_HEIGHT,
          kind: 'related',
          stepRef: step,
          relatedRef: related,
          parentId: mainNodeId,
        });

        // Create edge from parent to related resource
        edges.push({
          from: mainNodeId,
          to: relatedNodeId,
          kind: 'related',
          relationshipType: related.relationshipType,
        });
      });
    }
  });

  return { nodes, edges };
}

/**
 * Compute layout using dagre
 */
export function computeLayout(
  nodes: LayoutNode[],
  edges: LayoutEdge[]
): LayoutResult {
  if (nodes.length === 0) {
    return { nodes: [], edges: [], width: 0, height: 0 };
  }

  // Create dagre graph
  const g = new dagre.graphlib.Graph();
  g.setDefaultEdgeLabel(() => ({}));
  g.setGraph({
    rankdir: 'TB', // Top to bottom
    ranksep: RANK_SEP,
    nodesep: NODE_SEP,
    edgesep: EDGE_SEP,
    ranker: 'network-simplex', // Good for DAGs
  });

  // Add nodes
  nodes.forEach((node) => {
    g.setNode(node.id, {
      width: node.width,
      height: node.height,
    });
  });

  // Add edges with weights for related resources
  edges.forEach((edge) => {
    const edgeConfig: any = {};

    if (edge.kind === 'related') {
      // Keep related resources close to their parent
      edgeConfig.minlen = RELATED_MIN_LEN;
      edgeConfig.weight = RELATED_WEIGHT;
    } else {
      // Main causal edges get normal weight
      edgeConfig.weight = 1.0;
    }

    g.setEdge(edge.from, edge.to, edgeConfig);
  });

  // Run layout
  try {
    dagre.layout(g);
  } catch (error) {
    console.error('[graphLayout] Dagre layout failed:', error);
    // Fallback: simple layered layout
    return computeFallbackLayout(nodes, edges);
  }

  // Extract positioned nodes
  const positionedNodes = nodes.map((node) => {
    const dagreNode = g.node(node.id);
    if (!dagreNode) {
      console.warn(`[graphLayout] Missing dagre node for ${node.id}`);
      return { ...node, x: 0, y: 0 };
    }

    // Dagre gives center coordinates, convert to top-left for absolute positioning
    return {
      ...node,
      x: dagreNode.x - node.width / 2,
      y: dagreNode.y - node.height / 2,
    };
  });

  // Extract edge routes
  // Note: dagre edge points are in the same coordinate system as dagre node centers
  // Since we converted nodes from center to top-left, edge points are already correct
  // (they connect node centers, which is what we want)
  const routedEdges = edges.map((edge) => {
    const dagreEdge = g.edge(edge.from, edge.to);
    if (!dagreEdge || !dagreEdge.points) {
      // Fallback: straight line connecting node centers
      const fromNode = positionedNodes.find((n) => n.id === edge.from);
      const toNode = positionedNodes.find((n) => n.id === edge.to);
      if (fromNode && toNode) {
        return {
          ...edge,
          points: [
            { x: fromNode.x + fromNode.width / 2, y: fromNode.y + fromNode.height / 2 },
            { x: toNode.x + toNode.width / 2, y: toNode.y + toNode.height / 2 },
          ],
        };
      }
      return { ...edge, points: [] };
    }

    // Dagre points are in the coordinate system where nodes are centered
    // These points already connect node centers correctly, so we can use them as-is
    return {
      ...edge,
      points: dagreEdge.points.map((p: any) => ({ x: p.x, y: p.y })),
    };
  });

  // Calculate bounding box
  let minX = Infinity;
  let minY = Infinity;
  let maxX = -Infinity;
  let maxY = -Infinity;

  positionedNodes.forEach((node) => {
    minX = Math.min(minX, node.x);
    minY = Math.min(minY, node.y);
    maxX = Math.max(maxX, node.x + node.width);
    maxY = Math.max(maxY, node.y + node.height);
  });

  const width = maxX - minX;
  const height = maxY - minY;

  return {
    nodes: positionedNodes,
    edges: routedEdges,
    width,
    height,
  };
}

/**
 * Fallback layout if dagre fails (simple layered)
 */
function computeFallbackLayout(
  nodes: LayoutNode[],
  edges: LayoutEdge[]
): LayoutResult {
  console.warn('[graphLayout] Using fallback layout');

  const mainNodes = nodes.filter((n) => n.kind === 'main');
  const relatedNodes = nodes.filter((n) => n.kind === 'related');

  // Group related nodes by parent
  const relatedByParent = new Map<string, LayoutNode[]>();
  relatedNodes.forEach((node) => {
    if (node.parentId) {
      if (!relatedByParent.has(node.parentId)) {
        relatedByParent.set(node.parentId, []);
      }
      relatedByParent.get(node.parentId)!.push(node);
    }
  });

  // Simple top-to-bottom layout for main nodes
  const positionedNodes: Array<LayoutNode & { x: number; y: number }> = [];
  let currentY = 50;
  const centerX = 400; // Approximate center

  mainNodes.forEach((node, idx) => {
    const x = centerX;
    const y = currentY;

    positionedNodes.push({ ...node, x, y });

    // Add related nodes below parent
    const related = relatedByParent.get(node.id) || [];
    let relatedY = y + CARD_HEIGHT + 40;
    related.forEach((relNode) => {
      const relX = x + CARD_WIDTH + 40;
      positionedNodes.push({ ...relNode, x: relX, y: relatedY });
      relatedY += CARD_HEIGHT + 20;
    });

    currentY += CARD_HEIGHT + RANK_SEP;
  });

  // Simple edge routes (straight lines)
  const routedEdges = edges.map((edge) => {
    const fromNode = positionedNodes.find((n) => n.id === edge.from);
    const toNode = positionedNodes.find((n) => n.id === edge.to);
    if (fromNode && toNode) {
      return {
        ...edge,
        points: [
          { x: fromNode.x + fromNode.width / 2, y: fromNode.y + fromNode.height / 2 },
          { x: toNode.x + toNode.width / 2, y: toNode.y + toNode.height / 2 },
        ],
      };
    }
    return { ...edge, points: [] };
  });

  // Calculate bounding box
  let minX = Infinity;
  let minY = Infinity;
  let maxX = -Infinity;
  let maxY = -Infinity;

  positionedNodes.forEach((node) => {
    minX = Math.min(minX, node.x);
    minY = Math.min(minY, node.y);
    maxX = Math.max(maxX, node.x + node.width);
    maxY = Math.max(maxY, node.y + node.height);
  });

  return {
    nodes: positionedNodes,
    edges: routedEdges,
    width: maxX - minX,
    height: maxY - minY,
  };
}

/**
 * Convert layout result to smooth SVG path
 * Dagre provides polyline routes, we convert them to smooth curves
 */
export function edgePointsToPath(points: Array<{ x: number; y: number }>): string {
  if (points.length < 2) {
    return '';
  }

  if (points.length === 2) {
    // Simple straight line
    return `M ${points[0].x} ${points[0].y} L ${points[1].x} ${points[1].y}`;
  }

  // For multi-point paths, use a polyline with smooth corners
  // Dagre already provides good routing, so we mostly follow its points
  let path = `M ${points[0].x} ${points[0].y}`;

  // For paths with more than 2 points, use line segments with slight smoothing at corners
  for (let i = 1; i < points.length; i++) {
    const current = points[i];
    const prev = points[i - 1];

    if (i === points.length - 1) {
      // Last point - direct line
      path += ` L ${current.x} ${current.y}`;
    } else {
      // Intermediate points - use line segments (dagre handles routing well)
      path += ` L ${current.x} ${current.y}`;
    }
  }

  return path;
}

