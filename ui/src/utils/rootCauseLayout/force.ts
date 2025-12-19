/**
 * Force-directed layout engine for RootCauseView
 * Uses d3-force simulation with clustering by edgeType and relationshipType
 */

import * as d3 from 'd3';
import { LayoutNode, LayoutEdge, PositionedNode, calculatePorts, CARD_WIDTH } from './model';

// Force simulation parameters
const SIMULATION_ITERATIONS = 400; // Number of ticks to run (deterministic)
const COLLISION_PADDING = 60; // Padding around cards (balanced for readability)
const SPINE_LINK_DISTANCE = 200; // Base additional distance for spine edges (more compact)
const ATTACHMENT_LINK_DISTANCE = 150; // Base additional distance for attachment edges (more compact)
const CHARGE_STRENGTH = -1800; // Repulsion strength (reduced for compactness)
const SPINE_CENTER_Y = 400; // Y position for spine node cluster center
const SPINE_SPACING_X = CARD_WIDTH + 200; // Horizontal spacing for spine nodes (more compact)
const ATTACHMENT_RADIUS_BASE = 350; // Base radius for attachment nodes (more compact)
const ATTACHMENT_RADIUS_VARIANCE = 120; // Variance in radius per relationship type

/**
 * Simple hash function for deterministic positioning
 */
function hashString(str: string): number {
  let hash = 0;
  for (let i = 0; i < str.length; i++) {
    const char = str.charCodeAt(i);
    hash = ((hash << 5) - hash) + char;
    hash = hash & hash; // Convert to 32bit integer
  }
  return Math.abs(hash);
}

/**
 * Get deterministic initial position for a node
 */
function getInitialPosition(node: LayoutNode, allNodes: LayoutNode[]): { x: number; y: number } {
  const hash = hashString(node.id);

  if (node.kind === 'spine') {
    // Spine nodes: spread horizontally, centered vertically
    const spineNodes = allNodes.filter(n => n.kind === 'spine');
    const spineIndex = spineNodes.findIndex(n => n.id === node.id);
    const stepNumber = node.nodeRef.stepNumber || spineIndex;

    // Use stepNumber to order horizontally, with some jitter from hash
    const baseX = 500 + stepNumber * SPINE_SPACING_X;
    const jitterX = (hash % 80) - 40; // Reduced jitter for better initial spacing

    return {
      x: baseX + jitterX,
      y: SPINE_CENTER_Y + ((hash % 150) - 75), // Reduced vertical jitter
    };
  } else {
    // Side nodes: start in a ring around their parent
    // We'll refine this with relationship-based clustering
    const angle = (hash % 360) * (Math.PI / 180);
    const radius = ATTACHMENT_RADIUS_BASE + (hash % ATTACHMENT_RADIUS_VARIANCE);

    return {
      x: 800 + Math.cos(angle) * radius,
      y: SPINE_CENTER_Y + Math.sin(angle) * radius,
    };
  }
}

/**
 * Get relationship type cluster parameters (angle and radius offset)
 * Returns consistent positioning for nodes with the same relationship type
 */
function getRelationshipClusterParams(relationshipType: string | undefined, parentSpineId: string): { angle: number; radius: number } {
  if (!relationshipType) {
    relationshipType = 'UNKNOWN';
  }

  // Create a stable hash from relationship type + parent ID
  const clusterKey = `${relationshipType}_${parentSpineId}`;
  const hash = hashString(clusterKey);

  // Map to angle (0-360 degrees) and radius offset
  const angle = (hash % 360) * (Math.PI / 180);
  const radiusOffset = (hash % ATTACHMENT_RADIUS_VARIANCE) - (ATTACHMENT_RADIUS_VARIANCE / 2);

  return {
    angle,
    radius: ATTACHMENT_RADIUS_BASE + radiusOffset,
  };
}

/**
 * Build parent-child mapping for attachment nodes
 */
function buildParentMap(edges: LayoutEdge[]): Map<string, string> {
  const parentMap = new Map<string, string>();

  edges.forEach(edge => {
    if (edge.kind === 'attachment') {
      // Attachment edges: from spine to side
      parentMap.set(edge.to, edge.from);
    }
  });

  return parentMap;
}

/**
 * Place nodes using force-directed layout with clustering
 */
export function placeNodesForce(
  nodes: LayoutNode[],
  edges: LayoutEdge[]
): PositionedNode[] {
  if (nodes.length === 0) {
    console.warn('[placeNodesForce] No nodes to place');
    return [];
  }

  console.log('[placeNodesForce] Processing:', {
    total: nodes.length,
    spine: nodes.filter(n => n.kind === 'spine').length,
    side: nodes.filter(n => n.kind === 'side').length,
    edges: edges.length,
    nodeDetails: nodes.map(n => ({ id: n.id, kind: n.kind, resource: `${n.nodeRef.resource.kind}/${n.nodeRef.resource.name}` })),
    edgeDetails: edges.map(e => ({ from: e.from, to: e.to, kind: e.kind, relationshipType: e.relationshipType })),
  });

  // Build parent mapping for attachment nodes
  const parentMap = buildParentMap(edges);

  // Create d3-force simulation data
  // d3-force expects nodes with x, y properties that it will modify
  interface SimulationNode extends d3.SimulationNodeDatum {
    id: string;
    node: LayoutNode;
  }

  interface LinkData extends d3.SimulationLinkDatum<SimulationNode> {
    kind: 'spine' | 'attachment';
  }

  const simulationNodes: SimulationNode[] = nodes.map(node => {
    const initial = getInitialPosition(node, nodes);
    return {
      id: node.id,
      node: node,
      x: initial.x,
      y: initial.y,
    };
  });

  // Create node map for link force
  const nodeMap = new Map<string, SimulationNode>();
  simulationNodes.forEach(n => nodeMap.set(n.id, n));

  // Create link data with actual node references
  const linkData: LinkData[] = edges.map(edge => ({
    source: nodeMap.get(edge.from)!,
    target: nodeMap.get(edge.to)!,
    kind: edge.kind,
  }));

  const spineCount = nodes.filter(n => n.kind === 'spine').length;
  const spineTargetXById = new Map<string, number>();
  nodes.forEach(n => {
    if (n.kind !== 'spine') return;
    const step = n.nodeRef.stepNumber ?? 0;
    spineTargetXById.set(n.id, 500 + step * SPINE_SPACING_X);
  });

  // Create simulation
  const simulation = d3.forceSimulation(simulationNodes)
    .alphaDecay(0.02) // Slower decay for more stable results
    .velocityDecay(0.4)
    .force('link', d3.forceLink(linkData)
      .id((d: SimulationNode) => d.id)
      .distance((d: LinkData) => {
        // IMPORTANT: include actual node sizes so edges don’t pull large cards into overlap.
        const s = d.source as SimulationNode;
        const t = d.target as SimulationNode;
        const sR = Math.sqrt(s.node.width * s.node.width + s.node.height * s.node.height) / 2;
        const tR = Math.sqrt(t.node.width * t.node.width + t.node.height * t.node.height) / 2;
        const base = d.kind === 'spine' ? SPINE_LINK_DISTANCE : ATTACHMENT_LINK_DISTANCE;
        return base + sR + tR + COLLISION_PADDING;
      })
      .strength(0.8)
    )
    .force('charge', d3.forceManyBody()
      .strength(CHARGE_STRENGTH)
      .distanceMax(2000) // Increased repulsion range for better spacing
    )
    .force('collision', d3.forceCollide()
      .radius((d: SimulationNode) => {
        const node = d.node;
        // Use half-diagonal so the collision circle fully contains the rectangle footprint.
        const radius = Math.sqrt(node.width * node.width + node.height * node.height) / 2 + COLLISION_PADDING;
        return radius;
      })
      .strength(1.0) // Increased strength to ensure no overlaps
    )
    .force('center', d3.forceCenter(800, SPINE_CENTER_Y).strength(0.02))
    // Spine nodes: keep them in a readable band and ordered by step number
    // Stronger forces for better linear flow
    .force('spineX', d3.forceX((d: SimulationNode) => {
      if (d.node.kind !== 'spine') return d.x ?? 0;
      return spineTargetXById.get(d.id) ?? (d.x ?? 0);
    }).strength((d: any) => (d.node?.kind === 'spine' ? 0.5 : 0)))
    .force('spineY', d3.forceY((d: SimulationNode) => {
      if (d.node.kind !== 'spine') return d.y ?? 0;
      return SPINE_CENTER_Y;
    }).strength((d: any) => (d.node?.kind === 'spine' ? 0.6 : 0)))
    // Side nodes: cluster by relationship type around their parent spine's target position
    // Increased strength for tighter clustering
    .force('sideX', d3.forceX((d: SimulationNode) => {
      if (d.node.kind !== 'side') return d.x ?? 0;
      const parentId = parentMap.get(d.id);
      const parentTargetX = parentId ? (spineTargetXById.get(parentId) ?? 800) : 800;
      const attachmentEdge = edges.find(e => e.kind === 'attachment' && e.to === d.id);
      const rel = attachmentEdge?.relationshipType;
      const cluster = getRelationshipClusterParams(rel, parentId ?? 'NO_PARENT');
      return parentTargetX + Math.cos(cluster.angle) * cluster.radius;
    }).strength((d: any) => (d.node?.kind === 'side' ? 0.3 : 0)))
    .force('sideY', d3.forceY((d: SimulationNode) => {
      if (d.node.kind !== 'side') return d.y ?? 0;
      const parentId = parentMap.get(d.id);
      const parentTargetY = SPINE_CENTER_Y;
      const attachmentEdge = edges.find(e => e.kind === 'attachment' && e.to === d.id);
      const rel = attachmentEdge?.relationshipType;
      const cluster = getRelationshipClusterParams(rel, parentId ?? 'NO_PARENT');
      return parentTargetY + Math.sin(cluster.angle) * cluster.radius;
    }).strength((d: any) => (d.node?.kind === 'side' ? 0.3 : 0)));

  // Run simulation for fixed number of iterations
  for (let i = 0; i < SIMULATION_ITERATIONS; i++) {
    // Tick simulation
    simulation.tick();
  }

  // Stop simulation
  simulation.stop();

  // Ensure all nodes have valid positions (not NaN or Infinity)
  simulationNodes.forEach(simNode => {
    if (!isFinite(simNode.x) || !isFinite(simNode.y)) {
      console.warn(`[placeNodesForce] Invalid position for node ${simNode.id}, resetting`);
      const initial = getInitialPosition(simNode.node, nodes);
      simNode.x = initial.x;
      simNode.y = initial.y;
    }
  });

  // Convert to PositionedNode format
  const positionedNodes: PositionedNode[] = simulationNodes.map(simNode => {
    const positioned: PositionedNode = {
      ...simNode.node,
      x: simNode.x,
      y: simNode.y,
      ports: calculatePorts({
        ...simNode.node,
        x: simNode.x,
        y: simNode.y,
      } as PositionedNode),
    };
    return positioned;
  });

  // Log bounds for debugging
  if (positionedNodes.length > 0) {
    const minX = Math.min(...positionedNodes.map(n => n.x));
    const minY = Math.min(...positionedNodes.map(n => n.y));
    const maxX = Math.max(...positionedNodes.map(n => n.x + n.width));
    const maxY = Math.max(...positionedNodes.map(n => n.y + n.height));

    console.log('[placeNodesForce] Result:', {
      totalPositioned: positionedNodes.length,
      spine: positionedNodes.filter(n => n.kind === 'spine').length,
      side: positionedNodes.filter(n => n.kind === 'side').length,
      bounds: { x: minX, y: minY, width: maxX - minX, height: maxY - minY },
    });
  }

  return positionedNodes;
}

