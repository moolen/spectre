/**
 * Root Cause Layout Engine
 * Public API for computing compact, space-efficient graph layouts
 */

import { RootCauseAnalysisV2, LayoutResult } from './model';
import { buildLayoutModel } from './model';
import { placeNodes, calculateBounds } from './place';
import { routeEdges } from './route';
import { bundleEdges } from './bundle';
import { placeNodesForce } from './force';
import { routeEdgesDirect } from './forceRoute';

export interface LayoutOptions {
  engine?: 'force' | 'grid'; // Layout engine to use (default: 'force')
  enableBundling?: boolean; // Enable edge bundling (default: true)
  gridCellSize?: number; // Grid cell size for routing (default: 25, only used with 'grid' engine)
}

/**
 * Compute root cause layout
 * Main entry point for the layout engine
 */
export function computeRootCauseLayout(
  analysis: RootCauseAnalysisV2,
  options: LayoutOptions = {}
): LayoutResult {
  const { engine = 'force', enableBundling = true } = options;

  // Build layout model
  const { nodes, edges } = buildLayoutModel(analysis);

  if (nodes.length === 0) {
    return {
      nodes: [],
      edges: [],
      bounds: { x: 0, y: 0, width: 0, height: 0 },
    };
  }

  // Place nodes using selected engine
  let positionedNodes;
  if (engine === 'force') {
    positionedNodes = placeNodesForce(nodes, edges);
  } else {
    positionedNodes = placeNodes(nodes, edges);
  }

  // Calculate bounds
  const rawBounds = calculateBounds(positionedNodes);
  // Padding ensures the container always includes all nodes (and some breathing room)
  const boundsPadding = engine === 'force' ? 240 : 80;
  const bounds = {
    x: rawBounds.x - boundsPadding,
    y: rawBounds.y - boundsPadding,
    width: rawBounds.width + boundsPadding * 2,
    height: rawBounds.height + boundsPadding * 2,
  };

  // Route edges (use direct routing for force mode, A* for grid mode)
  let routedEdges;
  if (engine === 'force') {
    routedEdges = routeEdgesDirect(edges, positionedNodes);
  } else {
    routedEdges = routeEdges(edges, positionedNodes, bounds);
  }

  // Apply bundling if enabled
  if (enableBundling) {
    routedEdges = bundleEdges(routedEdges);
  }

  return {
    nodes: positionedNodes,
    edges: routedEdges,
    bounds,
  };
}

// Re-export types for convenience
export type { LayoutResult, PositionedNode, RoutedEdge } from './model';


