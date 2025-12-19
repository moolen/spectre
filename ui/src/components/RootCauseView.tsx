import React, { useEffect, useRef, useState, useMemo } from 'react';
import { RootCauseAnalysisV2, GraphNode, ChangeEventInfo, K8sEventInfo } from '../types/rootCause';
import * as d3 from 'd3';
import { useSettings } from '../hooks/useSettings';
import { diffJsonWithContext, DiffLine, detectChangeCategories, ChangeCategory } from '../utils/jsonDiff';
import { computeRootCauseLayout, LayoutResult } from '../utils/rootCauseLayout';

interface RootCauseViewProps {
  analysis: RootCauseAnalysisV2;
  initialResourceUID: string;
  initialTimestamp: Date;
  initialLookbackMs: number;
  onClose: () => void;
  onRefresh?: (uid: string, timestamp: Date, lookbackMs: number) => void;
  error?: string | null;
}

interface NodePosition {
  x: number;
  y: number;
  width: number;
  height: number;
  node: GraphNode;
  index: number;
  layer: number;
}

interface EdgeRoute {
  from: string;
  to: string;
  kind: 'spine' | 'attachment';
  relationshipType?: string;
  points: Array<{ x: number; y: number }>;
}

// Diff line component for displaying JSON diffs
const DiffLineView = ({ line }: { line: DiffLine }) => {
  const styleMap: Record<DiffLine['type'], string> = {
    add: 'text-emerald-400 bg-emerald-500/10',
    remove: 'text-red-400 bg-red-500/10',
    context: 'text-[var(--color-text-primary)]',
    gap: 'text-[var(--color-text-muted)] italic',
  };

  const prefixMap: Record<DiffLine['type'], string> = {
    add: '+',
    remove: '-',
    context: ' ',
    gap: '…',
  };

  return (
    <div className={`flex gap-2 px-2 rounded ${styleMap[line.type]}`}>
      <span className="select-none w-3 text-[var(--color-text-muted)]">{prefixMap[line.type]}</span>
      <span className="whitespace-pre-wrap break-all">{line.content}</span>
    </div>
  );
};

export const RootCauseView: React.FC<RootCauseViewProps> = ({
  analysis,
  initialResourceUID,
  initialTimestamp,
  initialLookbackMs,
  onClose,
  onRefresh,
  error
}) => {
  console.log('[RootCauseView] Component rendered with props:', {
    hasAnalysis: !!analysis,
    hasIncident: !!analysis?.incident,
    hasGraph: !!analysis?.incident?.graph,
    nodeCount: analysis?.incident?.graph?.nodes?.length || 0,
    error
  });

  const svgRef = useRef<SVGSVGElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const graphContainerRef = useRef<HTMLDivElement>(null);
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null);
  const [selectedEventIndex, setSelectedEventIndex] = useState<number>(0);
  const [containerSize, setContainerSize] = useState({ width: 0, height: 0 });
  const [bottomPanelHeight, setBottomPanelHeight] = useState<number>(208); // Reduced by 35% from 320px
  const [isCanvasReady, setIsCanvasReady] = useState<boolean>(false); // Hide canvas until fit is complete
  const [isResizing, setIsResizing] = useState<boolean>(false);
  const resizeStartY = useRef<number>(0);
  const resizeStartHeight = useRef<number>(0);
  const [showFullDiff, setShowFullDiff] = useState<boolean>(false);
  const [changeTypeFilter, setChangeTypeFilter] = useState<Set<ChangeCategory>>(new Set(['spec', 'status', 'metadata', 'other']));
  const { formatTime } = useSettings();

  // Pan/zoom state
  const [transform, setTransform] = useState<d3.ZoomTransform>(d3.zoomIdentity);
  const zoomRef = useRef<d3.ZoomBehavior<HTMLDivElement, unknown> | null>(null);

  // Get selected node from ID
  const selectedNode = useMemo(() => {
    if (!selectedNodeId) return null;
    return analysis.incident.graph.nodes.find(n => n.id === selectedNodeId) || null;
  }, [selectedNodeId, analysis]);

  // Editable parameters
  const [editableUID, setEditableUID] = useState(initialResourceUID);
  const [editableTimestamp, setEditableTimestamp] = useState(initialTimestamp);
  const [editableLookback, setEditableLookback] = useState(initialLookbackMs);
  const [isLoading, setIsLoading] = useState(false);


  // Helper function to decode base64 and parse JSON
  const parseEventData = (data: string | undefined): any => {
    if (!data) return null;

    try {
      // Check if data is base64 encoded (starts with eyJ which is base64 for '{"')
      if (data.startsWith('eyJ')) {
        // Decode base64
        const decoded = atob(data);
        return JSON.parse(decoded);
      } else {
        // Try parsing as plain JSON
        return JSON.parse(data);
      }
    } catch (e) {
      console.warn('Failed to parse event data:', e, 'Data preview:', data.substring(0, 50));
      return null;
    }
  };

  // Guard: Ensure we have valid analysis data
  if (!analysis || !analysis.incident || !analysis.incident.graph || analysis.incident.graph.nodes.length === 0) {
    console.error('[RootCauseView] Invalid analysis data - showing error screen:', {
      hasAnalysis: !!analysis,
      hasIncident: !!analysis?.incident,
      hasGraph: !!analysis?.incident?.graph,
      nodeCount: analysis?.incident?.graph?.nodes?.length || 0
    });
    return (
      <div className="fixed inset-0 z-[100] bg-[var(--color-app-bg)] flex items-center justify-center">
        <div className="text-center">
          <p className="text-red-400 mb-4">Invalid root cause analysis data</p>
          <button
            onClick={onClose}
            className="px-4 py-2 bg-blue-600 hover:bg-blue-700 rounded text-white"
          >
            Close
          </button>
        </div>
      </div>
    );
  }

  // Measure container
  useEffect(() => {
    if (!containerRef.current) return;

    const resizeObserver = new ResizeObserver((entries) => {
      for (const entry of entries) {
        const { width, height } = entry.contentRect;
        setContainerSize((prev) => {
          // Only update if size actually changed to prevent unnecessary re-renders
          if (prev.width === width && prev.height === height) {
            return prev;
          }
          return { width, height };
        });
      }
    });

    resizeObserver.observe(containerRef.current);
    return () => resizeObserver.disconnect();
  }, []);

  // Setup pan/zoom behavior
  useEffect(() => {
    if (!graphContainerRef.current) return;

    const zoomBehavior = d3.zoom<HTMLDivElement, unknown>()
      .scaleExtent([0.1, 3])
      .on('zoom', (event) => {
        setTransform(event.transform);
      });

    zoomRef.current = zoomBehavior;

    const selection = d3.select(graphContainerRef.current);
    selection.call(zoomBehavior);

    return () => {
      selection.on('.zoom', null);
    };
  }, []);

  // Auto-select first node (symptom) on load
  useEffect(() => {
    if (!selectedNodeId && analysis?.incident?.graph && analysis.incident.graph.nodes.length > 0) {
      // Select the symptom node (SPINE with step 1) by default
      const spineNodes = analysis.incident.graph.nodes
        .filter(n => n.nodeType === 'SPINE')
        .sort((a, b) => (a.stepNumber || 0) - (b.stepNumber || 0));

      if (spineNodes.length > 0) {
        setSelectedNodeId(spineNodes[0].id);
      }
    }
  }, [analysis, selectedNodeId]);

  // Clear loading state and reset canvas ready state when analysis data changes
  useEffect(() => {
    setIsLoading(false);
    setIsCanvasReady(false); // Hide canvas until fit completes
    hasInitialFit.current = false; // Reset initial fit flag for new data
  }, [analysis]);

  // Clear loading state when error occurs
  useEffect(() => {
    if (error) {
      setIsLoading(false);
    }
  }, [error]);

  // Combine change events and K8s events into a unified timeline with change detection
  const unifiedTimeline = useMemo(() => {
    if (!selectedNode) return [];

    console.log('[unifiedTimeline] Processing selectedNode:', {
      id: selectedNode.id,
      kind: selectedNode.resource.kind,
      hasAllEvents: !!selectedNode.allEvents,
      allEventsCount: selectedNode.allEvents?.length || 0,
      hasK8sEvents: !!selectedNode.k8sEvents,
      k8sEventsCount: selectedNode.k8sEvents?.length || 0
    });

    type TimelineEvent = {
      id: string;
      timestamp: Date;
      type: 'change' | 'k8s';
      changeEvent?: ChangeEventInfo;
      k8sEvent?: K8sEventInfo;
      changeCategories?: { spec: boolean; status: boolean; metadata: boolean; other: boolean };
    };

    const timeline: TimelineEvent[] = [];

    // Add change events with category detection
    if (selectedNode.allEvents) {
      selectedNode.allEvents.forEach((evt, idx) => {
        // Parse current and previous data to detect change categories
        const currentData = parseEventData(evt.data);
        let previousData = null;

        // Find previous event for comparison
        if (idx > 0) {
          previousData = parseEventData(selectedNode.allEvents![idx - 1].data);
        }

        let changeCategories = detectChangeCategories(previousData, currentData);

        // If this is the first event (no previous) or all categories are false, mark as 'other'
        if (!changeCategories.spec && !changeCategories.status && !changeCategories.metadata && !changeCategories.other) {
          changeCategories = { spec: false, status: false, metadata: false, other: true };
        }

        console.log('[unifiedTimeline] Event', idx, 'categories:', changeCategories);

        timeline.push({
          id: evt.eventId,
          timestamp: new Date(evt.timestamp),
          type: 'change',
          changeEvent: evt,
          changeCategories,
        });
      });
    }

    // Add K8s events (filter out empty/default events)
    if (selectedNode.k8sEvents) {
      selectedNode.k8sEvents.forEach(evt => {
        // Skip empty events (eventId empty or timestamp is epoch zero)
        if (!evt.eventId || evt.eventId === '' || new Date(evt.timestamp).getTime() <= 0) {
          return;
        }
        timeline.push({
          id: evt.eventId,
          timestamp: new Date(evt.timestamp),
          type: 'k8s',
          k8sEvent: evt,
        });
      });
    }

    // Sort by timestamp
    timeline.sort((a, b) => a.timestamp.getTime() - b.timestamp.getTime());

    console.log('[unifiedTimeline] Created timeline with', timeline.length, 'events');

    return timeline;
  }, [selectedNode]);

  // Filter timeline by change type
  const filteredTimeline = useMemo(() => {
    const filtered = unifiedTimeline.filter(event => {
      // Always show K8s events
      if (event.type === 'k8s') return true;

      // For change events, check if any of their categories match the filter
      if (event.changeCategories) {
        const hasMatchingCategory =
          (changeTypeFilter.has('spec') && event.changeCategories.spec) ||
          (changeTypeFilter.has('status') && event.changeCategories.status) ||
          (changeTypeFilter.has('metadata') && event.changeCategories.metadata) ||
          (changeTypeFilter.has('other') && event.changeCategories.other);

        return hasMatchingCategory;
      }

      // If no categories detected, show it
      return true;
    });

    console.log('[filteredTimeline] Filter result:', {
      total: unifiedTimeline.length,
      filtered: filtered.length,
      filters: Array.from(changeTypeFilter)
    });

    return filtered;
  }, [unifiedTimeline, changeTypeFilter]);

  // Keyboard navigation for events
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (!filteredTimeline || filteredTimeline.length === 0) return;

      if (e.key === 'ArrowUp') {
        e.preventDefault();
        setSelectedEventIndex((prev) => Math.max(0, prev - 1));
      } else if (e.key === 'ArrowDown') {
        e.preventDefault();
        setSelectedEventIndex((prev) => Math.min(filteredTimeline.length - 1, prev + 1));
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [filteredTimeline]);

  // Reset selected event index when node changes
  useEffect(() => {
    setSelectedEventIndex(0);
  }, [selectedNode]);

  // Scroll selected event into view
  useEffect(() => {
    if (selectedEventIndex >= 0 && filteredTimeline && filteredTimeline.length > 0) {
      // Find the event element by data attribute
      const eventElement = document.querySelector(`[data-event-index="${selectedEventIndex}"]`);
      if (eventElement) {
        eventElement.scrollIntoView({
          behavior: 'smooth',
          block: 'nearest',
          inline: 'nearest'
        });
      }
    }
  }, [selectedEventIndex, filteredTimeline]);

  // Handle resize of bottom panel
  useEffect(() => {
    const handleMouseMove = (e: MouseEvent) => {
      if (!containerRef.current) return;

      // Calculate delta from initial mouse position
      const deltaY = resizeStartY.current - e.clientY;
      const newHeight = resizeStartHeight.current + deltaY;

      // Constrain between min (200px) and max (80% of container)
      const containerRect = containerRef.current.getBoundingClientRect();
      const minHeight = 200;
      const maxHeight = containerRect.height * 0.8;
      const clampedHeight = Math.max(minHeight, Math.min(maxHeight, newHeight));

      setBottomPanelHeight(clampedHeight);
    };

    const handleMouseUp = () => {
      setIsResizing(false);
      document.body.style.userSelect = '';
      document.body.style.cursor = '';
    };

    if (isResizing) {
      document.addEventListener('mousemove', handleMouseMove);
      document.addEventListener('mouseup', handleMouseUp);
      // Prevent text selection while resizing
      document.body.style.userSelect = 'none';
      document.body.style.cursor = 'ns-resize';
    }

    return () => {
      document.removeEventListener('mousemove', handleMouseMove);
      document.removeEventListener('mouseup', handleMouseUp);
    };
  }, [isResizing]);

  // Calculate JSON diff for selected event
  const eventDiff = useMemo(() => {
    if (!filteredTimeline || filteredTimeline.length === 0) {
      return null;
    }

    const selectedItem = filteredTimeline[selectedEventIndex];

    // Only calculate diff for change events, not K8s events
    if (!selectedItem || selectedItem.type !== 'change' || !selectedItem.changeEvent?.data) {
      return null;
    }

    // Find previous change event (not K8s event) for diff
    let previousChangeEvent = null;
    for (let i = selectedEventIndex - 1; i >= 0; i--) {
      if (filteredTimeline[i].type === 'change' && filteredTimeline[i].changeEvent?.data) {
        previousChangeEvent = filteredTimeline[i].changeEvent;
        break;
      }
    }

    // Parse JSON data (handle base64 encoding)
    let currentData = null;
    let previousData = null;

    currentData = parseEventData(selectedItem.changeEvent.data);
    previousData = parseEventData(previousChangeEvent?.data);

    // Calculate diff
    if (!currentData) {
      return null;
    }

    // Show full diff (1000 lines context) or changed only (3 lines context)
    const contextLines = showFullDiff ? 1000 : 3;
    return diffJsonWithContext(previousData, currentData, contextLines);
  }, [filteredTimeline, selectedEventIndex, showFullDiff]);

  // Compute layout using custom layout engine
  const layoutResult: LayoutResult | null = useMemo(() => {
    console.log('[RootCauseView] layoutResult useMemo executing');
    try {
      if (!analysis?.incident?.graph || analysis.incident.graph.nodes.length === 0) {
        console.warn('[RootCauseView] No graph found in analysis');
        return null;
      }

      console.log('[RootCauseView] Computing layout for graph with', analysis.incident.graph.nodes.length, 'nodes');
      const result = computeRootCauseLayout(analysis);
      console.log('[RootCauseView] Layout result:', result ? `${result.nodes.length} nodes positioned, bounds:` : 'null', result?.bounds);
      return result;
    } catch (error) {
      console.error('[RootCauseView] Error computing layout:', error);
      return null;
    }
  }, [analysis]);

  // Convert layout result to nodePositions format for rendering
  const nodePositions: NodePosition[] = useMemo(() => {
    console.log('[RootCauseView] nodePositions useMemo executing, layoutResult:', !!layoutResult);
    if (!layoutResult) {
      console.log('[RootCauseView] No layoutResult, returning empty positions');
      return [];
    }

    const positions: NodePosition[] = [];

    layoutResult.nodes.forEach((layoutNode, idx) => {
      positions.push({
        x: layoutNode.x,
        y: layoutNode.y,
        width: layoutNode.width,
        height: layoutNode.height,
        node: layoutNode.nodeRef,
        index: idx,
        layer: layoutNode.nodeRef.stepNumber || 0,
      });
    });

    console.log('[RootCauseView] Created', positions.length, 'node positions');
    return positions;
  }, [layoutResult]);

  // Extract edge routes for rendering
  const edgeRoutes: EdgeRoute[] = useMemo(() => {
    if (!layoutResult) return [];
    return layoutResult.edges.map((edge) => ({
      from: edge.from,
      to: edge.to,
      kind: edge.kind,
      relationshipType: edge.relationshipType,
      points: edge.points,
    }));
  }, [layoutResult]);

  // Draw arrows with D3 using dagre routes
  useEffect(() => {
    if (!svgRef.current || edgeRoutes.length === 0 || nodePositions.length === 0) {
      return;
    }

    const svg = d3.select(svgRef.current);
    svg.selectAll('*').remove();

    // Define arrow markers
    const defs = svg.append('defs');

    // Normal arrow marker (gray for unselected main edges)
    defs.append('marker')
      .attr('id', 'arrowhead-normal')
      .attr('markerWidth', 4)
      .attr('markerHeight', 4)
      .attr('refX', 3.5)
      .attr('refY', 2)
      .attr('orient', 'auto')
      .append('path')
      .attr('d', 'M 0 0 L 4 2 L 0 4 z')
      .attr('fill', '#64748b');

    // Highlighted arrow marker (blue for selected main edges)
    defs.append('marker')
      .attr('id', 'arrowhead-highlight')
      .attr('markerWidth', 5)
      .attr('markerHeight', 5)
      .attr('refX', 4.5)
      .attr('refY', 2.5)
      .attr('orient', 'auto')
      .append('path')
      .attr('d', 'M 0 0 L 5 2.5 L 0 5 z')
      .attr('fill', '#60a5fa');

    // Related resource arrow marker (gray for unselected)
    defs.append('marker')
      .attr('id', 'arrowhead-related')
      .attr('markerWidth', 4)
      .attr('markerHeight', 4)
      .attr('refX', 3.5)
      .attr('refY', 2)
      .attr('orient', 'auto')
      .append('path')
      .attr('d', 'M 0 0 L 4 2 L 0 4 z')
      .attr('fill', '#9ca3af');

    // Related resource arrow marker (purple for selected)
    defs.append('marker')
      .attr('id', 'arrowhead-related-highlight')
      .attr('markerWidth', 5)
      .attr('markerHeight', 5)
      .attr('refX', 4.5)
      .attr('refY', 2.5)
      .attr('orient', 'auto')
      .append('path')
      .attr('d', 'M 0 0 L 5 2.5 L 0 5 z')
      .attr('fill', '#a78bfa');

    const arrowGroup = svg.append('g').attr('class', 'arrows');

    // Helper to find node by ID
    const findNodeById = (id: string): NodePosition | undefined => {
      return nodePositions.find(n => n.node.id === id);
    };

    // Draw edges using custom layout engine routes
    edgeRoutes.forEach((edge) => {
      const fromNode = findNodeById(edge.from);
      const toNode = findNodeById(edge.to);

      if (!fromNode || !toNode || edge.points.length < 2) {
        return;
      }

      // Determine if edge is connected to selected node
      const isConnected = selectedNodeId && (
        fromNode.node.id === selectedNodeId ||
        toNode.node.id === selectedNodeId
      );

      // Convert edge points to SVG path (offset by 10000 to match wrapper position)
      // The layout engine already provides points that connect to card boundaries
      const offset = 10000;
      let pathData = `M ${edge.points[0].x + offset} ${edge.points[0].y + offset}`;
      for (let i = 1; i < edge.points.length; i++) {
        pathData += ` L ${edge.points[i].x + offset} ${edge.points[i].y + offset}`;
      }

      // Determine styling based on edge kind
      const isAttachment = edge.kind === 'attachment';
      // Use distinct colors for attachment edges (purple/indigo for related resources)
      const strokeColor = isAttachment
        ? (isConnected ? '#a78bfa' : '#8b5cf6') // Purple for attachment edges
        : (isConnected ? '#60a5fa' : '#64748b'); // Blue for spine edges
      const strokeWidth = isConnected ? (isAttachment ? 2.5 : 2.5) : (isAttachment ? 2 : 1.5);
      const opacity = isAttachment ? (isConnected ? 1 : 0.6) : (isConnected ? 1 : 0.4);
      const markerId = isAttachment
        ? (isConnected ? 'arrowhead-related-highlight' : 'arrowhead-related')
        : (isConnected ? 'arrowhead-highlight' : 'arrowhead-normal');

      // Draw the path
      arrowGroup.append('path')
        .attr('d', pathData)
        .attr('stroke', strokeColor)
        .attr('stroke-width', strokeWidth)
        .attr('fill', 'none')
        .attr('opacity', opacity)
        .attr('stroke-dasharray', isAttachment ? '6,4' : 'none')
        .attr('marker-end', `url(#${markerId})`);

      // Draw connection point circles
      const startPoint = edge.points[0];
      const endPoint = edge.points[edge.points.length - 1];
      const connectorRadius = isConnected ? (isAttachment ? 4 : 5) : (isAttachment ? 3 : 4);
      const connectorStrokeWidth = isConnected ? 2 : 1.5;

      if (!isConnected) {
        arrowGroup.append('circle')
          .attr('cx', startPoint.x + offset)
          .attr('cy', startPoint.y + offset)
          .attr('r', connectorRadius)
          .attr('fill', 'var(--color-surface)')
          .attr('stroke', strokeColor)
          .attr('stroke-width', connectorStrokeWidth);

        arrowGroup.append('circle')
          .attr('cx', endPoint.x + offset)
          .attr('cy', endPoint.y + offset)
          .attr('r', connectorRadius)
          .attr('fill', 'var(--color-surface)')
          .attr('stroke', strokeColor)
          .attr('stroke-width', connectorStrokeWidth);
      } else {
        arrowGroup.append('circle')
          .attr('cx', startPoint.x + offset)
          .attr('cy', startPoint.y + offset)
          .attr('r', connectorRadius)
          .attr('fill', strokeColor)
          .attr('stroke', isAttachment ? '#8b5cf6' : '#3b82f6')
          .attr('stroke-width', connectorStrokeWidth);

        arrowGroup.append('circle')
          .attr('cx', endPoint.x + offset)
          .attr('cy', endPoint.y + offset)
          .attr('r', connectorRadius)
          .attr('fill', strokeColor)
          .attr('stroke', isAttachment ? '#8b5cf6' : '#3b82f6')
          .attr('stroke-width', connectorStrokeWidth);
      }

      // Relationship type label on edge (positioned at the middle of the edge)
      if (edge.relationshipType && edge.relationshipType !== 'SYMPTOM') {
        const midIdx = Math.floor(edge.points.length / 2);
        const midPoint = edge.points[midIdx];

        // For better label positioning on the middle of the edge
        // If we have more than 2 points, use the actual midpoint
        // Otherwise, calculate the midpoint between the two points
        let labelX = midPoint.x + offset;
        let labelY = midPoint.y + offset;

        if (edge.points.length === 2) {
          // For straight edges, use the exact midpoint
          labelX = (edge.points[0].x + edge.points[1].x) / 2 + offset;
          labelY = (edge.points[0].y + edge.points[1].y) / 2 + offset;
        }

        const labelText = edge.relationshipType.replace(/_/g, ' ');
        const estimatedWidth = labelText.length * 7 + 16;

        // Position label offset from the edge line
        const offsetY = edge.kind === 'attachment' ? -18 : -18; // Offset above the edge

        // Background
        arrowGroup.append('rect')
          .attr('x', labelX - estimatedWidth / 2)
          .attr('y', labelY + offsetY - 10)
          .attr('width', estimatedWidth)
          .attr('height', 18)
          .attr('fill', isConnected
            ? (edge.kind === 'attachment' ? 'rgba(139, 92, 246, 0.95)' : 'rgba(59, 130, 246, 0.95)')
            : 'var(--color-app-bg)')
          .attr('stroke', isConnected
            ? (edge.kind === 'attachment' ? '#a78bfa' : '#60a5fa')
            : '#64748b')
          .attr('stroke-width', 1)
          .attr('rx', 3);

        // Text
        arrowGroup.append('text')
          .attr('x', labelX)
          .attr('y', labelY + offsetY + 3)
          .attr('fill', isConnected ? '#ffffff' : '#94a3b8')
          .attr('font-size', '10px')
          .attr('font-weight', '700')
          .attr('text-anchor', 'middle')
          .attr('letter-spacing', '0.05em')
          .text(labelText);
      }
    });
  }, [edgeRoutes, nodePositions, selectedNodeId]);

  const getStatusColor = (index: number, total: number): string => {
    if (index === 0) return 'bg-blue-500';
    if (index === total - 1) return 'bg-red-500';
    return 'bg-amber-500';
  };

  // Get status color matching timeline view
  const getStatusColorFromStatus = (status?: string): string => {
    if (!status) return 'bg-gray-400 dark:bg-gray-600';

    switch (status) {
      case 'Ready':
        return 'bg-green-500';
      case 'Warning':
        return 'bg-yellow-500';
      case 'Error':
        return 'bg-red-500';
      case 'Terminating':
        return 'bg-orange-500';
      case 'Unknown':
      default:
        return 'bg-gray-400 dark:bg-gray-600';
    }
  };

  const getStatusTextColor = (status?: string): string => {
    if (!status) return 'text-gray-400 dark:text-gray-500';

    switch (status) {
      case 'Ready':
        return 'text-green-400';
      case 'Warning':
        return 'text-yellow-400';
      case 'Error':
        return 'text-red-400';
      case 'Terminating':
        return 'text-orange-400';
      case 'Unknown':
      default:
        return 'text-gray-400';
    }
  };

  const formatDuration = (ms: number): string => {
    if (ms < 60000) return `${ms / 1000}s`;
    if (ms < 3600000) return `${ms / 60000}m`;
    return `${ms / 3600000}h`;
  };

  const formatDateTimeLocal = (date: Date): string => {
    const year = date.getFullYear();
    const month = String(date.getMonth() + 1).padStart(2, '0');
    const day = String(date.getDate()).padStart(2, '0');
    const hours = String(date.getHours()).padStart(2, '0');
    const minutes = String(date.getMinutes()).padStart(2, '0');
    return `${year}-${month}-${day}T${hours}:${minutes}`;
  };

  // Track if we've done the initial fit
  const hasInitialFit = useRef(false);

  // Calculate transform to fit all nodes in view, centered on the symptom resource
  const fitToView = React.useCallback(() => {
    if (!layoutResult || !graphContainerRef.current || !containerRef.current) {
      console.log('[fitToView] Early return - missing layoutResult or container');
      return;
    }

    // Get the actual graph area dimensions (excluding bottom panel)
    const containerRect = containerRef.current.getBoundingClientRect();
    const viewportWidth = containerRect.width;
    const viewportHeight = containerRect.height; // This is already the graph area height (excludes bottom panel)

    if (viewportWidth <= 0 || viewportHeight <= 0) {
      console.log('[fitToView] Invalid viewport size:', { viewportWidth, viewportHeight });
      return;
    }

    // Calculate tight bounds from actual node positions (ignoring layout padding)
    // The layout bounds include 240px padding for edge routing, which is too much for fitting
    const nodes = layoutResult.nodes;
    let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
    for (const node of nodes) {
      minX = Math.min(minX, node.x);
      minY = Math.min(minY, node.y);
      maxX = Math.max(maxX, node.x + node.width);
      maxY = Math.max(maxY, node.y + node.height);
    }
    const tightBounds = {
      x: minX,
      y: minY,
      width: maxX - minX,
      height: maxY - minY,
    };
    const padding = 40; // Smaller padding for fitting

    console.log('[fitToView] Viewport (graph area):', { viewportWidth, viewportHeight });
    console.log('[fitToView] Tight bounds:', tightBounds);

    // Calculate scale to fit the graph in the viewport
    // Allow up to 2x scale to better utilize available space when graph is small
    const scaleX = (viewportWidth - padding * 2) / tightBounds.width;
    const scaleY = (viewportHeight - padding * 2) / tightBounds.height;
    const scale = Math.max(0.1, Math.min(scaleX, scaleY, 2.0));

    console.log('[fitToView] Scale:', { scaleX, scaleY, finalScale: scale });

    // Center on the tight bounds (all nodes visible), not just the symptom node
    // This ensures the entire graph is centered in the viewport
    const targetCenterX = tightBounds.x + tightBounds.width / 2;
    const targetCenterY = tightBounds.y + tightBounds.height / 2;
    console.log('[fitToView] Centering on tight bounds center');

    // After scaling, the target center will be at:
    const scaledTargetCenterX = targetCenterX * scale;
    const scaledTargetCenterY = targetCenterY * scale;

    // We want the target to be at the center of the viewport
    const viewportCenterX = viewportWidth / 2;
    const viewportCenterY = viewportHeight / 2;

    // So we need to translate by:
    const translateX = viewportCenterX - scaledTargetCenterX;
    const translateY = viewportCenterY - scaledTargetCenterY;

    console.log('[fitToView] TargetCenter:', { targetCenterX, targetCenterY });
    console.log('[fitToView] Transform:', { translateX, translateY, scale });

    const newTransform = d3.zoomIdentity.translate(translateX, translateY).scale(scale);

    if (zoomRef.current && graphContainerRef.current) {
      d3.select(graphContainerRef.current)
        .transition()
        .duration(300)
        .call(zoomRef.current.transform, newTransform)
        .on('end', () => {
          // Show canvas after fit animation completes
          setIsCanvasReady(true);
        });
    } else {
      // No animation needed, show immediately
      setIsCanvasReady(true);
    }
  }, [layoutResult]);

  // Auto-fit view once when layout is first computed
  useEffect(() => {
    if (layoutResult && !hasInitialFit.current) {
      // Wait for the container to be properly sized
      const timer = setTimeout(() => {
        hasInitialFit.current = true;
        fitToView();
      }, 200);
      return () => clearTimeout(timer);
    }
  }, [layoutResult, fitToView]);

  // Early return test - if this shows, component is rendering
  if (nodePositions.length === 0) {
    console.warn('[RootCauseView] No node positions calculated, showing fallback:', {
      hasLayoutResult: !!layoutResult,
      layoutNodes: layoutResult?.nodes?.length || 0,
      graphNodes: analysis?.incident?.graph?.nodes?.length || 0
    });
    return (
      <div className="fixed inset-0 z-[100] bg-[var(--color-app-bg)] flex items-center justify-center" data-testid="root-cause-view">
        <div className="text-center">
          <div className="bg-red-500 text-white p-4 mb-4 rounded">
            ROOT CAUSE VIEW IS RENDERING (but no positions)
          </div>
          <p className="text-[var(--color-text-muted)] mb-4">
            Graph nodes: {analysis.incident.graph?.nodes.length || 0}
          </p>
          <button
            onClick={onClose}
            className="px-4 py-2 bg-blue-600 hover:bg-blue-700 rounded text-white"
          >
            Close
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="fixed inset-0 z-[100] bg-[var(--color-app-bg)] overflow-hidden flex flex-col" data-testid="root-cause-view">
      {/* Header */}
      <div className="h-14 border-b border-[var(--color-border-soft)] px-6 flex items-center justify-between bg-[var(--color-surface)]">
        <div className="flex items-center gap-4">
          <h1 className="text-lg font-semibold text-[var(--color-text-primary)]">
            Root Cause Analysis
          </h1>
          <span className="text-sm text-[var(--color-text-muted)]">
            {analysis.incident.observedSymptom.resource.kind}/{analysis.incident.observedSymptom.resource.name}
          </span>
        </div>


        {/* Parameter Editor */}
        {onRefresh && (
          <div className="flex items-center gap-3 text-sm">
            <div className="flex items-center gap-2">
              <span className="text-[var(--color-text-muted)] text-xs">UID:</span>
              <input
                type="text"
                value={editableUID}
                onChange={(e) => {
                  const newUID = e.target.value;
                  setEditableUID(newUID);
                  if (newUID.length > 0) {
                    setIsLoading(true);
                    onRefresh(newUID, editableTimestamp, editableLookback);
                  }
                }}
                className="px-2 py-1 text-xs bg-[var(--color-surface-muted)] border border-[var(--color-border-soft)] rounded text-[var(--color-text-primary)] font-mono w-64"
                placeholder="Resource UID"
                disabled={isLoading}
              />
            </div>
            <input
              type="datetime-local"
              value={formatDateTimeLocal(editableTimestamp)}
              onChange={(e) => {
                const newDate = new Date(e.target.value);
                if (!isNaN(newDate.getTime())) {
                  setEditableTimestamp(newDate);
                  setIsLoading(true);
                  onRefresh(editableUID, newDate, editableLookback);
                }
              }}
              className="px-2 py-1 text-xs bg-[var(--color-surface-muted)] border border-[var(--color-border-soft)] rounded text-[var(--color-text-primary)]"
              disabled={isLoading}
            />
            <select
              value={editableLookback}
              onChange={(e) => {
                const newLookback = Number(e.target.value);
                setEditableLookback(newLookback);
                setIsLoading(true);
                onRefresh(editableUID, editableTimestamp, newLookback);
              }}
              className="px-2 py-1 text-xs bg-[var(--color-surface-muted)] border border-[var(--color-border-soft)] rounded text-[var(--color-text-primary)]"
              disabled={isLoading}
            >
              <option value={300000}>5 minutes</option>
              <option value={600000}>10 minutes</option>
              <option value={1800000}>30 minutes</option>
              <option value={3600000}>1 hour</option>
              <option value={21600000}>6 hours</option>
              <option value={86400000}>24 hours</option>
              <option value={172800000}>48 hours</option>
            </select>
          </div>
        )}

        <button
          onClick={onClose}
          className="px-4 py-2 text-sm bg-[var(--color-surface-muted)] hover:bg-[var(--color-border-soft)] rounded text-[var(--color-text-primary)] transition-colors"
        >
          Close
        </button>
      </div>

      {/* Main graph area */}
      <div className="flex-1 overflow-hidden bg-[#0a0e1a] relative" ref={containerRef}>
        {/* Reset Zoom button - positioned in canvas, only show when ready */}
        {layoutResult && isCanvasReady && (
          <button
            onClick={fitToView}
            className="absolute top-4 right-4 z-20 px-3 py-1.5 text-xs bg-[var(--color-surface)] hover:bg-[var(--color-surface-hover)] border border-[var(--color-border-soft)] rounded text-[var(--color-text-primary)] transition-colors shadow-lg flex items-center gap-1.5"
            title="Fit all nodes to view"
          >
            <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4" />
            </svg>
            Reset Zoom
          </button>
        )}

        {/* Loading indicator while canvas is positioning */}
        {layoutResult && !isCanvasReady && !isLoading && (
          <div className="absolute inset-0 flex items-center justify-center z-10">
            <div className="flex flex-col items-center gap-3">
              <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-500"></div>
              <span className="text-[var(--color-text-muted)] text-sm">Positioning graph...</span>
            </div>
          </div>
        )}

        {/* Loading overlay */}
        {isLoading && (
          <div className="absolute inset-0 bg-black/50 flex items-center justify-center z-[200]">
            <div className="flex flex-col items-center gap-3">
              <div className="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-500"></div>
              <span className="text-white text-sm font-medium">Loading analysis...</span>
            </div>
          </div>
        )}

        {/* Error toast */}
        {error && !isLoading && (
          <div className="absolute top-4 left-1/2 transform -translate-x-1/2 z-[200] animate-in slide-in-from-top duration-300">
            <div className="bg-red-500/90 backdrop-blur-sm text-white px-4 py-3 rounded-lg shadow-lg max-w-md flex items-start gap-3">
              <svg className="w-5 h-5 flex-shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
              <div className="flex-1">
                <p className="font-medium text-sm">{error}</p>
              </div>
            </div>
          </div>
        )}
        <div
          ref={graphContainerRef}
          className="relative w-full h-full transition-opacity duration-300"
          style={{
            minHeight: layoutResult ? Math.max(400, layoutResult.bounds.height + 100) : 400,
            minWidth: layoutResult ? Math.max(400, layoutResult.bounds.width + 100) : 400,
            padding: '40px',
            opacity: isCanvasReady ? 1 : 0,
          }}
        >
          {/* Transform wrapper for both SVG and cards */}
          {/* Position accounts for container padding so coordinates align */}
          <div
            style={{
              position: 'absolute',
              top: '-10000px',
              left: '-10000px',
              width: '20000px',
              height: '20000px',
              transform: `translate(${transform.x}px, ${transform.y}px) scale(${transform.k})`,
              transformOrigin: '10040px 10040px',
              backgroundImage: `
                linear-gradient(rgba(30, 41, 59, 0.5) 1px, transparent 1px),
                linear-gradient(90deg, rgba(30, 41, 59, 0.5) 1px, transparent 1px),
                radial-gradient(circle at 0px 0px, rgba(51, 65, 85, 0.6) 1px, transparent 1px)
              `,
              backgroundSize: '20px 20px, 20px 20px, 20px 20px',
              backgroundPosition: '0 0, 0 0, 0 0',
            }}
          >
            {/* SVG for arrows - on top of cards so they're always visible */}
            <svg
              ref={svgRef}
              className="absolute pointer-events-none"
              style={{
                top: 0,
                left: 0,
                zIndex: 15,
                width: '20000px',
                height: '20000px',
              }}
            />

            {/* Resource cards */}
            {nodePositions.length === 0 ? (
              <div className="absolute inset-0 flex items-center justify-center">
                <div className="text-center">
                  <p className="text-[var(--color-text-muted)] mb-4">No graph data available</p>
                  <p className="text-sm text-[var(--color-text-muted)]">Graph nodes: {analysis.incident.graph?.nodes.length || 0}</p>
                </div>
              </div>
            ) : (
              <div>
              {nodePositions.map((nodePos) => {
              const node = nodePos.node;
              const resource = node.resource;
              const events = node.allEvents;
              const isSpine = node.nodeType === 'SPINE';

              return (
            <div
              key={node.id}
              className="absolute cursor-pointer transition-all duration-200"
              style={{
                left: nodePos.x + 10000,
                top: nodePos.y + 10000,
                width: 280,
                height: nodePos.height,
                zIndex: 10,
              }}
              onClick={() => setSelectedNodeId(node.id)}
            >
              <div
                className={`
                  bg-white dark:bg-[#1e293b] border-2 rounded-lg overflow-hidden
                  ${!isSpine ? 'border-dashed' : ''}
                  ${selectedNodeId === node.id
                    ? 'border-blue-500 shadow-[0_0_12px_rgba(59,130,246,0.3)]'
                    : 'border-gray-200 dark:border-slate-600 hover:border-blue-300 shadow-[0_2px_8px_rgba(0,0,0,0.2)]'
                  }
                `}
                style={{ height: '100%', display: 'flex', flexDirection: 'column' }}
              >
                {/* Card Header */}
                <div className="p-3 border-b border-gray-100 dark:border-slate-700 bg-white dark:bg-slate-800/50">
                  <div className="flex items-start justify-between mb-1.5">
                    <div className="flex items-center gap-2">
                      <div className={`w-2.5 h-2.5 rounded-full ${
                        !isSpine
                          ? (events && events.length > 0 ? 'bg-orange-500' : 'bg-gray-400')
                          : getStatusColor(nodePos.index, nodePositions.filter(n => n.node.nodeType === 'SPINE').length)
                      }`} />
                      <span className="text-[10px] text-gray-500 dark:text-[var(--color-text-muted)] uppercase font-bold tracking-wider">
                        {resource.kind}
                      </span>
                    </div>
                    {node.changeEvent && (
                      <span className="text-[10px] text-gray-400 dark:text-[var(--color-text-muted)]">
                        {new Date(node.changeEvent.timestamp).toLocaleTimeString()}
                      </span>
                    )}
                  </div>
                  <div className="text-sm font-bold text-gray-900 dark:text-[var(--color-text-primary)] truncate" title={resource.name}>
                    {resource.name}
                  </div>
                </div>

                {/* Event counts */}
                {events && events.length > 0 && (() => {
                  const counts = {
                    CREATE: 0,
                    DELETE: 0,
                    UPDATE: {
                      total: 0,
                      spec: 0,
                      status: 0,
                      metadata: 0,
                    },
                  };

                  events.forEach((evt) => {
                    if (evt.eventType === 'CREATE') {
                      counts.CREATE++;
                    } else if (evt.eventType === 'DELETE') {
                      counts.DELETE++;
                    } else if (evt.eventType === 'UPDATE') {
                      counts.UPDATE.total++;
                      if (evt.data) {
                        try {
                          const prevData = JSON.parse(evt.data);
                          const categories = detectChangeCategories(prevData, prevData);
                          if (categories.spec) counts.UPDATE.spec++;
                          if (categories.status) counts.UPDATE.status++;
                          if (categories.metadata) counts.UPDATE.metadata++;
                        } catch (e) {}
                      }
                    }
                  });

                  return (
                    <div className="px-3 py-2 bg-gray-50 dark:bg-slate-900/30">
                      <div className="flex flex-wrap gap-1.5">
                        {counts.CREATE > 0 && (
                          <div className="flex items-center gap-1 text-[10px] font-semibold px-2 py-1 rounded bg-green-100 dark:bg-green-900/30 text-green-700 dark:text-green-400">
                            <span>CREATE</span>
                            <span className="opacity-70">×{counts.CREATE}</span>
                          </div>
                        )}
                        {counts.UPDATE.total > 0 && (
                          <div className="flex items-center gap-1 text-[10px] font-semibold px-2 py-1 rounded bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-400">
                            <span>UPDATE</span>
                            <span className="opacity-70">×{counts.UPDATE.total}</span>
                          </div>
                        )}
                        {counts.DELETE > 0 && (
                          <div className="flex items-center gap-1 text-[10px] font-semibold px-2 py-1 rounded bg-red-100 dark:bg-red-900/30 text-red-700 dark:text-red-400">
                            <span>DELETE</span>
                            <span className="opacity-70">×{counts.DELETE}</span>
                          </div>
                        )}
                      </div>

                      {/* UPDATE subcategories */}
                      {counts.UPDATE.total > 0 && (counts.UPDATE.spec > 0 || counts.UPDATE.status > 0 || counts.UPDATE.metadata > 0) && (
                        <div className="flex flex-wrap gap-1.5 mt-1.5 pl-2 border-l-2 border-blue-300 dark:border-blue-700">
                          {counts.UPDATE.spec > 0 && (
                            <div className="text-[9px] font-semibold px-1.5 py-0.5 rounded bg-blue-50 dark:bg-blue-950/50 text-blue-600 dark:text-blue-300">
                              .spec ×{counts.UPDATE.spec}
                            </div>
                          )}
                          {counts.UPDATE.status > 0 && (
                            <div className="text-[9px] font-semibold px-1.5 py-0.5 rounded bg-amber-50 dark:bg-amber-950/50 text-amber-600 dark:text-amber-300">
                              .status ×{counts.UPDATE.status}
                            </div>
                          )}
                          {counts.UPDATE.metadata > 0 && (
                            <div className="text-[9px] font-semibold px-1.5 py-0.5 rounded bg-purple-50 dark:bg-purple-950/50 text-purple-600 dark:text-purple-300">
                              .metadata ×{counts.UPDATE.metadata}
                            </div>
                          )}
                        </div>
                      )}
                    </div>
                  );
                })()}
              </div>
            </div>
              );
              })}
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Full-width bottom panel - split into diff (left) and event timeline (right) */}
      <div
        className="border-t border-[var(--color-border-soft)] bg-[var(--color-surface)] flex flex-col relative"
        style={{ height: `${bottomPanelHeight}px` }}
      >
        {/* Resize handle - increased hit area */}
        <div
          className="absolute top-0 left-0 right-0 h-2 -mt-1 cursor-ns-resize hover:bg-blue-500/30 transition-colors z-10 group"
          onMouseDown={(e) => {
            e.preventDefault();
            resizeStartY.current = e.clientY;
            resizeStartHeight.current = bottomPanelHeight;
            setIsResizing(true);
          }}
          title="Drag to resize"
        >
          {/* Visual indicator */}
          <div className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 w-12 h-1 bg-[var(--color-border-soft)] group-hover:bg-blue-500/70 rounded-full transition-colors" />
        </div>
        <div className="flex items-center justify-between px-4 py-2 border-b border-[var(--color-border-soft)]">
          <h3 className="text-sm font-semibold text-[var(--color-text-primary)] uppercase tracking-wider">
            Event Timeline & Manifest Comparison
          </h3>
          {selectedNode && (
            <div className="text-xs text-[var(--color-text-muted)]">
              <span className="font-mono text-[var(--color-text-primary)]">{selectedNode.resource.kind}</span>
              <span className="mx-2">/</span>
              <span className="font-mono text-[var(--color-text-primary)]">{selectedNode.resource.name}</span>
            </div>
          )}
          {/* Change type filters */}
          <div className="flex gap-2 text-[10px]">
            {(['spec', 'status', 'metadata'] as ChangeCategory[]).map(category => {
              const isActive = changeTypeFilter.has(category);
              const colors = {
                spec: { bg: 'bg-blue-500/20', text: 'text-blue-400', activeBg: 'bg-blue-500', activeText: 'text-white' },
                status: { bg: 'bg-amber-500/20', text: 'text-amber-400', activeBg: 'bg-amber-500', activeText: 'text-white' },
                metadata: { bg: 'bg-purple-500/20', text: 'text-purple-400', activeBg: 'bg-purple-500', activeText: 'text-white' },
              }[category];

              return (
                <button
                  key={category}
                  onClick={() => {
                    const newFilter = new Set(changeTypeFilter);
                    if (isActive) {
                      newFilter.delete(category);
                    } else {
                      newFilter.add(category);
                    }
                    setChangeTypeFilter(newFilter);
                    setSelectedEventIndex(0); // Reset selection when filter changes
                  }}
                  className={`px-2 py-1 rounded cursor-pointer transition-all ${
                    isActive
                      ? `${colors.activeBg} ${colors.activeText} font-semibold`
                      : `${colors.bg} ${colors.text} hover:opacity-80`
                  }`}
                  title={`${isActive ? 'Hide' : 'Show'} ${category} changes`}
                >
                  .{category.toUpperCase()}
                </button>
              );
            })}
          </div>
        </div>

        {/* Split panel: diff (left 70%) and event list (right 30%) */}
        <div className="flex-1 flex overflow-hidden">
          {/* Left: Diff view */}
          <div className="flex-[7] border-r border-[var(--color-border-soft)] flex flex-col">
            {/* Toolbar */}
            <div className="px-4 py-2 border-b border-[var(--color-border-soft)] flex items-center justify-between">
              <div className="text-[10px] font-semibold text-[var(--color-text-muted)] uppercase">
                Manifest Diff
              </div>
              <button
                onClick={() => setShowFullDiff(!showFullDiff)}
                className="text-[10px] px-2 py-1 rounded bg-[var(--color-surface-active)] hover:bg-[var(--color-surface-hover)] text-[var(--color-text-primary)] transition-colors"
              >
                {showFullDiff ? 'Show Changes Only' : 'Show Full Diff'}
              </button>
            </div>

            {/* Diff content */}
            <div className="flex-1 overflow-y-auto overflow-x-auto p-4">
              {filteredTimeline && filteredTimeline[selectedEventIndex] ? (
                <>
                  {/* Show diff for change events */}
                  {filteredTimeline[selectedEventIndex].type === 'change' && eventDiff && eventDiff.length > 0 ? (
                    <div className="font-mono text-xs space-y-0">
                      {eventDiff.map((line, i) => (
                        <DiffLineView key={i} line={line} />
                      ))}
                    </div>
                  ) : filteredTimeline[selectedEventIndex].type === 'change' ? (
                    <div className="flex items-center justify-center h-full text-[var(--color-text-muted)]">
                      {selectedEventIndex === 0 ? 'First event - no previous state to compare' : 'No data available for diff'}
                    </div>
                  ) : null}

                  {/* Show K8s event details */}
                  {filteredTimeline[selectedEventIndex].type === 'k8s' && filteredTimeline[selectedEventIndex].k8sEvent && (
                    <div className="space-y-4">
                      <div className="border border-[var(--color-border-soft)] rounded-lg p-4 bg-[var(--color-surface-muted)]">
                        <div className="flex items-start gap-3 mb-3">
                          <div className={`px-2 py-1 rounded text-xs font-semibold ${
                            filteredTimeline[selectedEventIndex].k8sEvent!.type === 'Warning' ? 'bg-amber-500/20 text-amber-400' :
                            filteredTimeline[selectedEventIndex].k8sEvent!.type === 'Error' ? 'bg-red-500/20 text-red-400' :
                            'bg-blue-500/20 text-blue-400'
                          }`}>
                            {filteredTimeline[selectedEventIndex].k8sEvent!.type}
                          </div>
                          <div className="flex-1">
                            <div className="text-sm font-semibold text-[var(--color-text-primary)] mb-1">
                              {filteredTimeline[selectedEventIndex].k8sEvent!.reason}
                            </div>
                            <div className="text-xs text-[var(--color-text-muted)]">
                              Count: {filteredTimeline[selectedEventIndex].k8sEvent!.count} •{' '}
                              Source: {filteredTimeline[selectedEventIndex].k8sEvent!.source || 'Unknown'}
                            </div>
                          </div>
                        </div>
                        <div className="text-sm text-[var(--color-text-primary)] whitespace-pre-wrap">
                          {filteredTimeline[selectedEventIndex].k8sEvent!.message}
                        </div>
                      </div>
                    </div>
                  )}
                </>
              ) : (
                <div className="flex items-center justify-center h-full text-[var(--color-text-muted)]">
                  Click a resource card to view events
                </div>
              )}
            </div>
          </div>

          {/* Right: Event timeline list */}
          <div className="flex-[3] flex flex-col">
            <div className="px-3 py-2 border-b border-[var(--color-border-soft)] bg-[var(--color-surface-muted)]">
              <div className="text-[10px] font-semibold text-[var(--color-text-muted)] uppercase tracking-wider">
                Event Timeline
              </div>
              {filteredTimeline && filteredTimeline.length > 0 && (
                <div className="text-[9px] text-[var(--color-text-muted)] mt-0.5">
                  {filteredTimeline.length} event{filteredTimeline.length !== 1 ? 's' : ''} • Use ↑↓ to navigate
                </div>
              )}
            </div>
            <div className="flex-1 overflow-y-auto">
              {filteredTimeline && filteredTimeline.length > 0 ? (
                <div className="divide-y divide-[var(--color-border-soft)]">
                  {filteredTimeline.map((item, index) => {
                    // Determine border color based on status (use ! for important to override divide-y)
                    let borderColor = '!border-l-gray-500'; // default
                    if (item.type === 'change' && item.changeEvent?.status) {
                      switch (item.changeEvent.status) {
                        case 'Ready':
                          borderColor = '!border-l-green-500';
                          break;
                        case 'Warning':
                          borderColor = '!border-l-yellow-500';
                          break;
                        case 'Error':
                          borderColor = '!border-l-red-500';
                          break;
                        case 'Terminating':
                          borderColor = '!border-l-orange-500';
                          break;
                        default:
                          borderColor = '!border-l-gray-500';
                      }
                    } else if (item.type === 'k8s') {
                      // K8s events get a different color scheme
                      borderColor = '!border-l-blue-400';
                    }

                    return (
                      <div
                        key={item.id}
                        data-event-index={index}
                        onClick={() => setSelectedEventIndex(index)}
                        className={`px-3 py-2 cursor-pointer transition-colors border-l-4 ${borderColor} ${
                          index === selectedEventIndex
                            ? 'bg-blue-500/20'
                            : 'hover:bg-[var(--color-surface-active)]'
                        }`}
                      >
                      {item.type === 'change' && item.changeEvent ? (
                        <>
                          <div className="flex items-center justify-between mb-1">
                            <div className={`text-[10px] font-semibold ${
                              item.changeEvent.eventType === 'CREATE' ? 'text-emerald-400' :
                              item.changeEvent.eventType === 'DELETE' ? 'text-red-400' :
                              'text-blue-400'
                            }`}>
                              {item.changeEvent.eventType}
                            </div>
                            <div className="text-[9px] text-[var(--color-text-muted)]">
                              {formatTime(item.timestamp)}
                            </div>
                          </div>
                          <div className="flex gap-1.5 flex-wrap">
                            {item.changeCategories?.spec && (
                              <span className="text-[8px] px-1.5 py-0.5 bg-blue-500/20 text-blue-400 rounded">
                                .spec
                              </span>
                            )}
                            {item.changeCategories?.status && (
                              <span className="text-[8px] px-1.5 py-0.5 bg-amber-500/20 text-amber-400 rounded">
                                .status
                              </span>
                            )}
                            {item.changeCategories?.metadata && (
                              <span className="text-[8px] px-1.5 py-0.5 bg-purple-500/20 text-purple-400 rounded">
                                .metadata
                              </span>
                            )}
                          </div>
                          {item.changeEvent.description && (
                            <div className="text-[9px] text-[var(--color-text-muted)] mt-1 truncate">
                              {item.changeEvent.description}
                            </div>
                          )}
                        </>
                      ) : item.type === 'k8s' && item.k8sEvent ? (
                        <>
                          <div className="flex items-center justify-between mb-1">
                            <div className="flex items-center gap-2">
                              <div className="text-[10px] font-semibold text-[var(--color-text-primary)]">
                                K8s Event
                              </div>
                              <div className={`text-[8px] px-1.5 py-0.5 rounded ${
                                item.k8sEvent.type === 'Warning' ? 'bg-amber-500/20 text-amber-400' :
                                item.k8sEvent.type === 'Error' ? 'bg-red-500/20 text-red-400' :
                                'bg-blue-500/20 text-blue-400'
                              }`}>
                                {item.k8sEvent.type}
                              </div>
                            </div>
                            <div className="text-[9px] text-[var(--color-text-muted)]">
                              {formatTime(item.timestamp)}
                            </div>
                          </div>
                          <div className="text-[9px] font-semibold text-[var(--color-text-primary)] mb-0.5">
                            {item.k8sEvent.reason}
                          </div>
                          <div className="text-[9px] text-[var(--color-text-muted)] truncate">
                            {item.k8sEvent.message}
                          </div>
                        </>
                      ) : null}
                    </div>
                  );
                  })}
                </div>
              ) : (
                <div className="flex items-center justify-center h-full px-3 text-center text-[10px] text-[var(--color-text-muted)]">
                  {selectedNode ? 'No events in time window' : 'Select a resource to view events'}
                </div>
              )}
            </div>
          </div>
        </div>
      </div>

      {/* Legend */}
      <div className="h-6 border-t border-[var(--color-border-soft)] bg-[var(--color-app-bg)] px-6 flex items-center gap-4 text-[10px] text-[var(--color-text-muted)]">
        <div className="flex items-center gap-1.5">
          <div className="w-1.5 h-1.5 rounded-full bg-green-500" />
          <span>READY</span>
        </div>
        <div className="flex items-center gap-1.5">
          <div className="w-1.5 h-1.5 rounded-full bg-yellow-500" />
          <span>WARNING</span>
        </div>
        <div className="flex items-center gap-1.5">
          <div className="w-1.5 h-1.5 rounded-full bg-red-500" />
          <span>ERROR</span>
        </div>
        <div className="flex items-center gap-1.5">
          <div className="w-1.5 h-1.5 rounded-full bg-orange-500" />
          <span>TERMINATING</span>
        </div>
        <div className="border-l border-[var(--color-border-soft)] h-4 mx-2" />
        <div className="flex items-center gap-1.5">
          <span className="text-emerald-400">+</span>
          <span>ADDITIONS</span>
        </div>
        <div className="flex items-center gap-1.5">
          <span className="text-red-400">-</span>
          <span>DELETIONS</span>
        </div>
        <div className="border-l border-[var(--color-border-soft)] h-4 mx-2" />
        <div className="flex items-center gap-1.5">
          <div className="w-6 h-0.5 bg-blue-400" />
          <span>CAUSAL CHAIN</span>
        </div>
        <div className="flex items-center gap-1.5">
          <div className="w-6 h-0.5 border-t-2 border-dashed border-purple-400" />
          <span>RELATED RESOURCES</span>
        </div>
      </div>
    </div>
  );
};
