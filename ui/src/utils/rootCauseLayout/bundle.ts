/**
 * Edge bundling: assign edges to lanes and add offsets for parallel edges
 */

import { RoutedEdge } from './model';

// Offset spacing between parallel edges
const EDGE_OFFSET_SPACING = 8;

interface LaneSegment {
  x1: number;
  y1: number;
  x2: number;
  y2: number;
  edges: Array<{ edge: RoutedEdge; index: number }>;
}

/**
 * Check if two line segments are parallel (within tolerance)
 */
function areParallel(
  x1: number,
  y1: number,
  x2: number,
  y2: number,
  x3: number,
  y3: number,
  x4: number,
  y4: number,
  tolerance: number = 5
): boolean {
  const dx1 = x2 - x1;
  const dy1 = y2 - y1;
  const dx2 = x4 - x3;
  const dy2 = y4 - y3;

  // Normalize vectors
  const len1 = Math.sqrt(dx1 * dx1 + dy1 * dy1);
  const len2 = Math.sqrt(dx2 * dx2 + dy2 * dy2);

  if (len1 < 0.1 || len2 < 0.1) return false;

  const nx1 = dx1 / len1;
  const ny1 = dy1 / len1;
  const nx2 = dx2 / len2;
  const ny2 = dy2 / len2;

  // Check if normalized vectors are similar (dot product close to 1 or -1)
  const dot = nx1 * nx2 + ny1 * ny2;
  return Math.abs(Math.abs(dot) - 1) < tolerance / 100;
}

/**
 * Calculate perpendicular offset for a point along a line segment
 */
function offsetPoint(
  x1: number,
  y1: number,
  x2: number,
  y2: number,
  offset: number
): { x: number; y: number } {
  const dx = x2 - x1;
  const dy = y2 - y1;
  const len = Math.sqrt(dx * dx + dy * dy);

  if (len < 0.1) {
    return { x: x1, y: y1 };
  }

  // Perpendicular vector (normalized)
  const perpX = -dy / len;
  const perpY = dx / len;

  return {
    x: x1 + perpX * offset,
    y: y1 + perpY * offset,
  };
}

/**
 * Apply bundling to routed edges
 * Groups parallel segments and adds offsets
 */
export function bundleEdges(edges: RoutedEdge[]): RoutedEdge[] {
  if (edges.length === 0) {
    return edges;
  }

  // Extract all line segments from edges
  const segments: LaneSegment[] = [];

  edges.forEach((edge, edgeIdx) => {
    for (let i = 0; i < edge.points.length - 1; i++) {
      const p1 = edge.points[i];
      const p2 = edge.points[i + 1];

      // Check if this segment is parallel to any existing segment
      let foundLane = false;
      for (const lane of segments) {
        if (
          areParallel(
            lane.x1,
            lane.y1,
            lane.x2,
            lane.y2,
            p1.x,
            p1.y,
            p2.x,
            p2.y
          )
        ) {
          // Check if segments overlap or are close
          const dist1 = Math.abs((p2.x - p1.x) * (lane.y1 - p1.y) - (p2.y - p1.y) * (lane.x1 - p1.x)) / Math.sqrt((p2.x - p1.x) ** 2 + (p2.y - p1.y) ** 2);
          const dist2 = Math.abs((p2.x - p1.x) * (lane.y2 - p1.y) - (p2.y - p1.y) * (lane.x2 - p1.x)) / Math.sqrt((p2.x - p1.x) ** 2 + (p2.y - p1.y) ** 2);

          if (dist1 < EDGE_OFFSET_SPACING * 2 || dist2 < EDGE_OFFSET_SPACING * 2) {
            lane.edges.push({ edge, index: i });
            foundLane = true;
            break;
          }
        }
      }

      if (!foundLane) {
        // Create new lane
        segments.push({
          x1: p1.x,
          y1: p1.y,
          x2: p2.x,
          y2: p2.y,
          edges: [{ edge, index: i }],
        });
      }
    }
  });

  // Assign offsets to edges in each lane
  const bundledEdges = edges.map((edge) => ({ ...edge, points: [...edge.points] }));

  segments.forEach((lane) => {
    if (lane.edges.length <= 1) return; // No bundling needed for single edge

    // Sort edges by their position along the lane
    lane.edges.sort((a, b) => {
      const aMidX = (a.edge.points[a.index].x + a.edge.points[a.index + 1].x) / 2;
      const aMidY = (a.edge.points[a.index].y + a.edge.points[a.index + 1].y) / 2;
      const bMidX = (b.edge.points[b.index].x + b.edge.points[b.index + 1].x) / 2;
      const bMidY = (b.edge.points[b.index].y + b.edge.points[b.index + 1].y) / 2;

      // Project onto lane direction
      const dx = lane.x2 - lane.x1;
      const dy = lane.y2 - lane.y1;
      const len = Math.sqrt(dx * dx + dy * dy);
      if (len < 0.1) return 0;

      const projA = ((aMidX - lane.x1) * dx + (aMidY - lane.y1) * dy) / len;
      const projB = ((bMidX - lane.x1) * dx + (bMidY - lane.y1) * dy) / len;

      return projA - projB;
    });

    // Assign offsets
    const centerOffset = -(lane.edges.length - 1) * EDGE_OFFSET_SPACING / 2;
    lane.edges.forEach((item, idx) => {
      const offset = centerOffset + idx * EDGE_OFFSET_SPACING;
      const p1 = item.edge.points[item.index];
      const p2 = item.edge.points[item.index + 1];

      const offset1 = offsetPoint(lane.x1, lane.y1, lane.x2, lane.y2, offset);
      const offset2 = offsetPoint(lane.x1, lane.y1, lane.x2, lane.y2, offset);

      // Calculate actual offset points
      const dx = p2.x - p1.x;
      const dy = p2.y - p1.y;
      const len = Math.sqrt(dx * dx + dy * dy);
      if (len > 0.1) {
        const perpX = -dy / len;
        const perpY = dx / len;

        const offsetP1 = {
          x: p1.x + perpX * offset,
          y: p1.y + perpY * offset,
        };
        const offsetP2 = {
          x: p2.x + perpX * offset,
          y: p2.y + perpY * offset,
        };

        // Update points in bundled edge
        const bundledEdge = bundledEdges.find((e) => e.id === item.edge.id);
        if (bundledEdge) {
          bundledEdge.points[item.index] = offsetP1;
          bundledEdge.points[item.index + 1] = offsetP2;
        }
      }
    });
  });

  return bundledEdges;
}






