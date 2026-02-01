import React, { useEffect, useRef, useMemo, useCallback, useImperativeHandle, forwardRef } from 'react';
import * as d3 from 'd3';
import {
  ObservatoryGraphResponse,
  D3ObservatoryNode,
  D3ObservatoryLink,
  transformToD3Graph,
  NODE_TYPE_COLORS,
  EDGE_TYPE_COLORS,
  ObservatoryNodeType,
} from '../../types/observatoryGraph';

interface ObservatoryGraphProps {
  /** Graph data from API */
  data: ObservatoryGraphResponse;
  /** Callback when a node is clicked */
  onNodeClick?: (node: D3ObservatoryNode) => void;
  /** Currently selected node ID */
  selectedNodeId?: string | null;
  /** Width of the container (optional, uses container size if not provided) */
  width?: number;
  /** Height of the container (optional, uses container size if not provided) */
  height?: number;
}

/** Imperative handle for controlling zoom from parent */
export interface ObservatoryGraphHandle {
  zoomIn: () => void;
  zoomOut: () => void;
  fitToView: () => void;
  resetZoom: () => void;
}

// Node radius by type
const NODE_RADIUS: Record<ObservatoryNodeType, number> = {
  SignalAnchor: 28,
  SignalBaseline: 22,
  Alert: 26,
  Dashboard: 30,
  Panel: 22,
  Query: 20,
  Metric: 24,
  Service: 26,
  Workload: 26,
};

// Default node radius
const DEFAULT_NODE_RADIUS = 24;
// Collision radius multiplier
const COLLISION_MULTIPLIER = 2.5;
// Zoom scale factor for zoom in/out buttons
const ZOOM_SCALE_FACTOR = 1.3;

/**
 * Force-directed graph visualization for Observatory data
 *
 * Features:
 * - D3 force simulation with repulsion, centering, and collision
 * - Pan and zoom support
 * - Draggable nodes
 * - Type-based coloring for nodes and edges
 * - Node type labels
 */
export const ObservatoryGraph = forwardRef<ObservatoryGraphHandle, ObservatoryGraphProps>(
  ({ data, onNodeClick, selectedNodeId, width: propWidth, height: propHeight }, ref) => {
    const containerRef = useRef<HTMLDivElement>(null);
    const svgRef = useRef<SVGSVGElement>(null);
    const simulationRef = useRef<d3.Simulation<D3ObservatoryNode, D3ObservatoryLink> | null>(null);
    const zoomRef = useRef<d3.ZoomBehavior<SVGSVGElement, unknown> | null>(null);

    // Track the minimum zoom scale (set by fitToView)
    const minScaleRef = useRef<number>(0.1);

    // Track if the graph has been initialized
    const isInitializedRef = useRef(false);

    // Track selectedNodeId in a ref to avoid re-rendering the entire graph
    const selectedNodeIdRef = useRef<string | null | undefined>(selectedNodeId);
    selectedNodeIdRef.current = selectedNodeId;

    // Track onNodeClick in a ref to avoid re-rendering when callback changes
    const onNodeClickRef = useRef(onNodeClick);
    onNodeClickRef.current = onNodeClick;

    // Transform API data to D3 format
    const { nodes, links } = useMemo(() => transformToD3Graph(data), [data]);

    // Get container dimensions
    const [containerSize, setContainerSize] = React.useState({ width: 800, height: 600 });
    const sizeInitializedRef = useRef(false);

    useEffect(() => {
      if (!containerRef.current) return;

      const resizeObserver = new ResizeObserver(entries => {
        for (const entry of entries) {
          const { width, height } = entry.contentRect;
          if (width <= 0 || height <= 0) return;

          if (!sizeInitializedRef.current) {
            sizeInitializedRef.current = true;
            setContainerSize({ width, height });
            return;
          }

          setContainerSize({ width, height });
        }
      });

      resizeObserver.observe(containerRef.current);
      return () => resizeObserver.disconnect();
    }, []);

    const width = propWidth ?? containerSize.width;
    const height = propHeight ?? containerSize.height;

    // Get node radius by type
    const getNodeRadius = useCallback((node: D3ObservatoryNode): number => {
      return NODE_RADIUS[node.type] || DEFAULT_NODE_RADIUS;
    }, []);

    // Get node color by type
    const getNodeColor = useCallback((node: D3ObservatoryNode): string => {
      return NODE_TYPE_COLORS[node.type] || '#6b7280';
    }, []);

    // Truncate label for display
    const truncateLabel = useCallback((label: string, maxLen: number = 25): string => {
      if (label.length <= maxLen) return label;
      return label.slice(0, maxLen - 3) + '...';
    }, []);

    // Create drag behavior
    const createDragBehavior = useCallback(() => {
      const simulation = simulationRef.current;
      if (!simulation) return null;

      return d3
        .drag<SVGGElement, D3ObservatoryNode>()
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

    // Render a node group
    const renderNodeGroup = useCallback(
      (
        nodeEnter: d3.Selection<d3.EnterElement, D3ObservatoryNode, SVGGElement, unknown>
      ): d3.Selection<SVGGElement, D3ObservatoryNode, SVGGElement, unknown> => {
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
          .attr('r', d => getNodeRadius(d))
          .attr('fill', d => getNodeColor(d))
          .attr('stroke', '#1f2937')
          .attr('stroke-width', 2)
          .attr('opacity', 0.9);

        // Selection ring
        g.append('circle')
          .attr('r', d => getNodeRadius(d) + 4)
          .attr('fill', 'none')
          .attr('stroke', '#3b82f6')
          .attr('stroke-width', 2)
          .attr('opacity', d => (d.id === selectedNodeIdRef.current ? 1 : 0))
          .attr('class', 'selection-ring');

        // Type label (above node)
        g.append('text')
          .attr('y', d => -getNodeRadius(d) - 8)
          .attr('text-anchor', 'middle')
          .attr('fill', '#9ca3af')
          .attr('font-size', '9px')
          .attr('font-weight', 'bold')
          .text(d => d.type);

        // Name label (below node)
        g.append('text')
          .attr('y', d => getNodeRadius(d) + 14)
          .attr('text-anchor', 'middle')
          .attr('fill', '#f8fafc')
          .attr('font-size', '10px')
          .text(d => truncateLabel(d.label));

        return g;
      },
      [getNodeRadius, getNodeColor, truncateLabel]
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

          const padding = 80;
          minX -= padding;
          maxX += padding;
          minY -= padding;
          maxY += padding;

          const graphWidth = maxX - minX;
          const graphHeight = maxY - minY;

          const scale =
            Math.min(
              width / graphWidth,
              height / graphHeight,
              1.5
            ) * 0.9;

          // Update the minimum scale to the fit-to-view scale
          // This prevents zoom out beyond fit-to-view and fixes the jump issue
          minScaleRef.current = scale;
          zoomRef.current.scaleExtent([scale, 4]);

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
      }),
      [width, height]
    );

    // Main D3 rendering effect
    useEffect(() => {
      if (!svgRef.current || nodes.length === 0) return;

      const svg = d3.select(svgRef.current);

      // Clear previous content on full rebuild
      if (!isInitializedRef.current) {
        svg.selectAll('*').remove();

        // Add definitions for filters
        const defs = svg.append('defs');

        // Glow filter for alerts
        const filter = defs
          .append('filter')
          .attr('id', 'glow-alert')
          .attr('x', '-50%')
          .attr('y', '-50%')
          .attr('width', '200%')
          .attr('height', '200%');

        filter
          .append('feGaussianBlur')
          .attr('stdDeviation', '3')
          .attr('result', 'coloredBlur');

        const feMerge = filter.append('feMerge');
        feMerge.append('feMergeNode').attr('in', 'coloredBlur');
        feMerge.append('feMergeNode').attr('in', 'SourceGraphic');

        // Arrow marker for edges
        defs
          .append('marker')
          .attr('id', 'arrowhead')
          .attr('viewBox', '0 -5 10 10')
          .attr('refX', 15)
          .attr('refY', 0)
          .attr('markerWidth', 6)
          .attr('markerHeight', 6)
          .attr('orient', 'auto')
          .append('path')
          .attr('d', 'M0,-5L10,0L0,5')
          .attr('fill', '#6b7280');
      }

      // Create main group for zoom/pan
      let g = svg.select<SVGGElement>('g.main-group');
      if (g.empty()) {
        g = svg.append('g').attr('class', 'main-group');
      }

      // Create link group
      let linkGroup = g.select<SVGGElement>('g.links');
      if (linkGroup.empty()) {
        linkGroup = g.append('g').attr('class', 'links');
      }

      // Create node group
      let nodeGroup = g.select<SVGGElement>('g.nodes');
      if (nodeGroup.empty()) {
        nodeGroup = g.append('g').attr('class', 'nodes');
      }

      // Setup zoom behavior
      if (!zoomRef.current) {
        const zoom = d3
          .zoom<SVGSVGElement, unknown>()
          .scaleExtent([0.1, 4])
          .on('zoom', event => {
            g.attr('transform', event.transform);
          });

        svg.call(zoom);
        zoomRef.current = zoom;

        // Set initial zoom
        const initialScale = 0.8;
        const initialTransform = d3.zoomIdentity
          .translate((width * (1 - initialScale)) / 2, (height * (1 - initialScale)) / 2)
          .scale(initialScale);
        svg.call(zoom.transform, initialTransform);
      }

      // Click on background to deselect
      svg.on('click', () => {
        onNodeClickRef.current?.(null as any);
      });

      // Create force simulation (matching NamespaceGraph parameters for consistent feel)
      const simulation = d3
        .forceSimulation<D3ObservatoryNode>(nodes)
        .force('charge', d3.forceManyBody<D3ObservatoryNode>().strength(-800))
        .force('center', d3.forceCenter(width / 2, height / 2))
        .force(
          'collision',
          d3.forceCollide<D3ObservatoryNode>().radius(d => getNodeRadius(d) * COLLISION_MULTIPLIER)
        )
        .force(
          'link',
          d3
            .forceLink<D3ObservatoryNode, D3ObservatoryLink>(links)
            .id(d => d.id)
            .distance(150)
            .strength(0.3)
        );

      simulationRef.current = simulation;

      // Pre-run simulation for instant rendering
      for (let i = 0; i < 300; i++) {
        simulation.tick();
      }

      // Render links
      const linkSelection = linkGroup
        .selectAll<SVGLineElement, D3ObservatoryLink>('line')
        .data(links, d => d.id);

      linkSelection.exit().remove();

      const linkEnter = linkSelection
        .enter()
        .append('line')
        .attr('stroke', d => EDGE_TYPE_COLORS[d.relationshipType] || '#6b7280')
        .attr('stroke-width', 1.5)
        .attr('stroke-opacity', 0.6)
        .attr('marker-end', 'url(#arrowhead)');

      const allLinks = linkEnter.merge(linkSelection);

      // Render nodes
      const nodeSelection = nodeGroup
        .selectAll<SVGGElement, D3ObservatoryNode>('g.node')
        .data(nodes, d => d.id);

      nodeSelection.exit().remove();

      const nodeEnter = renderNodeGroup(nodeSelection.enter());
      const allNodes = nodeEnter.merge(nodeSelection);

      // Apply drag behavior to all nodes (not just newly entered ones)
      const drag = createDragBehavior();
      if (drag) {
        allNodes.call(drag);
      }

      // Update positions
      simulation.on('tick', () => {
        allLinks
          .attr('x1', d => (d.source as D3ObservatoryNode).x ?? 0)
          .attr('y1', d => (d.source as D3ObservatoryNode).y ?? 0)
          .attr('x2', d => (d.target as D3ObservatoryNode).x ?? 0)
          .attr('y2', d => (d.target as D3ObservatoryNode).y ?? 0);

        allNodes.attr('transform', d => `translate(${d.x ?? 0},${d.y ?? 0})`);
      });

      // Stop simulation after initial layout
      simulation.alphaTarget(0);

      isInitializedRef.current = true;

      return () => {
        simulation.stop();
      };
    }, [nodes, links, width, height, getNodeRadius, renderNodeGroup, createDragBehavior]);

    // Update selection ring when selectedNodeId changes
    useEffect(() => {
      if (!svgRef.current) return;

      const svg = d3.select(svgRef.current);
      svg.selectAll<SVGCircleElement, D3ObservatoryNode>('.selection-ring')
        .attr('opacity', d => (d.id === selectedNodeId ? 1 : 0));
    }, [selectedNodeId]);

    return (
      <div ref={containerRef} className="w-full h-full bg-[#111111]">
        <svg
          ref={svgRef}
          width={width}
          height={height}
          className="w-full h-full"
        />
      </div>
    );
  }
);

ObservatoryGraph.displayName = 'ObservatoryGraph';

export default ObservatoryGraph;
