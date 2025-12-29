/**
 * Simple direct edge routing for force-directed layout
 * Routes edges as straight lines between selected ports
 */

import { LayoutEdge, PositionedNode, RoutedEdge, Port, selectPorts, HEADER_HEIGHT } from './model';

/**
 * Route edges using direct connections (no obstacle avoidance)
 * For force-directed layouts, direct routing is sufficient since
 * the layout already avoids overlaps
 */
export function routeEdgesDirect(
  edges: LayoutEdge[],
  nodes: PositionedNode[]
): RoutedEdge[] {
  if (edges.length === 0) {
    return [];
  }

  // Create node map for quick lookup
  const nodeMap = new Map<string, PositionedNode>();
  nodes.forEach((node) => {
    nodeMap.set(node.id, node);
  });

  const routedEdges: RoutedEdge[] = [];

  edges.forEach((edge) => {
    const fromNode = nodeMap.get(edge.from);
    const toNode = nodeMap.get(edge.to);

    if (!fromNode || !toNode) {
      console.warn(`[routeEdgesDirect] Missing node for edge ${edge.from} -> ${edge.to}`, {
        edge: edge,
        fromNodeExists: !!fromNode,
        toNodeExists: !!toNode,
        availableNodeIds: Array.from(nodeMap.keys()),
      });
      return;
    }

    // Select ports based on node positions
    let fromPort: Port, toPort: Port;

    if (edge.kind === 'attachment') {
      // Attachment edges: spine → side node
      // Connect at card boundaries
      const dy = toNode.y - fromNode.y;
      if (dy < 0) {
        // Side node is above spine: use top edge of spine, bottom edge of side
        fromPort = {
          side: 'top',
          x: fromNode.x + fromNode.width / 2,
          y: fromNode.y, // Top edge of card
        };
        toPort = {
          side: 'bottom',
          x: toNode.x + toNode.width / 2,
          y: toNode.y + toNode.height, // Bottom edge of card
        };
      } else {
        // Side node is below spine: use bottom edge of spine, top edge of side
        fromPort = {
          side: 'bottom',
          x: fromNode.x + fromNode.width / 2,
          y: fromNode.y + fromNode.height, // Bottom edge of card
        };
        toPort = {
          side: 'top',
          x: toNode.x + toNode.width / 2,
          y: toNode.y, // Top edge of card
        };
      }
    } else {
      // Spine edges: use general port selection
      const ports = selectPorts(fromNode, toNode);
      fromPort = ports.fromPort;
      toPort = ports.toPort;

      // Adjust ports to be at actual card boundaries
      if (fromPort.side === 'top') {
        fromPort = {
          side: 'top',
          x: fromNode.x + fromNode.width / 2,
          y: fromNode.y,
        };
      } else if (fromPort.side === 'bottom') {
        fromPort = {
          side: 'bottom',
          x: fromNode.x + fromNode.width / 2,
          y: fromNode.y + fromNode.height,
        };
      } else if (fromPort.side === 'left') {
        fromPort = {
          side: 'left',
          x: fromNode.x,
          y: fromNode.y + HEADER_HEIGHT / 2,
        };
      } else if (fromPort.side === 'right') {
        fromPort = {
          side: 'right',
          x: fromNode.x + fromNode.width,
          y: fromNode.y + HEADER_HEIGHT / 2,
        };
      }

      if (toPort.side === 'top') {
        toPort = {
          side: 'top',
          x: toNode.x + toNode.width / 2,
          y: toNode.y,
        };
      } else if (toPort.side === 'bottom') {
        toPort = {
          side: 'bottom',
          x: toNode.x + toNode.width / 2,
          y: toNode.y + toNode.height,
        };
      } else if (toPort.side === 'left') {
        toPort = {
          side: 'left',
          x: toNode.x,
          y: toNode.y + HEADER_HEIGHT / 2,
        };
      } else if (toPort.side === 'right') {
        toPort = {
          side: 'right',
          x: toNode.x + toNode.width,
          y: toNode.y + HEADER_HEIGHT / 2,
        };
      }
    }

    // Create direct route (2 points: from port to to port)
    routedEdges.push({
      ...edge,
      points: [
        { x: fromPort.x, y: fromPort.y },
        { x: toPort.x, y: toPort.y },
      ],
      fromPort,
      toPort,
    });
  });

  return routedEdges;
}

