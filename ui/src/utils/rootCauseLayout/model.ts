/**

 * Layout model for root cause analysis graph
 * Defines nodes, edges, ports, and their relationships
 */

import { RootCauseAnalysisV2, GraphNode, CausalGraph } from '../../types/rootCause';

export interface LayoutNode {
  id: string;
  kind: 'spine' | 'side';
  nodeRef: GraphNode;
  width: number;
  height: number;
}

export interface LayoutEdge {
  id: string;
  from: string;
  to: string;
  kind: 'spine' | 'attachment';
  relationshipType?: string;
}

export interface Port {
  side: 'top' | 'bottom' | 'left' | 'right';
  x: number;
  y: number;
}

export interface PositionedNode extends LayoutNode {
  x: number;
  y: number;
  ports: {
    top: Port;
    bottom: Port;
    left: Port;
    right: Port;
  };
}

export interface RoutedEdge extends LayoutEdge {
  points: Array<{ x: number; y: number }>;
  fromPort: Port;
  toPort: Port;
}

export interface LayoutResult {
  nodes: PositionedNode[];
  edges: RoutedEdge[];
  bounds: {
    x: number;
    y: number;
    width: number;
    height: number;
  };
}

// Card dimensions (matching RootCauseView)
export const CARD_WIDTH = 280;
export const CARD_HEIGHT = 180;
export const HEADER_HEIGHT = 60; // Height of card header where resource name is displayed

// Port inset from card edge
const PORT_INSET = 0; // Ports are on the card border

/**
 * Build layout model from root cause analysis
 */
export function buildLayoutModel(analysis: RootCauseAnalysisV2): {
  nodes: LayoutNode[];
  edges: LayoutEdge[];
} {
  if (!analysis?.incident?.graph || analysis.incident.graph.nodes.length === 0) {
    console.warn('[buildLayoutModel] No graph data in analysis');
    return { nodes: [], edges: [] };
  }

  const graph = analysis.incident.graph;
  const nodes: LayoutNode[] = [];
  const edges: LayoutEdge[] = [];

  console.log('[buildLayoutModel] Processing graph:', {
    nodeCount: graph.nodes.length,
    edgeCount: graph.edges.length,
    nodeTypes: graph.nodes.map(n => n.nodeType)
  });

  // Create layout nodes from graph nodes with dynamic heights
  graph.nodes.forEach((node) => {
    // Calculate dynamic height based on content
    const hasEvents = node.allEvents && node.allEvents.length > 0;
    const eventCount = node.allEvents?.length || 0;

    // Base height: header (60px) + padding
    let calculatedHeight = HEADER_HEIGHT + 10; // 70px minimum

    // Add height for event badges if present
    if (hasEvents) {
      // Event type badges: ~40px for main badges + ~25px for subcategories
      calculatedHeight += 65;
    }

    // Ensure minimum height for visual consistency
    const finalHeight = Math.max(calculatedHeight, 80);

    const layoutNode: LayoutNode = {
      id: node.id,
      kind: node.nodeType === 'SPINE' ? 'spine' : 'side',
      nodeRef: node,
      width: CARD_WIDTH,
      height: finalHeight,
    };
    nodes.push(layoutNode);
    console.log('[buildLayoutModel] Created node:', {
      id: layoutNode.id,
      kind: layoutNode.kind,
      nodeType: node.nodeType,
      resourceKind: node.resource.kind,
      hasEvents,
      eventCount,
      calculatedHeight: finalHeight
    });
  });

  // Create layout edges from graph edges
  graph.edges.forEach((edge) => {
    edges.push({
      id: edge.id,
      from: edge.from,
      to: edge.to,
      kind: edge.edgeType === 'SPINE' ? 'spine' : 'attachment',
      relationshipType: edge.relationshipType,
    });
  });

  console.log('[buildLayoutModel] Created:', {
    nodes: nodes.length,
    spineNodes: nodes.filter(n => n.kind === 'spine').length,
    sideNodes: nodes.filter(n => n.kind === 'side').length,
    edges: edges.length,
    spineEdges: edges.filter(e => e.kind === 'spine').length,
    attachmentEdges: edges.filter(e => e.kind === 'attachment').length
  });

  return { nodes, edges };
}

/**
 * Calculate port positions for a positioned node
 * Ports are positioned at the header center (vertically) for better visual alignment
 */
export function calculatePorts(node: PositionedNode): PositionedNode['ports'] {
  const halfWidth = node.width / 2;
  const headerCenterY = HEADER_HEIGHT / 2; // Center of header vertically (30px from top)

  return {
    top: {
      side: 'top',
      x: node.x + halfWidth,
      y: node.y + headerCenterY, // Connect at header center
    },
    bottom: {
      side: 'bottom',
      x: node.x + halfWidth,
      y: node.y + node.height,
    },
    left: {
      side: 'left',
      x: node.x,
      y: node.y + headerCenterY, // Connect at header center
    },
    right: {
      side: 'right',
      x: node.x + node.width,
      y: node.y + headerCenterY, // Connect at header center
    },
  };
}

/**
 * Select appropriate ports for an edge based on relative positions
 */
export function selectPorts(
  fromNode: PositionedNode,
  toNode: PositionedNode
): { fromPort: Port; toPort: Port } {
  const dx = toNode.x - fromNode.x;
  const dy = toNode.y - fromNode.y;

  // Determine primary direction
  if (Math.abs(dy) > Math.abs(dx)) {
    // Vertical connection
    if (dy > 0) {
      // Target is below: use bottom → top
      return {
        fromPort: fromNode.ports.bottom,
        toPort: toNode.ports.top,
      };
    } else {
      // Target is above: use top → bottom
      return {
        fromPort: fromNode.ports.top,
        toPort: toNode.ports.bottom,
      };
    }
  } else {
    // Horizontal connection
    if (dx > 0) {
      // Target is to the right: use right → left
      return {
        fromPort: fromNode.ports.right,
        toPort: toNode.ports.left,
      };
    } else {
      // Target is to the left: use left → right
      return {
        fromPort: fromNode.ports.left,
        toPort: toNode.ports.right,
      };
    }
  }
}

