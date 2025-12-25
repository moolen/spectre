/**
 * Obstacle-aware orthogonal edge router
 * Uses grid-based A* pathfinding with lane/trunk cost optimization
 */

import { PositionedNode, RoutedEdge, LayoutEdge, Port, selectPorts, HEADER_HEIGHT } from './model';

// Grid cell size (coarse grid for performance)
const GRID_CELL_SIZE = 25;

// Cost constants for pathfinding
const COST_STRAIGHT = 1;
const COST_BEND = 5;
const COST_NEW_LANE = 10;
const COST_SPINE_TRUNK = 0.5; // Prefer routing along spine
const COST_ATTACHMENT_TRUNK = 0.8; // Prefer routing along attachment lanes
const COST_BLOCKED = 10000;

interface GridCell {
  x: number;
  y: number;
  blocked: boolean;
  isSpineTrunk: boolean;
  isAttachmentTrunk: boolean;
}

interface PathNode {
  x: number;
  y: number;
  g: number; // Cost from start
  h: number; // Heuristic to end
  f: number; // Total cost
  parent?: PathNode;
}

/**
 * Build obstacle grid from positioned nodes
 */
function buildGrid(
  nodes: PositionedNode[],
  bounds: { x: number; y: number; width: number; height: number }
): GridCell[][] {
  const cols = Math.ceil(bounds.width / GRID_CELL_SIZE) + 2;
  const rows = Math.ceil(bounds.height / GRID_CELL_SIZE) + 2;
  const grid: GridCell[][] = [];

  // Initialize grid
  for (let y = 0; y < rows; y++) {
    grid[y] = [];
    for (let x = 0; x < cols; x++) {
      const worldX = bounds.x + (x - 1) * GRID_CELL_SIZE;
      const worldY = bounds.y + (y - 1) * GRID_CELL_SIZE;
      grid[y][x] = {
        x: worldX,
        y: worldY,
        blocked: false,
        isSpineTrunk: false,
        isAttachmentTrunk: false,
      };
    }
  }

  // Mark node rectangles as blocked
  const OBSTACLE_PADDING = 0; // No padding - we'll rely on better node positioning

  nodes.forEach((node) => {
    const startCol = Math.floor((node.x - OBSTACLE_PADDING - bounds.x) / GRID_CELL_SIZE);
    const endCol = Math.ceil((node.x + node.width + OBSTACLE_PADDING - bounds.x) / GRID_CELL_SIZE);
    const startRow = Math.floor((node.y - OBSTACLE_PADDING - bounds.y) / GRID_CELL_SIZE);
    const endRow = Math.ceil((node.y + node.height + OBSTACLE_PADDING - bounds.y) / GRID_CELL_SIZE);

    for (let row = startRow; row <= endRow; row++) {
      for (let col = startCol; col <= endCol; col++) {
        if (row >= 0 && row < rows && col >= 0 && col < cols) {
          grid[row][col].blocked = true;
        }
      }
    }
  });

  // Mark spine trunk (horizontal lane through spine nodes)
  const spineNodes = nodes.filter((n) => n.kind === 'spine');
  if (spineNodes.length > 0) {
    const spineCenterY = spineNodes.reduce((sum, n) => sum + n.y + n.height / 2, 0) / spineNodes.length;
    const spineRow = Math.floor((spineCenterY - bounds.y) / GRID_CELL_SIZE);

    for (let col = 0; col < cols; col++) {
      if (spineRow >= 0 && spineRow < rows && !grid[spineRow][col].blocked) {
        grid[spineRow][col].isSpineTrunk = true;
      }
    }
  }

  // Mark attachment trunks (vertical lanes from spine to side clusters)
  spineNodes.forEach((spineNode) => {
    const sideNodes = nodes.filter((n) => n.kind === 'side' && n.parentId === spineNode.id);
    if (sideNodes.length > 0) {
      const spineCenterX = spineNode.x + spineNode.width / 2;
      const spineCol = Math.floor((spineCenterX - bounds.x) / GRID_CELL_SIZE);

      sideNodes.forEach((sideNode) => {
        const sideCenterY = sideNode.y + sideNode.height / 2;
        const sideRow = Math.floor((sideCenterY - bounds.y) / GRID_CELL_SIZE);
        const spineRow = Math.floor((spineNode.y + spineNode.height / 2 - bounds.y) / GRID_CELL_SIZE);

        const startRow = Math.min(spineRow, sideRow);
        const endRow = Math.max(spineRow, sideRow);

        for (let row = startRow; row <= endRow; row++) {
          if (row >= 0 && row < rows && spineCol >= 0 && spineCol < cols && !grid[row][spineCol].blocked) {
            grid[row][spineCol].isAttachmentTrunk = true;
          }
        }
      });
    }
  });

  return grid;
}

/**
 * Convert world coordinates to grid coordinates
 */
function worldToGrid(
  x: number,
  y: number,
  bounds: { x: number; y: number }
): { col: number; row: number } {
  return {
    col: Math.floor((x - bounds.x) / GRID_CELL_SIZE),
    row: Math.floor((y - bounds.y) / GRID_CELL_SIZE),
  };
}

/**
 * Convert grid coordinates to world coordinates (center of cell)
 */
function gridToWorld(col: number, row: number, bounds: { x: number; y: number }): { x: number; y: number } {
  return {
    x: bounds.x + col * GRID_CELL_SIZE + GRID_CELL_SIZE / 2,
    y: bounds.y + row * GRID_CELL_SIZE + GRID_CELL_SIZE / 2,
  };
}

/**
 * Heuristic function (Manhattan distance)
 */
function heuristic(a: PathNode, b: PathNode): number {
  return Math.abs(a.x - b.x) + Math.abs(a.y - b.y);
}

/**
 * Get neighbors for A* pathfinding (orthogonal only)
 */
function getNeighbors(node: PathNode, grid: GridCell[][], bounds: { x: number; y: number }): PathNode[] {
  const neighbors: PathNode[] = [];
  const { col, row } = worldToGrid(node.x, node.y, bounds);

  const directions = [
    { dx: 0, dy: -1 }, // Up
    { dx: 1, dy: 0 },  // Right
    { dx: 0, dy: 1 },  // Down
    { dx: -1, dy: 0 }, // Left
  ];

  directions.forEach((dir) => {
    const newRow = row + dir.dy;
    const newCol = col + dir.dx;

    if (newRow >= 0 && newRow < grid.length && newCol >= 0 && newCol < grid[0].length) {
      const cell = grid[newRow][newCol];
      if (!cell.blocked) {
        const worldPos = gridToWorld(newCol, newRow, bounds);
        neighbors.push({
          x: worldPos.x,
          y: worldPos.y,
          g: 0,
          h: 0,
          f: 0,
        });
      }
    }
  });

  return neighbors;
}

/**
 * Calculate movement cost between two cells
 */
function getMoveCost(
  from: PathNode,
  to: PathNode,
  grid: GridCell[][],
  bounds: { x: number; y: number }
): number {
  const { col: fromCol, row: fromRow } = worldToGrid(from.x, from.y, bounds);
  const { col: toCol, row: toRow } = worldToGrid(to.x, to.y, bounds);

  if (fromRow < 0 || fromRow >= grid.length || fromCol < 0 || fromCol >= grid[0].length) {
    return COST_BLOCKED;
  }

  const cell = grid[toRow][toCol];
  if (cell.blocked) {
    return COST_BLOCKED;
  }

  let cost = COST_STRAIGHT;

  // Check if this is a bend
  if (from.parent) {
    const { col: parentCol, row: parentRow } = worldToGrid(from.parent.x, from.parent.y, bounds);
    const isBend = (parentCol !== toCol && parentRow !== toRow) || (fromCol !== toCol && fromRow !== toRow);
    if (isBend) {
      cost += COST_BEND;
    }
  }

  // Prefer routing along trunks
  if (cell.isSpineTrunk) {
    cost *= COST_SPINE_TRUNK;
  } else if (cell.isAttachmentTrunk) {
    cost *= COST_ATTACHMENT_TRUNK;
  } else {
    // Penalty for creating new lanes
    cost += COST_NEW_LANE;
  }

  return cost;
}

/**
 * A* pathfinding algorithm
 */
function findPath(
  start: { x: number; y: number },
  end: { x: number; y: number },
  grid: GridCell[][],
  bounds: { x: number; y: number }
): Array<{ x: number; y: number }> {
  const startNode: PathNode = { x: start.x, y: start.y, g: 0, h: heuristic({ x: start.x, y: start.y, g: 0, h: 0, f: 0 }, { x: end.x, y: end.y, g: 0, h: 0, f: 0 }), f: 0 };
  const endNode: PathNode = { x: end.x, y: end.y, g: 0, h: 0, f: 0 };

  const openSet: PathNode[] = [startNode];
  const closedSet = new Set<string>();

  const getKey = (node: PathNode): string => `${node.x},${node.y}`;

  while (openSet.length > 0) {
    // Find node with lowest f score
    openSet.sort((a, b) => a.f - b.f);
    const current = openSet.shift()!;

    if (getKey(current) === getKey(endNode)) {
      // Reconstruct path
      const path: Array<{ x: number; y: number }> = [];
      let node: PathNode | undefined = current;
      while (node) {
        path.unshift({ x: node.x, y: node.y });
        node = node.parent;
      }
      return path;
    }

    closedSet.add(getKey(current));

    const neighbors = getNeighbors(current, grid, bounds);
    for (const neighbor of neighbors) {
      if (closedSet.has(getKey(neighbor))) continue;

      const tentativeG = current.g + getMoveCost(current, neighbor, grid, bounds);
      neighbor.parent = current;
      neighbor.g = tentativeG;
      neighbor.h = heuristic(neighbor, endNode);
      neighbor.f = neighbor.g + neighbor.h;

      // Check if already in open set
      const existing = openSet.find((n) => getKey(n) === getKey(neighbor));
      if (!existing) {
        openSet.push(neighbor);
      } else if (tentativeG < existing.g) {
        existing.g = tentativeG;
        existing.f = existing.g + existing.h;
        existing.parent = current;
      }
    }
  }

  // No path found, return straight line
  return [{ x: start.x, y: start.y }, { x: end.x, y: end.y }];
}

/**
 * Simplify path by removing collinear points
 */
function simplifyPath(points: Array<{ x: number; y: number }>): Array<{ x: number; y: number }> {
  if (points.length <= 2) return points;

  const simplified: Array<{ x: number; y: number }> = [points[0]];

  for (let i = 1; i < points.length - 1; i++) {
    const prev = points[i - 1];
    const curr = points[i];
    const next = points[i + 1];

    // Check if current point is collinear with prev and next
    const dx1 = curr.x - prev.x;
    const dy1 = curr.y - prev.y;
    const dx2 = next.x - curr.x;
    const dy2 = next.y - curr.y;

    // Points are collinear if cross product is zero (within tolerance)
    const crossProduct = dx1 * dy2 - dy1 * dx2;
    if (Math.abs(crossProduct) > 0.1) {
      // Not collinear, keep this point
      simplified.push(curr);
    }
  }

  simplified.push(points[points.length - 1]);
  return simplified;
}

/**
 * Route edges using grid-based A* pathfinding
 */
export function routeEdges(
  edges: LayoutEdge[],
  nodes: PositionedNode[],
  bounds: { x: number; y: number; width: number; height: number }
): RoutedEdge[] {
  if (edges.length === 0) {
    return [];
  }

  // Build obstacle grid
  const grid = buildGrid(nodes, bounds);

  // Create node map for quick lookup
  const nodeMap = new Map<string, PositionedNode>();
  nodes.forEach((node) => {
    nodeMap.set(node.id, node);
  });

  // Route each edge
  const routedEdges: RoutedEdge[] = [];

  edges.forEach((edge) => {
    const fromNode = nodeMap.get(edge.from);
    const toNode = nodeMap.get(edge.to);

    if (!fromNode || !toNode) {
      console.warn(`[routeEdges] Missing node for edge ${edge.from} -> ${edge.to}`);
      return;
    }

    // Select ports
    // For attachment edges, use specific ports based on layout
    let fromPort: Port, toPort: Port;
    if (edge.kind === 'attachment') {
      // Attachment edges: spine → side node
      // Connect at actual card boundaries (not header center)
      const dy = toNode.y - fromNode.y;
      if (dy < 0) {
        // Side node is above spine: use top edge of spine, bottom edge of side
        fromPort = {
          side: 'top',
          x: fromNode.x + fromNode.width / 2,
          y: fromNode.y  // Top edge of card
        };
        toPort = {
          side: 'bottom',
          x: toNode.x + toNode.width / 2,
          y: toNode.y + toNode.height  // Bottom edge of card
        };
      } else {
        // Side node is below spine: use bottom edge of spine, top edge of side
        fromPort = {
          side: 'bottom',
          x: fromNode.x + fromNode.width / 2,
          y: fromNode.y + fromNode.height  // Bottom edge of card
        };
        toPort = {
          side: 'top',
          x: toNode.x + toNode.width / 2,
          y: toNode.y  // Top edge of card
        };
      }
    } else {
      // Spine edges: use general port selection, but ensure ports are at card boundaries
      const ports = selectPorts(fromNode, toNode);
      fromPort = ports.fromPort;
      toPort = ports.toPort;

      // Adjust fromPort to be at actual card boundaries
      // Preserve the coordinate that's not on the boundary (x for top/bottom, y for left/right)
      if (fromPort.side === 'top') {
        fromPort = {
          side: 'top',
          x: fromNode.x + fromNode.width / 2, // Center horizontally
          y: fromNode.y // Top edge
        };
      } else if (fromPort.side === 'bottom') {
        fromPort = {
          side: 'bottom',
          x: fromNode.x + fromNode.width / 2, // Center horizontally
          y: fromNode.y + fromNode.height // Bottom edge
        };
      } else if (fromPort.side === 'left') {
        fromPort = {
          side: 'left',
          x: fromNode.x, // Left edge
          y: fromNode.y + HEADER_HEIGHT / 2 // Header center (30px from top)
        };
      } else if (fromPort.side === 'right') {
        fromPort = {
          side: 'right',
          x: fromNode.x + fromNode.width, // Right edge
          y: fromNode.y + HEADER_HEIGHT / 2 // Header center (30px from top)
        };
      }

      // Adjust toPort to be at actual card boundaries
      if (toPort.side === 'top') {
        toPort = {
          side: 'top',
          x: toNode.x + toNode.width / 2, // Center horizontally
          y: toNode.y // Top edge
        };
      } else if (toPort.side === 'bottom') {
        toPort = {
          side: 'bottom',
          x: toNode.x + toNode.width / 2, // Center horizontally
          y: toNode.y + toNode.height // Bottom edge
        };
      } else if (toPort.side === 'left') {
        toPort = {
          side: 'left',
          x: toNode.x, // Left edge
          y: toNode.y + HEADER_HEIGHT / 2 // Header center (30px from top)
        };
      } else if (toPort.side === 'right') {
        toPort = {
          side: 'right',
          x: toNode.x + toNode.width, // Right edge
          y: toNode.y + HEADER_HEIGHT / 2 // Header center (30px from top)
        };
      }
    }

    // Store exact port positions to ensure they're used
    const exactFromPort = { x: fromPort.x, y: fromPort.y };
    const exactToPort = { x: toPort.x, y: toPort.y };

    // Find path using A*
    let path = findPath(
      exactFromPort,
      exactToPort,
      grid,
      bounds
    );

    // Ensure we have at least 2 points (fallback to direct line if routing fails)
    if (path.length < 2) {
      path = [exactFromPort, exactToPort];
    }

    // CRITICAL: Ensure first and last points are exactly at port positions
    // This guarantees edges connect precisely to card boundaries
    // Override any rounding or approximation from pathfinding
    path[0] = exactFromPort;
    path[path.length - 1] = exactToPort;

    // Simplify path (but preserve endpoints)
    const simplifiedPath = simplifyPath(path);

    // CRITICAL: After simplification, force endpoints to exact port positions
    // This ensures no rounding or approximation affects the connection points
    simplifiedPath[0] = exactFromPort;
    simplifiedPath[simplifiedPath.length - 1] = exactToPort;

    routedEdges.push({
      ...edge,
      points: simplifiedPath,
      fromPort: { ...fromPort, x: exactFromPort.x, y: exactFromPort.y },
      toPort: { ...toPort, x: exactToPort.x, y: exactToPort.y },
    });
  });

  return routedEdges;
}

