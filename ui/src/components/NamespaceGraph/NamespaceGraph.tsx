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

// Node radius
const NODE_RADIUS = 24;
// Collision radius (larger to prevent overlap and allow space for labels)
const COLLISION_RADIUS = 60;
// Zoom scale factor for zoom in/out buttons
const ZOOM_SCALE_FACTOR = 1.3;

/**
 * Force-directed graph visualization for Kubernetes namespace resources
 *
 * Features:
 * - D3 force simulation with repulsion, centering, and collision
 * - Pan and zoom support
 * - Draggable nodes
 * - Status-based coloring with glow effect for errors
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
    const truncateName = useCallback((name: string, maxLen: number = 20): string => {
      if (name.length <= maxLen) return name;
      return name.slice(0, maxLen - 3) + '...';
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

        // Node circle
        g.append('circle')
          .attr('r', NODE_RADIUS)
          .attr('fill', d => getNodeColor(d))
          .attr('stroke', d => (d.isClusterScoped ? '#9ca3af' : 'none'))
          .attr('stroke-width', d => (d.isClusterScoped ? 2 : 0))
          .attr('stroke-dasharray', d => (d.isClusterScoped ? '4,2' : 'none'))
          .attr('filter', d => (d.status === 'Error' ? 'url(#glow-error)' : 'none'));

        // Selection ring
        g.append('circle')
          .attr('r', NODE_RADIUS + 4)
          .attr('fill', 'none')
          .attr('stroke', '#3b82f6')
          .attr('stroke-width', 2)
          .attr('opacity', d => (d.uid === selectedNodeIdRef.current ? 1 : 0))
          .attr('class', 'selection-ring');

        // Kind label (above node)
        g.append('text')
          .attr('y', -NODE_RADIUS - 8)
          .attr('text-anchor', 'middle')
          .attr('fill', '#9ca3af')
          .attr('font-size', '10px')
          .attr('font-weight', 'bold')
          .text(d => d.kind);

        // Name label (below node)
        g.append('text')
          .attr('y', NODE_RADIUS + 16)
          .attr('text-anchor', 'middle')
          .attr('fill', '#f8fafc')
          .attr('font-size', '11px')
          .text(d => truncateName(d.name));

        // Anomaly badge (only for nodes with anomalies)
        const badgeGroup = g
          .filter(d => d.anomalyCount > 0)
          .append('g')
          .attr('class', 'anomaly-badge')
          .attr('transform', `translate(${NODE_RADIUS - 6}, ${-NODE_RADIUS + 6})`);

        badgeGroup
          .append('circle')
          .attr('r', 10)
          .attr('fill', '#ef4444')
          .attr('stroke', '#1f2937')
          .attr('stroke-width', 2);

        badgeGroup
          .append('text')
          .attr('text-anchor', 'middle')
          .attr('dominant-baseline', 'central')
          .attr('fill', 'white')
          .attr('font-size', '9px')
          .attr('font-weight', 'bold')
          .text(d => (d.anomalyCount > 9 ? '9+' : d.anomalyCount.toString()));

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
          const padding = NODE_RADIUS * 3;
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
          const padding = NODE_RADIUS * 4;
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

        // Error glow filter
        const glowFilter = defs
          .append('filter')
          .attr('id', 'glow-error')
          .attr('x', '-50%')
          .attr('y', '-50%')
          .attr('width', '200%')
          .attr('height', '200%');

        glowFilter.append('feGaussianBlur').attr('stdDeviation', '4').attr('result', 'blur');

        glowFilter.append('feFlood').attr('flood-color', '#ef4444').attr('flood-opacity', '0.6').attr('result', 'color');

        glowFilter.append('feComposite').attr('in', 'color').attr('in2', 'blur').attr('operator', 'in').attr('result', 'coloredBlur');

        const glowMerge = glowFilter.append('feMerge');
        glowMerge.append('feMergeNode').attr('in', 'coloredBlur');
        glowMerge.append('feMergeNode').attr('in', 'SourceGraphic');

        // Arrow marker for edges
        defs
          .append('marker')
          .attr('id', 'arrow')
          .attr('viewBox', '0 -5 10 10')
          .attr('refX', NODE_RADIUS + 8)
          .attr('refY', 0)
          .attr('markerWidth', 6)
          .attr('markerHeight', 6)
          .attr('orient', 'auto')
          .append('path')
          .attr('d', 'M0,-5L10,0L0,5')
          .attr('fill', '#6b7280');

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
          .attr('stroke', '#6b7280')
          .attr('stroke-width', 1.5)
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
          link
            .attr('x1', d => (d.source as D3GraphNode).x ?? 0)
            .attr('y1', d => (d.source as D3GraphNode).y ?? 0)
            .attr('x2', d => (d.target as D3GraphNode).x ?? 0)
            .attr('y2', d => (d.target as D3GraphNode).y ?? 0);

          nodeSelection.attr('transform', d => `translate(${d.x ?? 0}, ${d.y ?? 0})`);
        });

        // Set initial positions from pre-computed simulation
        link
          .attr('x1', d => (d.source as D3GraphNode).x ?? 0)
          .attr('y1', d => (d.source as D3GraphNode).y ?? 0)
          .attr('x2', d => (d.target as D3GraphNode).x ?? 0)
          .attr('y2', d => (d.target as D3GraphNode).y ?? 0);

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
          const padding = NODE_RADIUS * 3;
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
                .attr('stroke', '#6b7280')
                .attr('stroke-width', 1.5)
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
          linkGroup
            .selectAll<SVGLineElement, D3GraphLink>('line')
            .attr('x1', d => (d.source as D3GraphNode).x ?? 0)
            .attr('y1', d => (d.source as D3GraphNode).y ?? 0)
            .attr('x2', d => (d.target as D3GraphNode).x ?? 0)
            .attr('y2', d => (d.target as D3GraphNode).y ?? 0);

          nodeGroup.selectAll<SVGGElement, D3GraphNode>('g.node').attr('transform', d => `translate(${d.x ?? 0}, ${d.y ?? 0})`);
        });

        // Gently reheat simulation to integrate new nodes (not full restart)
        simulation.alpha(0.3).restart();

        // Set initial positions for new elements
        linkGroup
          .selectAll<SVGLineElement, D3GraphLink>('line')
          .attr('x1', d => (d.source as D3GraphNode).x ?? 0)
          .attr('y1', d => (d.source as D3GraphNode).y ?? 0)
          .attr('x2', d => (d.target as D3GraphNode).x ?? 0)
          .attr('y2', d => (d.target as D3GraphNode).y ?? 0);

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
          return highlighted.has(d.uid) ? 1 : 0.15;
        });

      // Update link opacity
      svg
        .selectAll<SVGLineElement, D3GraphLink>('line')
        .transition()
        .duration(300)
        .attr('stroke-opacity', d => {
          if (!highlighted) return 0.6;
          const sourceUid = typeof d.source === 'string' ? d.source : (d.source as D3GraphNode).uid;
          const targetUid = typeof d.target === 'string' ? d.target : (d.target as D3GraphNode).uid;
          return highlighted.has(sourceUid) && highlighted.has(targetUid) ? 0.9 : 0.05;
        })
        .attr('stroke-width', d => {
          if (!highlighted) return 1.5;
          const sourceUid = typeof d.source === 'string' ? d.source : (d.source as D3GraphNode).uid;
          const targetUid = typeof d.target === 'string' ? d.target : (d.target as D3GraphNode).uid;
          return highlighted.has(sourceUid) && highlighted.has(targetUid) ? 2.5 : 1;
        });
    }, [highlightedNodeUids]);

    return (
      <div ref={containerRef} className="w-full h-full bg-[var(--color-app-bg)] relative overflow-hidden" style={{ minHeight: 400 }}>
        <svg ref={svgRef} className="block w-full h-full" style={{ position: 'absolute', top: 0, left: 0 }} />

        {/* Empty state */}
        {nodes.length === 0 && (
          <div className="absolute inset-0 flex items-center justify-center text-[var(--color-text-muted)]">
            <div className="text-center">
              <svg className="w-16 h-16 mx-auto mb-4 opacity-50" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={1.5}
                  d="M9 17V7m0 10a2 2 0 01-2 2H5a2 2 0 01-2-2V7a2 2 0 012-2h2a2 2 0 012 2m0 10a2 2 0 002 2h2a2 2 0 002-2M9 7a2 2 0 012-2h2a2 2 0 012 2m0 10V7m0 10a2 2 0 002 2h2a2 2 0 002-2V7a2 2 0 00-2-2h-2a2 2 0 00-2 2"
                />
              </svg>
              <p>No resources found in this namespace</p>
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
