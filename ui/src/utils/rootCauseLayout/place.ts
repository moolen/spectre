/**
 * Improved graph layout algorithm with edge crossing minimization
 * Uses topological layering and assigns rows to minimize overlaps
 */

import { LayoutNode, LayoutEdge, PositionedNode, CARD_WIDTH, CARD_HEIGHT, HEADER_HEIGHT, calculatePorts } from './model';

// Spacing constants - optimized to minimize edge overlaps
const SPINE_HORIZONTAL_SPACING = 200; // Horizontal spacing between spine nodes
const SPINE_VERTICAL_SPACING = 150; // Vertical spacing between rows (grid spacing)
const SIDE_NODE_SPACING = 80; // Spacing between side nodes
const SIDE_REGION_OFFSET = 180; // Vertical offset from spine for side regions

/**
 * Build adjacency lists for the graph
 */
function buildGraph(spineNodes: LayoutNode[], edges: LayoutEdge[]): {
  outgoing: Map<string, string[]>,
  incoming: Map<string, string[]>
} {
  const outgoing = new Map<string, string[]>();
  const incoming = new Map<string, string[]>();

  spineNodes.forEach(node => {
    outgoing.set(node.id, []);
    incoming.set(node.id, []);
  });

  edges.filter(e => e.kind === 'spine').forEach(edge => {
    outgoing.get(edge.from)?.push(edge.to);
    incoming.get(edge.to)?.push(edge.from);
  });

  return { outgoing, incoming };
}

/**
 * Assign layers using longest path from root nodes
 */
function assignLayers(spineNodes: LayoutNode[], edges: LayoutEdge[]): Map<string, number> {
  const { outgoing, incoming } = buildGraph(spineNodes, edges);
  const layers = new Map<string, number>();

  // Find root nodes (nodes with no incoming edges)
  const roots = spineNodes.filter(node =>
    (incoming.get(node.id)?.length || 0) === 0
  );

  // If no roots found (cycle or all nodes have incoming edges), use lowest step number
  if (roots.length === 0) {
    const firstNode = spineNodes.sort((a, b) => (a.nodeRef.stepNumber || 0) - (b.nodeRef.stepNumber || 0))[0];
    roots.push(firstNode);
  }

  // BFS to assign layers based on longest path
  const visited = new Set<string>();
  const queue: Array<{ nodeId: string, layer: number }> = [];

  roots.forEach(root => {
    layers.set(root.id, 0);
    queue.push({ nodeId: root.id, layer: 0 });
  });

  while (queue.length > 0) {
    const { nodeId, layer } = queue.shift()!;

    if (visited.has(nodeId)) continue;
    visited.add(nodeId);

    const children = outgoing.get(nodeId) || [];
    children.forEach(childId => {
      const currentLayer = layers.get(childId) ?? -1;
      const newLayer = layer + 1;

      if (newLayer > currentLayer) {
        layers.set(childId, newLayer);
      }

      if (!visited.has(childId)) {
        queue.push({ nodeId: childId, layer: newLayer });
      }
    });
  }

  // Assign layers to any unvisited nodes (disconnected components)
  spineNodes.forEach(node => {
    if (!layers.has(node.id)) {
      layers.set(node.id, 0);
    }
  });

  return layers;
}

/**
 * Assign rows within each column to minimize edge crossings and overlaps
 */
function assignRows(
  nodesByLayer: Map<number, LayoutNode[]>,
  edges: LayoutEdge[]
): Map<string, number> {
  const rows = new Map<string, number>();
  const { outgoing, incoming } = buildGraph(
    Array.from(nodesByLayer.values()).flat(),
    edges
  );

  // For each layer, assign rows to minimize crossings
  const sortedLayers = Array.from(nodesByLayer.keys()).sort((a, b) => a - b);

  sortedLayers.forEach(layer => {
    const nodesInLayer = nodesByLayer.get(layer) || [];

    if (layer === 0) {
      // First layer: assign rows based on step number
      nodesInLayer.sort((a, b) => (a.nodeRef.stepNumber || 0) - (b.nodeRef.stepNumber || 0));
      nodesInLayer.forEach((node, idx) => {
        rows.set(node.id, idx);
      });
    } else {
      // Sort by median position of parents in previous layer
      const nodeScores = nodesInLayer.map(node => {
        const parents = incoming.get(node.id) || [];
        if (parents.length === 0) {
          return { node, score: 0 };
        }

        const parentRows = parents
          .map(p => rows.get(p))
          .filter(r => r !== undefined) as number[];

        if (parentRows.length === 0) {
          return { node, score: 0 };
        }

        // Use median of parent rows
        parentRows.sort((a, b) => a - b);
        const median = parentRows[Math.floor(parentRows.length / 2)];
        return { node, score: median };
      });

      nodeScores.sort((a, b) => a.score - b.score);
      nodeScores.forEach((item, idx) => {
        rows.set(item.node.id, idx);
      });
    }
  });

  // Post-process: adjust positions to minimize edge overlaps with nodes in between
  optimizeRowsToAvoidEdgeOverlaps(rows, nodesByLayer, edges, incoming, outgoing);

  return rows;
}

/**
 * Optimize row assignments to prevent edges from passing behind intermediate nodes
 */
/**
 * Optimize spine node rows to avoid overlapping with edges from side nodes
 */
function optimizeForSideNodeEdges(
  rows: Map<string, number>,
  nodesByLayer: Map<number, LayoutNode[]>,
  edges: LayoutEdge[],
  sideNodes: LayoutNode[]
): void {
  // Find edges connecting side nodes to spine nodes
  // Note: attachment edges go FROM spine TO side
  const sideNodeIds = new Set(sideNodes.map(n => n.id));
  const attachmentEdges = edges.filter(e =>
    e.kind === 'attachment' && sideNodeIds.has(e.to)
  );

  if (attachmentEdges.length === 0) return;

  // Strategy: Move spine nodes WITH attachments down to create vertical clearance
  // for edges from side nodes at the top to connect without overlapping

  // Identify spine nodes that have side nodes attached
  const spineNodesWithAttachments = new Set(attachmentEdges.map(e => e.from));

  // Move nodes with attachments to a higher row number (lower vertical position)
  // This creates space above them for edges from side nodes
  Array.from(rows.keys()).forEach(nodeId => {
    if (spineNodesWithAttachments.has(nodeId)) {
      const currentRow = rows.get(nodeId) || 0;
      rows.set(nodeId, currentRow + 1); // Move down one row
    }
  });
}

function optimizeRowsToAvoidEdgeOverlaps(
  rows: Map<string, number>,
  nodesByLayer: Map<number, LayoutNode[]>,
  edges: LayoutEdge[],
  incoming: Map<string, string[]>,
  outgoing: Map<string, string[]>
): void {
  const sortedLayers = Array.from(nodesByLayer.keys()).sort((a, b) => a - b);

  // Build layer map for quick lookup
  const nodeToLayer = new Map<string, number>();
  sortedLayers.forEach(layer => {
    const nodesInLayer = nodesByLayer.get(layer) || [];
    nodesInLayer.forEach(node => nodeToLayer.set(node.id, layer));
  });

  // Iterate multiple times to propagate adjustments
  for (let iteration = 0; iteration < 3; iteration++) {
    // For each intermediate layer, check if nodes need to be moved
    for (let layer = 1; layer < sortedLayers.length; layer++) {
      const currentLayer = sortedLayers[layer];
      const intermediateNodes = nodesByLayer.get(currentLayer) || [];

      // For each node in this layer, check if edges pass through its position
      intermediateNodes.forEach(node => {
        const nodeRow = rows.get(node.id);
        if (nodeRow === undefined) return;

        // Check all edges that span this layer
        edges.filter(e => e.kind === 'spine').forEach(edge => {
          const fromLayer = nodeToLayer.get(edge.from);
          const toLayer = nodeToLayer.get(edge.to);
          const fromRow = rows.get(edge.from);
          const toRow = rows.get(edge.to);

          if (fromLayer === undefined || toLayer === undefined ||
              fromRow === undefined || toRow === undefined) {
            return;
          }

          // Does this edge span through the current layer?
          if (fromLayer < currentLayer && toLayer > currentLayer) {
            // Check if the node is in the path of the edge
            const minRow = Math.min(fromRow, toRow);
            const maxRow = Math.max(fromRow, toRow);

            // If node is blocking the edge path
            if (nodeRow >= minRow - 0.5 && nodeRow <= maxRow + 0.5) {
              // Try to move this node out of the way
              const nodesInCurrentLayer = intermediateNodes.length;

              // Prefer moving node to align with its own connections
              const nodeConnections = [
                ...(incoming.get(node.id) || []),
                ...(outgoing.get(node.id) || [])
              ];

              if (nodeConnections.length > 0) {
                // Calculate median position of connected nodes
                const connectedRows = nodeConnections
                  .map(id => rows.get(id))
                  .filter(r => r !== undefined) as number[];

                if (connectedRows.length > 0) {
                  connectedRows.sort((a, b) => a - b);
                  const medianRow = connectedRows[Math.floor(connectedRows.length / 2)];

                  // Move toward median if it would clear the edge path
                  if (medianRow < minRow - 0.5 || medianRow > maxRow + 0.5) {
                    rows.set(node.id, medianRow);
                  } else {
                    // Move to the less crowded side
                    if (minRow > 0) {
                      rows.set(node.id, minRow - 1);
                    } else if (maxRow < nodesInCurrentLayer - 1) {
                      rows.set(node.id, maxRow + 1);
                    }
                  }
                }
              }
            }
          }
        });
      });
    }
  }

  console.log('[optimizeForSideNodeEdges] Final rows after optimization:', Array.from(rows.entries()));
}

/**
 * Place nodes in compact 2D layout with improved edge crossing minimization
 */
export function placeNodes(
  nodes: LayoutNode[],
  edges: LayoutEdge[]
): PositionedNode[] {
  if (nodes.length === 0) {
    console.warn('[placeNodes] No nodes to place');
    return [];
  }

  // Separate spine and side nodes
  const spineNodes = nodes.filter((n) => n.kind === 'spine');
  const sideNodes = nodes.filter((n) => n.kind === 'side');

  console.log('[placeNodes] Processing:', {
    total: nodes.length,
    spine: spineNodes.length,
    side: sideNodes.length
  });

  // Assign layers (horizontal position) based on graph structure
  const layers = assignLayers(spineNodes, edges);

  // Group nodes by layer
  const nodesByLayer = new Map<number, LayoutNode[]>();
  spineNodes.forEach(node => {
    const layer = layers.get(node.id) ?? 0;
    if (!nodesByLayer.has(layer)) {
      nodesByLayer.set(layer, []);
    }
    nodesByLayer.get(layer)!.push(node);
  });

  // Assign rows (vertical position) to minimize crossings
  const rows = assignRows(nodesByLayer, edges);

  // Optimize rows to account for side node connections
  optimizeForSideNodeEdges(rows, nodesByLayer, edges, sideNodes);

  // Calculate positions
  const positionedSpine: PositionedNode[] = [];
  const startX = 50;
  const startY = 200;

  spineNodes.forEach(node => {
    const layer = layers.get(node.id) ?? 0;
    const row = rows.get(node.id) ?? 0;

    const x = startX + layer * (CARD_WIDTH + SPINE_HORIZONTAL_SPACING);
    const y = startY + row * SPINE_VERTICAL_SPACING;

    const positioned: PositionedNode = {
      ...node,
      x,
      y,
      ports: calculatePorts({ ...node, x, y } as PositionedNode),
    };

    positionedSpine.push(positioned);
  });

  // Group side nodes by parent (use edges to find parent-child relationships)
  const sideNodesByParent = new Map<string, LayoutNode[]>();

  // Build parent mapping from attachment edges
  const attachmentEdges = edges.filter(e => e.kind === 'attachment');
  console.log('[placeNodes] Found', attachmentEdges.length, 'attachment edges');

  attachmentEdges.forEach((edge) => {
    const sideNode = sideNodes.find(n => n.id === edge.to);
    if (sideNode) {
      if (!sideNodesByParent.has(edge.from)) {
        sideNodesByParent.set(edge.from, []);
      }
      sideNodesByParent.get(edge.from)!.push(sideNode);
      console.log('[placeNodes] Mapped side node', sideNode.id, 'to parent', edge.from);
    }
  });

  // Place side nodes in attachment regions
  const positionedSide: PositionedNode[] = [];
  const spineMap = new Map<string, PositionedNode>();
  positionedSpine.forEach((node) => {
    spineMap.set(node.id, node);
  });

  positionedSpine.forEach((spineNode) => {
    const relatedNodes = sideNodesByParent.get(spineNode.id) || [];
    if (relatedNodes.length === 0) return;

    // Determine which side to use (alternate per step)
    const stepNum = spineNode.nodeRef.stepNumber || 0;
    const useTop = stepNum % 2 === 1; // Odd steps use top, even use bottom

    // Calculate the maximum height of side nodes in this group for proper spacing
    const maxSideHeight = Math.max(...relatedNodes.map(n => n.height));

    // Calculate attachment region position using actual heights
    const regionY = useTop
      ? spineNode.y - SIDE_REGION_OFFSET - maxSideHeight
      : spineNode.y + spineNode.height + SIDE_REGION_OFFSET;

    // Order side nodes by their target spine node's horizontal position to minimize edge crossings
    const orderedNodes = [...relatedNodes].sort((a, b) => {
      // Find the spine node(s) each side node connects to
      const aTargets = edges
        .filter(e => e.from === a.id && e.kind === 'spine')
        .map(e => spineMap.get(e.to))
        .filter(n => n !== undefined) as PositionedNode[];

      const bTargets = edges
        .filter(e => e.from === b.id && e.kind === 'spine')
        .map(e => spineMap.get(e.to))
        .filter(n => n !== undefined) as PositionedNode[];

      // Use average x position of targets for ordering
      const aAvgX = aTargets.length > 0
        ? aTargets.reduce((sum, n) => sum + n.x, 0) / aTargets.length
        : spineNode.x;
      const bAvgX = bTargets.length > 0
        ? bTargets.reduce((sum, n) => sum + n.x, 0) / bTargets.length
        : spineNode.x;

      return aAvgX - bAvgX;
    });

    // Pack side nodes horizontally in the attachment region
    // Align with spine node's horizontal center
    const spineCenterX = spineNode.x + spineNode.width / 2;
    const totalSideWidth = orderedNodes.length * (CARD_WIDTH + SIDE_NODE_SPACING) - SIDE_NODE_SPACING;
    let startX = spineCenterX - totalSideWidth / 2;

    orderedNodes.forEach((sideNode) => {
      const positioned: PositionedNode = {
        ...sideNode,
        x: startX,
        y: regionY,
        ports: calculatePorts({ ...sideNode, x: startX, y: regionY } as PositionedNode),
      };

      positionedSide.push(positioned);
      startX += CARD_WIDTH + SIDE_NODE_SPACING;
    });
  });

  // Combine and recalculate ports (ensure they use header-centered positioning)
  const allPositioned = [...positionedSpine, ...positionedSide];

  console.log('[placeNodes] Result:', {
    totalPositioned: allPositioned.length,
    spine: positionedSpine.length,
    side: positionedSide.length
  });

  return allPositioned.map((node) => {
    // Use the calculatePorts function to ensure consistent port positioning
    return {
      ...node,
      ports: calculatePorts(node),
    };
  });
}

/**
 * Calculate bounding box for positioned nodes
 */
export function calculateBounds(nodes: PositionedNode[]): {
  x: number;
  y: number;
  width: number;
  height: number;
} {
  if (nodes.length === 0) {
    return { x: 0, y: 0, width: 0, height: 0 };
  }

  let minX = Infinity;
  let minY = Infinity;
  let maxX = -Infinity;
  let maxY = -Infinity;

  nodes.forEach((node) => {
    minX = Math.min(minX, node.x);
    minY = Math.min(minY, node.y);
    maxX = Math.max(maxX, node.x + node.width);
    maxY = Math.max(maxY, node.y + node.height);
  });

  return {
    x: minX,
    y: minY,
    width: maxX - minX,
    height: maxY - minY,
  };
}

