import React, { useEffect, useRef, useMemo, useCallback, useImperativeHandle, forwardRef } from 'react';
import * as d3 from 'd3';
import {
  NamespaceGraphResponse,
  D3GraphNode,
  D3GraphLink,
  transformToD3Graph,
  NODE_STATUS_COLORS,
} from '../../types/namespaceGraph';

interface NamespaceGraphProps {
  /** Graph data from API */
  data: NamespaceGraphResponse;
  /** Callback when a node is clicked */
  onNodeClick?: (node: D3GraphNode) => void;
  /** Currently selected node UID */
  selectedNodeId?: string | null;
  /** Set of node UIDs to highlight (dims non-highlighted nodes/edges when set) */
  highlightedNodeUids?: Set<string> | null;
  /** Width of the container (optional, uses container size if not provided) */
  width?: number;
  /** Height of the container (optional, uses container size if not provided) */
  height?: number;
}

/** Imperative handle for controlling zoom from parent */
export interface NamespaceGraphHandle {
  zoomIn: () => void;
  zoomOut: () => void;
  fitToView: () => void;
  resetZoom: () => void;
  focusOnNodes: (nodeUids: Set<string>) => void;
}

// Nodes are drawn as compact cards (kind + name)
const NODE_WIDTH = 196;
const NODE_HEIGHT = 42;
// Padding (around node centres) used when fitting nodes into view
const FIT_PADDING = NODE_WIDTH * 0.75;
// Collision radius (cards are wide, keep them from overlapping)
const COLLISION_RADIUS = 110;
// Gap between an edge's end and the card border
const EDGE_GAP = 4;

/**
 * Clip the straight line between two node centres to the card borders so
 * edges (and their arrowheads) start and end at the card edge.
 */
function clipToCard(fromX: number, fromY: number, toX: number, toY: number, gap: number): [number, number] {
  const dx = toX - fromX;
  const dy = toY - fromY;
  if (dx === 0 && dy === 0) return [fromX, fromY];
  const halfW = NODE_WIDTH / 2 + gap;
  const halfH = NODE_HEIGHT / 2 + gap;
  const scale = Math.min(halfW / Math.abs(dx || 1e-9), halfH / Math.abs(dy || 1e-9));
  return [fromX + dx * Math.min(1, scale), fromY + dy * Math.min(1, scale)];
}

function positionLinks(selection: d3.Selection<d3.BaseType | SVGLineElement, D3GraphLink, SVGGElement, unknown>) {
  selection.each(function (d) {
    const source = d.source as D3GraphNode;
    const target = d.target as D3GraphNode;
    const sx = source.x ?? 0;
    const sy = source.y ?? 0;
    const tx = target.x ?? 0;
    const ty = target.y ?? 0;
    const [x1, y1] = clipToCard(sx, sy, tx, ty, EDGE_GAP);
    const [x2, y2] = clipToCard(tx, ty, sx, sy, EDGE_GAP);
    d3.select(this).attr('x1', x1).attr('y1', y1).attr('x2', x2).attr('y2', y2);
  });
}
// Zoom scale factor for zoom in/out buttons
const ZOOM_SCALE_FACTOR = 1.3;

/**
 * Force-directed graph visualization for Kubernetes namespace resources
 *
 * Features:
 * - D3 force simulation with repulsion, centering, and collision
 * - Pan and zoom support
 * - Draggable nodes
 * - Card nodes with a status dot
 * - Anomaly count badges
 * - Dotted border for cluster-scoped resources
 * - Incremental updates for pagination (new nodes added without full rebuild)
 */
export const NamespaceGraph = forwardRef<NamespaceGraphHandle, NamespaceGraphProps>(
  ({ data, onNodeClick, selectedNodeId, highlightedNodeUids, width: propWidth, height: propHeight }, ref) => {
    const containerRef = useRef<HTMLDivElement>(null);
    const svgRef = useRef<SVGSVGElement>(null);
    const simulationRef = useRef<d3.Simulation<D3GraphNode, D3GraphLink> | null>(null);
    const zoomRef = useRef<d3.ZoomBehavior<SVGSVGElement, unknown> | null>(null);

    // Track if the graph has been initialized
    const isInitializedRef = useRef(false);

    // Track previous node UIDs to detect new nodes for incremental updates
    const prevNodeUidsRef = useRef<Set<string>>(new Set());

    // Track selectedNodeId in a ref to avoid re-rendering the entire graph
    const selectedNodeIdRef = useRef<string | null | undefined>(selectedNodeId);
    selectedNodeIdRef.current = selectedNodeId;

    // Track onNodeClick in a ref to avoid re-rendering when callback changes
    const onNodeClickRef = useRef(onNodeClick);
    onNodeClickRef.current = onNodeClick;

    // Transform API data to D3 format
    const { nodes, links } = useMemo(() => transformToD3Graph(data), [data]);

    // Get container dimensions - only respond to height changes to prevent re-render on sidebar animation
    const [containerSize, setContainerSize] = React.useState({ width: 800, height: 600 });
    const lastSizeRef = useRef({ width: 0, height: 0 });
    const sizeInitializedRef = useRef(false);

    useEffect(() => {
      if (!containerRef.current) return;

      const resizeObserver = new ResizeObserver(entries => {
        for (const entry of entries) {
          const { width, height } = entry.contentRect;
          if (width <= 0 || height <= 0) return;
          
          // Always update on first observation
          if (!sizeInitializedRef.current) {
            sizeInitializedRef.current = true;
            lastSizeRef.current = { width, height };
            setContainerSize({ width, height });
            return;
          }
          
          // Only respond to height changes, not width changes
          // Width changes are typically caused by sidebar animation
          const heightDiff = Math.abs(height - lastSizeRef.current.height);
          if (heightDiff < 1) return;
          
          lastSizeRef.current = { width: lastSizeRef.current.width, height };
          setContainerSize(prev => ({ ...prev, height }));
        }
      });

      resizeObserver.observe(containerRef.current);
      return () => resizeObserver.disconnect();
    }, []);

    const width = propWidth ?? containerSize.width;
    const height = propHeight ?? containerSize.height;

    // Get status color for a node
    const getNodeColor = useCallback((node: D3GraphNode): string => {
      return NODE_STATUS_COLORS[node.status] || NODE_STATUS_COLORS.Unknown;
    }, []);

    // Truncate name for display
    const truncateName = useCallback((name: string, maxLen: number = 24): string => {
      if (name.length <= maxLen) return name;
      return name.slice(0, maxLen - 1) + '…';
    }, []);

    // Create drag behavior
    const createDragBehavior = useCallback(() => {
      const simulation = simulationRef.current;
      if (!simulation) return null;

      return d3
        .drag<SVGGElement, D3GraphNode>()
        .on('start', (event, d) => {
          if (!event.active) simulation.alphaTarget(0.3).restart();
          d.fx = d.x;
          d.fy = d.y;
        })
        .on('drag', (event, d) => {
          d.fx = event.x;
          d.fy = event.y;
        })
        .on('end', (event, d) => {
          if (!event.active) simulation.alphaTarget(0);
          d.fx = null;
          d.fy = null;
        });
    }, []);

    // Render a node group (used for both initial render and incremental updates)
    const renderNodeGroup = useCallback(
      (
        nodeEnter: d3.Selection<d3.EnterElement, D3GraphNode, SVGGElement, unknown>
      ): d3.Selection<SVGGElement, D3GraphNode, SVGGElement, unknown> => {
        const g = nodeEnter
          .append('g')
          .attr('class', 'node')
          .attr('cursor', 'pointer')
          .on('click', (event, d) => {
            event.stopPropagation();
            onNodeClickRef.current?.(d);
          });

        // Card
        g.append('rect')
          .attr('class', 'node-card')
          .attr('x', -NODE_WIDTH / 2)
          .attr('y', -NODE_HEIGHT / 2)
          .attr('width', NODE_WIDTH)
          .attr('height', NODE_HEIGHT)
          .attr('rx', 8)
          .style('fill', 'var(--color-surface-elevated)')
          .style('stroke', 'var(--color-border-strong)')
          .attr('stroke-width', 1)
          .attr('stroke-dasharray', d => (d.isClusterScoped ? '4,3' : 'none'));

        // Selection ring (accent outline around the card)
        g.append('rect')
          .attr('class', 'selection-ring')
          .attr('x', -NODE_WIDTH / 2 - 3)
          .attr('y', -NODE_HEIGHT / 2 - 3)
          .attr('width', NODE_WIDTH + 6)
          .attr('height', NODE_HEIGHT + 6)
          .attr('rx', 10)
          .attr('fill', 'none')
          .style('stroke', 'var(--color-accent)')
          .attr('stroke-width', 1.5)
          .attr('opacity', d => (d.uid === selectedNodeIdRef.current ? 1 : 0));

        // Status dot
        g.append('circle')
          .attr('class', 'node-status')
          .attr('cx', -NODE_WIDTH / 2 + 14)
          .attr('cy', 0)
          .attr('r', 3.5)
          .style('fill', d => getNodeColor(d));

        // Kind label
        g.append('text')
          .attr('x', -NODE_WIDTH / 2 + 26)
          .attr('y', -4)
          .style('fill', 'var(--color-text-faint)')
          .attr('font-size', '10px')
          .text(d => d.kind);

        // Name label
        g.append('text')
          .attr('x', -NODE_WIDTH / 2 + 26)
          .attr('y', 11)
          .style('fill', 'var(--color-text-primary)')
          .attr('font-size', '11.5px')
          .attr('font-weight', 500)
          .text(d => truncateName(d.name, d.anomalyCount > 0 ? 23 : 28))
          .append('title')
          .text(d => `${d.kind}/${d.name}`);

        // Anomaly badge (only for nodes with anomalies)
        const badgeGroup = g
          .filter(d => d.anomalyCount > 0)
          .append('g')
          .attr('class', 'anomaly-badge');

        const badgeLabel = (d: D3GraphNode) => (d.anomalyCount > 9 ? '9+' : d.anomalyCount.toString());
        const badgeWidth = (d: D3GraphNode) => badgeLabel(d).length * 6 + 10;
        const isError = (d: D3GraphNode) => d.status === 'Error';

        badgeGroup
          .append('rect')
          .attr('x', d => NODE_WIDTH / 2 - 8 - badgeWidth(d))
          .attr('y', -8)
          .attr('width', badgeWidth)
          .attr('height', 16)
          .attr('rx', 4)
          .style('fill', d => (isError(d) ? 'var(--color-status-err-soft)' : 'var(--color-status-warn-soft)'));

        badgeGroup
          .append('text')
          .attr('x', d => NODE_WIDTH / 2 - 8 - badgeWidth(d) / 2)
          .attr('text-anchor', 'middle')
          .attr('dominant-baseline', 'central')
          .style('fill', d => (isError(d) ? 'var(--color-status-err-text)' : 'var(--color-status-warn-text)'))
          .attr('font-size', '10px')
          .attr('font-weight', 600)
          .text(badgeLabel);

        return g;
      },
      [getNodeColor, truncateName]
    );

    // Expose zoom controls via ref
    useImperativeHandle(
      ref,
      () => ({
        zoomIn: () => {
          if (!svgRef.current || !zoomRef.current) return;
          const svg = d3.select(svgRef.current);
          svg.transition().duration(300).call(zoomRef.current.scaleBy, ZOOM_SCALE_FACTOR);
        },
        zoomOut: () => {
          if (!svgRef.current || !zoomRef.current) return;
          const svg = d3.select(svgRef.current);
          svg.transition().duration(300).call(zoomRef.current.scaleBy, 1 / ZOOM_SCALE_FACTOR);
        },
        fitToView: () => {
          if (!svgRef.current || !zoomRef.current || !simulationRef.current) return;
          const svg = d3.select(svgRef.current);

          // Get bounds of all nodes
          const simNodes = simulationRef.current.nodes();
          if (simNodes.length === 0) return;

          let minX = Infinity,
            maxX = -Infinity;
          let minY = Infinity,
            maxY = -Infinity;

          simNodes.forEach(node => {
            const x = node.x ?? 0;
            const y = node.y ?? 0;
            minX = Math.min(minX, x);
            maxX = Math.max(maxX, x);
            minY = Math.min(minY, y);
            maxY = Math.max(maxY, y);
          });

          // Add padding for node size and labels
          const padding = FIT_PADDING;
          minX -= padding;
          maxX += padding;
          minY -= padding;
          maxY += padding;

          const graphWidth = maxX - minX;
          const graphHeight = maxY - minY;

          // Calculate scale to fit
          const scale =
            Math.min(
              width / graphWidth,
              height / graphHeight,
              1.5 // Max zoom for fit
            ) * 0.9; // Leave some margin

          // Calculate translation to center
          const centerX = (minX + maxX) / 2;
          const centerY = (minY + maxY) / 2;
          const translateX = width / 2 - centerX * scale;
          const translateY = height / 2 - centerY * scale;

          const transform = d3.zoomIdentity.translate(translateX, translateY).scale(scale);

          svg.transition().duration(500).call(zoomRef.current.transform, transform);
        },
        resetZoom: () => {
          if (!svgRef.current || !zoomRef.current) return;
          const svg = d3.select(svgRef.current);
          const initialScale = 0.8;
          const initialTransform = d3.zoomIdentity
            .translate((width * (1 - initialScale)) / 2, (height * (1 - initialScale)) / 2)
            .scale(initialScale);
          svg.transition().duration(500).call(zoomRef.current.transform, initialTransform);
        },
        focusOnNodes: (nodeUids: Set<string>) => {
          if (!svgRef.current || !zoomRef.current || !simulationRef.current) return;
          const svg = d3.select(svgRef.current);

          // Get bounds of specified nodes only
          const simNodes = simulationRef.current.nodes();
          const targetNodes = simNodes.filter(n => nodeUids.has(n.uid));
          if (targetNodes.length === 0) return;

          let minX = Infinity,
            maxX = -Infinity;
          let minY = Infinity,
            maxY = -Infinity;

          targetNodes.forEach(node => {
            const x = node.x ?? 0;
            const y = node.y ?? 0;
            minX = Math.min(minX, x);
            maxX = Math.max(maxX, x);
            minY = Math.min(minY, y);
            maxY = Math.max(maxY, y);
          });

          // Add padding for node size and labels
          const padding = FIT_PADDING * 1.3;
          minX -= padding;
          maxX += padding;
          minY -= padding;
          maxY += padding;

          const graphWidth = maxX - minX;
          const graphHeight = maxY - minY;

          // Calculate scale to fit the subgraph
          const scale =
            Math.min(
              width / graphWidth,
              height / graphHeight,
              1.5 // Max zoom for fit
            ) * 0.85; // Leave some margin

          // Calculate translation to center
          const centerX = (minX + maxX) / 2;
          const centerY = (minY + maxY) / 2;
          const translateX = width / 2 - centerX * scale;
          const translateY = height / 2 - centerY * scale;

          const transform = d3.zoomIdentity.translate(translateX, translateY).scale(scale);

          svg.transition().duration(500).call(zoomRef.current.transform, transform);
        },
      }),
      [width, height]
    );

    // Main D3 rendering effect - handles both initial render and incremental updates
    useEffect(() => {
      if (!svgRef.current || width === 0 || height === 0) return;

      const svg = d3.select(svgRef.current);
      const currentNodeUids = new Set(nodes.map(n => n.uid));
      const prevNodeUids = prevNodeUidsRef.current;

      // Detect if this is a reset (namespace changed, etc.) or incremental update
      const isReset = !isInitializedRef.current || prevNodeUids.size === 0;
      const hasNewNodes = !isReset && nodes.some(n => !prevNodeUids.has(n.uid));

      // Update ref for next comparison
      prevNodeUidsRef.current = currentNodeUids;

      if (nodes.length === 0) {
        // Clear graph if no nodes
        svg.selectAll('.graph-container').remove();
        isInitializedRef.current = false;
        simulationRef.current = null;
        return;
      }

      if (isReset) {
        // Full initialization on first render or reset
        svg.selectAll('*').remove();

        // Create defs for filters and markers
        const defs = svg.append('defs');

        // Arrow markers for edges (default and causal-path accent)
        [
          { id: 'arrow', color: 'var(--color-text-faint)' },
          { id: 'arrow-accent', color: 'var(--color-accent)' },
        ].forEach(({ id, color }) => {
          defs
            .append('marker')
            .attr('id', id)
            .attr('viewBox', '0 -5 10 10')
            .attr('refX', 9)
            .attr('refY', 0)
            .attr('markerWidth', 6)
            .attr('markerHeight', 6)
            .attr('orient', 'auto')
            .append('path')
            .attr('d', 'M0,-5L10,0L0,5')
            .style('fill', color);
        });

        // Create container group for zoom/pan
        const container = svg.append('g').attr('class', 'graph-container');

        // Setup zoom behavior
        const zoom = d3
          .zoom<SVGSVGElement, unknown>()
          .scaleExtent([0.1, 4])
          .on('zoom', event => {
            container.attr('transform', event.transform);
          });

        zoomRef.current = zoom;
        svg.call(zoom);

        // Create a copy of nodes for the simulation (D3 mutates these)
        const simulationNodes: D3GraphNode[] = nodes.map(n => ({ ...n }));

        // Create links with references to simulation nodes
        const simulationLinks: D3GraphLink[] = links.map(l => ({
          ...l,
          source: l.source,
          target: l.target,
        }));

        // Create force simulation
        const simulation = d3
          .forceSimulation<D3GraphNode>(simulationNodes)
          .force('charge', d3.forceManyBody<D3GraphNode>().strength(-800))
          .force('center', d3.forceCenter(width / 2, height / 2))
          .force('collision', d3.forceCollide<D3GraphNode>().radius(COLLISION_RADIUS))
          .force(
            'link',
            d3
              .forceLink<D3GraphNode, D3GraphLink>(simulationLinks)
              .id(d => d.uid)
              .distance(150)
              .strength(0.3)
          );

        // Run simulation to completion synchronously (skip animation, show final state)
        simulation.stop();
        for (let i = 0; i < 300; i++) {
          simulation.tick();
        }

        simulationRef.current = simulation;

        // Create links (edges)
        const linkGroup = container.append('g').attr('class', 'links');
        const link = linkGroup
          .selectAll('line')
          .data(simulationLinks, (d: D3GraphLink) => d.id)
          .join('line')
          .style('stroke', 'var(--color-text-faint)')
          .attr('stroke-width', 1.25)
          .attr('stroke-opacity', 0.6)
          .attr('marker-end', 'url(#arrow)');

        // Create node groups
        const nodeGroup = container.append('g').attr('class', 'nodes');
        const nodeSelection = nodeGroup
          .selectAll<SVGGElement, D3GraphNode>('g')
          .data(simulationNodes, (d: D3GraphNode) => d.uid)
          .join(enter => renderNodeGroup(enter));

        // Apply drag behavior
        const drag = createDragBehavior();
        if (drag) {
          nodeSelection.call(drag);
        }

        // Simulation tick handler (for drag interactions)
        simulation.on('tick', () => {
          positionLinks(link);

          nodeSelection.attr('transform', d => `translate(${d.x ?? 0}, ${d.y ?? 0})`);
        });

        // Set initial positions from pre-computed simulation
        positionLinks(link);

        nodeSelection.attr('transform', d => `translate(${d.x ?? 0}, ${d.y ?? 0})`);

        // Fit to view on initial load (no animation)
        if (simulationNodes.length > 0) {
          let minX = Infinity,
            maxX = -Infinity;
          let minY = Infinity,
            maxY = -Infinity;

          simulationNodes.forEach(n => {
            const x = n.x ?? 0;
            const y = n.y ?? 0;
            minX = Math.min(minX, x);
            maxX = Math.max(maxX, x);
            minY = Math.min(minY, y);
            maxY = Math.max(maxY, y);
          });

          // Add padding for node size and labels
          const padding = FIT_PADDING;
          minX -= padding;
          maxX += padding;
          minY -= padding;
          maxY += padding;

          const graphWidth = maxX - minX;
          const graphHeight = maxY - minY;

          // Calculate scale to fit
          const scale =
            Math.min(
              width / graphWidth,
              height / graphHeight,
              1.5 // Max zoom for fit
            ) * 0.9; // Leave some margin

          // Calculate translation to center
          const centerX = (minX + maxX) / 2;
          const centerY = (minY + maxY) / 2;
          const translateX = width / 2 - centerX * scale;
          const translateY = height / 2 - centerY * scale;

          const initialTransform = d3.zoomIdentity.translate(translateX, translateY).scale(scale);

          svg.call(zoom.transform, initialTransform);
        }

        isInitializedRef.current = true;

        // Cleanup
        return () => {
          simulation.stop();
        };
      } else if (hasNewNodes) {
        // Incremental update - add new nodes/links without rebuilding
        const simulation = simulationRef.current;
        if (!simulation) return;

        const container = svg.select<SVGGElement>('.graph-container');
        const linkGroup = container.select<SVGGElement>('.links');
        const nodeGroup = container.select<SVGGElement>('.nodes');

        // Get existing simulation nodes and their positions
        const existingSimNodes = simulation.nodes();
        const existingNodeMap = new Map<string, D3GraphNode>();
        for (const node of existingSimNodes) {
          existingNodeMap.set(node.uid, node);
        }

        // Build new simulation nodes array, preserving positions for existing nodes
        const newSimulationNodes: D3GraphNode[] = nodes.map(n => {
          const existing = existingNodeMap.get(n.uid);
          if (existing) {
            // Preserve position and velocity for existing nodes
            return {
              ...n,
              x: existing.x,
              y: existing.y,
              vx: existing.vx,
              vy: existing.vy,
              fx: existing.fx,
              fy: existing.fy,
            };
          } else {
            // New node - position near center with some randomness
            return {
              ...n,
              x: width / 2 + (Math.random() - 0.5) * 200,
              y: height / 2 + (Math.random() - 0.5) * 200,
            };
          }
        });

        // Build new links array
        const newSimulationLinks: D3GraphLink[] = links.map(l => ({
          ...l,
          source: l.source,
          target: l.target,
        }));

        // Update simulation with new nodes and links
        simulation.nodes(newSimulationNodes);

        const linkForce = simulation.force('link') as d3.ForceLink<D3GraphNode, D3GraphLink>;
        if (linkForce) {
          linkForce.links(newSimulationLinks);
        }

        // Update links using data join with key function
        linkGroup
          .selectAll<SVGLineElement, D3GraphLink>('line')
          .data(newSimulationLinks, d => d.id)
          .join(
            enter =>
              enter
                .append('line')
                .style('stroke', 'var(--color-text-faint)')
                .attr('stroke-width', 1.25)
                .attr('stroke-opacity', 0)
                .attr('marker-end', 'url(#arrow)')
                .call(enter => enter.transition().duration(300).attr('stroke-opacity', 0.6)),
            update => update,
            exit => exit.transition().duration(200).attr('stroke-opacity', 0).remove()
          );

        // Update nodes using data join with key function
        const nodeSelection = nodeGroup
          .selectAll<SVGGElement, D3GraphNode>('g.node')
          .data(newSimulationNodes, d => d.uid)
          .join(
            enter => {
              const g = renderNodeGroup(enter);
              // Fade in new nodes
              g.attr('opacity', 0).transition().duration(300).attr('opacity', 1);
              return g;
            },
            update => update,
            exit => exit.transition().duration(200).attr('opacity', 0).remove()
          );

        // Apply drag behavior to all nodes (including new ones)
        const drag = createDragBehavior();
        if (drag) {
          nodeSelection.call(drag);
        }

        // Update tick handler with new selections
        simulation.on('tick', () => {
          positionLinks(linkGroup.selectAll<SVGLineElement, D3GraphLink>('line'));

          nodeGroup.selectAll<SVGGElement, D3GraphNode>('g.node').attr('transform', d => `translate(${d.x ?? 0}, ${d.y ?? 0})`);
        });

        // Gently reheat simulation to integrate new nodes (not full restart)
        simulation.alpha(0.3).restart();

        // Set initial positions for new elements
        positionLinks(linkGroup.selectAll<SVGLineElement, D3GraphLink>('line'));

        nodeGroup.selectAll<SVGGElement, D3GraphNode>('g.node').attr('transform', d => `translate(${d.x ?? 0}, ${d.y ?? 0})`);
      }

    }, [nodes, links, width, height, renderNodeGroup, createDragBehavior]);

    // Update selection ring when selectedNodeId changes
    useEffect(() => {
      if (!svgRef.current) return;

      d3.select(svgRef.current)
        .selectAll<SVGCircleElement, D3GraphNode>('.selection-ring')
        .attr('opacity', d => (d.uid === selectedNodeId ? 1 : 0));
    }, [selectedNodeId]);

    // Update node/link opacity when highlightedNodeUids changes (for causal path highlighting)
    useEffect(() => {
      if (!svgRef.current) return;

      const svg = d3.select(svgRef.current);
      const highlighted = highlightedNodeUids;

      // Update node group opacity
      svg
        .selectAll<SVGGElement, D3GraphNode>('.node')
        .transition()
        .duration(300)
        .attr('opacity', d => {
          if (!highlighted) return 1;
          return highlighted.has(d.uid) ? 1 : 0.3;
        });

      // Causal path nodes get an accent border
      svg
        .selectAll<SVGRectElement, D3GraphNode>('.node-card')
        .style('stroke', d => (highlighted?.has(d.uid) ? 'var(--color-accent-line)' : 'var(--color-border-strong)'));

      const isPathLink = (d: D3GraphLink) => {
        if (!highlighted) return false;
        const sourceUid = typeof d.source === 'string' ? d.source : (d.source as D3GraphNode).uid;
        const targetUid = typeof d.target === 'string' ? d.target : (d.target as D3GraphNode).uid;
        return highlighted.has(sourceUid) && highlighted.has(targetUid);
      };

      svg
        .selectAll<SVGLineElement, D3GraphLink>('line')
        .style('stroke', d => (isPathLink(d) ? 'var(--color-accent)' : 'var(--color-text-faint)'))
        .attr('marker-end', d => (isPathLink(d) ? 'url(#arrow-accent)' : 'url(#arrow)'));

      // Update link opacity
      svg
        .selectAll<SVGLineElement, D3GraphLink>('line')
        .transition()
        .duration(300)
        .attr('stroke-opacity', d => {
          if (!highlighted) return 0.6;
          return isPathLink(d) ? 1 : 0.12;
        })
        .attr('stroke-width', d => {
          if (!highlighted) return 1.25;
          return isPathLink(d) ? 1.75 : 1;
        });
    }, [highlightedNodeUids]);

    return (
      <div
        ref={containerRef}
        className="w-full h-full relative overflow-hidden"
        style={{
          minHeight: 400,
          backgroundColor: 'var(--color-app-bg)',
          backgroundImage: 'radial-gradient(circle at 1px 1px, var(--color-border-soft) 1px, transparent 0)',
          backgroundSize: '22px 22px',
        }}
      >
        <svg ref={svgRef} className="block w-full h-full" style={{ position: 'absolute', top: 0, left: 0 }} />

        {/* Empty state */}
        {nodes.length === 0 && (
          <div className="absolute inset-0 flex items-center justify-center text-[var(--color-text-faint)]">
            <div className="text-center">
              <svg className="w-10 h-10 mx-auto mb-3 opacity-60" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={1.5}
                  d="M9 17V7m0 10a2 2 0 01-2 2H5a2 2 0 01-2-2V7a2 2 0 012-2h2a2 2 0 012 2m0 10a2 2 0 002 2h2a2 2 0 002-2M9 7a2 2 0 012-2h2a2 2 0 012 2m0 10V7m0 10a2 2 0 002 2h2a2 2 0 002-2V7a2 2 0 00-2-2h-2a2 2 0 00-2 2"
                />
              </svg>
              <p className="text-[12.5px]">No resources found in this namespace</p>
            </div>
          </div>
        )}
      </div>
    );
  }
);

// Display name for debugging
NamespaceGraph.displayName = 'NamespaceGraph';

export default NamespaceGraph;
