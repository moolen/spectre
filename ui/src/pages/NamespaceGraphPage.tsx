import React, { useState, useMemo, useEffect, useCallback, useRef } from 'react';
import { useSearchParams } from 'react-router-dom';
import { useMetadata } from '../hooks/useMetadata';
import { useNamespaceGraph } from '../hooks/useNamespaceGraph';
import { usePersistedFilters } from '../hooks/usePersistedFilters';
import { usePersistedGraphNamespace } from '../hooks/usePersistedGraphNamespace';
import { usePersistedGraphTimestamp } from '../hooks/usePersistedGraphTimestamp';
import { usePersistedGraphLookback } from '../hooks/usePersistedGraphLookback';
import { useSettings } from '../hooks/useSettings';
import { parseTimeExpression, formatDateTimeForDisplay } from '../utils/timeParsing';
import { sortNamespaceFilterOptions, toNamespaceFilterValue } from '../utils/namespaceFilters';
import { D3GraphNode } from '../types/namespaceGraph';
import {
  NamespaceGraph,
  NamespaceGraphHandle,
  NamespaceGraphControls,
  NodeDetailPanel,
  ZoomControls,
  CausalPathsSidebar,
} from '../components/NamespaceGraph';

/**
 * Loading spinner component
 */
const LoadingSpinner: React.FC<{ message?: string }> = ({ message = 'Loading...' }) => (
  <div className="flex flex-col items-center justify-center h-full gap-3">
    <div className="w-6 h-6 rounded-full border-2 border-[var(--color-border-strong)] border-t-[var(--color-accent)] animate-spin" />
    <span className="text-[12.5px] text-[var(--color-text-faint)]">{message}</span>
  </div>
);

/**
 * Error message component
 */
const ErrorMessage: React.FC<{ error: Error; onRetry?: () => void }> = ({ error, onRetry }) => (
  <div className="flex flex-col items-center justify-center h-full p-8 gap-3 text-center">
    <div className="w-10 h-10 rounded-full grid place-items-center bg-[var(--color-status-err-soft)] text-[var(--color-status-err)]">
      <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.75} strokeLinecap="round">
        <circle cx="12" cy="12" r="9" /><path d="M12 8v4M12 16h.01" />
      </svg>
    </div>
    <h3 className="text-[13.5px] font-medium text-[var(--color-text-primary)]">Failed to load graph</h3>
    <p className="text-xs text-[var(--color-text-faint)] max-w-md">
      {error.message}
    </p>
    {onRetry && (
      <button onClick={onRetry} className="ui-control mt-1">
        Try Again
      </button>
    )}
  </div>
);

/**
 * Empty state component when no namespaces are available
 */
const EmptyState: React.FC = () => (
  <div className="flex flex-col items-center justify-center h-full p-8 gap-3 text-center">
    <div className="w-10 h-10 rounded-full grid place-items-center bg-[var(--color-surface-muted)] text-[var(--color-text-faint)]">
      <svg className="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.75} strokeLinecap="round" strokeLinejoin="round">
        <path d="M22 12h-6l-2 3h-4l-2-3H2" />
        <path d="M5.45 5.11 2 12v6a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2v-6l-3.45-6.89A2 2 0 0 0 16.76 4H7.24a2 2 0 0 0-1.79 1.11z" />
      </svg>
    </div>
    <h3 className="text-[13.5px] font-medium text-[var(--color-text-primary)]">No namespaces found</h3>
    <p className="text-xs text-[var(--color-text-faint)] max-w-md">
      No namespaces are available in the cluster. Make sure resources are being monitored.
    </p>
  </div>
);

const LEGEND_ITEMS = [
  { label: 'Healthy', color: 'var(--color-status-ok)' },
  { label: 'Degraded', color: 'var(--color-status-warn)' },
  { label: 'Failing', color: 'var(--color-status-err)' },
  { label: 'Causal path', color: 'var(--color-accent)' },
];

/**
 * Compact legend shown next to the zoom controls
 */
const GraphLegend: React.FC = () => (
  <div className="absolute bottom-4 left-[60px] h-[30px] px-3 flex items-center gap-3.5 rounded-lg border border-[var(--color-border-soft)] bg-[var(--color-surface-elevated)]/90 text-[11.5px] text-[var(--color-text-faint)] z-10">
    {LEGEND_ITEMS.map(item => (
      <span key={item.label} className="inline-flex items-center gap-1.5">
        <span className="w-2 h-2 rounded-sm" style={{ background: item.color }} />
        {item.label}
      </span>
    ))}
  </div>
);

/**
 * Main Namespace Graph Page
 * 
 * Auto-selects the first namespace (alphabetically) and displays the resource graph.
 * Uses a simplified control bar with just a namespace dropdown.
 * 
 * URL Parameters:
 * - namespace: Selected namespace name
 */
export default function NamespaceGraphPage() {
  const [searchParams, setSearchParams] = useSearchParams();
  
  // Ref to graph component for zoom controls
  const graphRef = useRef<NamespaceGraphHandle>(null);
  
  // Currently selected node for detail panel
  const [selectedNode, setSelectedNode] = useState<D3GraphNode | null>(null);
  
  // Currently selected causal path for highlighting
  const [selectedCausalPathId, setSelectedCausalPathId] = useState<string | null>(null);

  // Time range for fetching available namespaces (last 24 hours)
  const metadataTimeRange = useMemo(() => ({
    start: new Date(Date.now() - 24 * 60 * 60 * 1000),
    end: new Date(),
  }), []);

  // Fetch available namespaces and kinds
  const { namespaces, kinds: availableKinds, loading: namespacesLoading, error: namespacesError } = useMetadata(metadataTimeRange);

  // Sort namespaces alphabetically and keep cluster-scoped resources explicit
  const sortedNamespaces = useMemo(() => sortNamespaceFilterOptions(namespaces), [namespaces]);

  // Persisted namespace from localStorage
  const { namespace: persistedNamespace, setNamespace: setPersistedNamespace } = usePersistedGraphNamespace(sortedNamespaces);

  // Parse URL parameters - namespace from URL takes priority
  const rawUrlNamespace = searchParams.get('namespace');
  const urlNamespace = rawUrlNamespace === null ? null : toNamespaceFilterValue(rawUrlNamespace);
  
  // Effective namespace: URL param > persisted > null
  const selectedNamespace = urlNamespace ?? persistedNamespace;

  // Update URL and persist when namespace changes
  const setSelectedNamespace = useCallback((ns: string | null) => {
    // Update URL
    if (ns !== null) {
      setSearchParams({ namespace: ns }, { replace: true });
    } else {
      setSearchParams({}, { replace: true });
    }
    // Persist to localStorage
    setPersistedNamespace(ns);
    // Clear selection state
    setSelectedNode(null);
    setSelectedCausalPathId(null);
  }, [setSearchParams, setPersistedNamespace]);

  // Sync URL with persisted namespace on initial load (if no URL param but has persisted)
  useEffect(() => {
    if (urlNamespace === null && persistedNamespace && sortedNamespaces.includes(persistedNamespace)) {
      setSearchParams({ namespace: persistedNamespace }, { replace: true });
    }
  }, [urlNamespace, persistedNamespace, sortedNamespaces, setSearchParams]);

  // Get settings
  const { defaultKinds, hideInactiveReplicaSets } = useSettings();

  // Use persisted filters for kind selection
  const { kinds: selectedKinds, setKinds } = usePersistedFilters(availableKinds, [], defaultKinds);

  // Use persisted timestamp expression
  const { expression: timestampExpression, setExpression: setTimestampExpression } = usePersistedGraphTimestamp();

  // Use persisted lookback period for spec changes
  const { lookback, setLookback } = usePersistedGraphLookback();

  // Parse timestamp expression to Date, re-evaluate when expression or namespace changes
  const graphTimestamp = useMemo(() => {
    const parsed = parseTimeExpression(timestampExpression);
    if (parsed) {
      return parsed;
    }
    // Fallback to "now" if expression is invalid
    return new Date();
  }, [timestampExpression]);

  // Determine if we're in live mode (viewing "now")
  const isLiveMode = timestampExpression.toLowerCase().trim() === 'now';

  // Auto-select first namespace when namespaces load and none is selected
  useEffect(() => {
    if (!selectedNamespace && !namespacesLoading && sortedNamespaces.length > 0) {
      setSelectedNamespace(sortedNamespaces[0]);
    }
  }, [selectedNamespace, namespacesLoading, sortedNamespaces, setSelectedNamespace]);

  // Fetch graph data - always use current time and include anomalies and causal paths
  // Uses pagination with auto-loading for better performance
  const { data, isLoading, isLoadingMore, hasMore, pagesLoaded, error, refetch } = useNamespaceGraph({
    namespace: selectedNamespace,
    timestamp: graphTimestamp,
    includeAnomalies: true,
    includeCausalPaths: true,
    lookback,
    enabled: !!selectedNamespace,
    pageSize: 50,
    autoLoad: true,
  });

  // Filter graph data by selected kinds and settings (client-side)
  const filteredData = useMemo(() => {
    if (!data) return null;

    // Get UIDs of nodes in the selected causal path (if any)
    // When a causal path is selected, we want to show all nodes in the path
    // regardless of the kind filter
    let causalPathNodeUids: Set<string> | null = null;
    if (selectedCausalPathId && data.causalPaths) {
      const path = data.causalPaths.find(p => p.id === selectedCausalPathId);
      if (path) {
        causalPathNodeUids = new Set(path.steps.map(step => step.node.resource.uid));
      }
    }

    // Filter nodes by selected kinds, but always include nodes in the selected causal path
    let filteredNodes = selectedKinds.length > 0
      ? data.graph.nodes.filter(node =>
          selectedKinds.includes(node.kind) || causalPathNodeUids?.has(node.uid)
        )
      : [];

    // If no kinds are selected OR selected kinds don't match any graph nodes,
    // show ALL nodes from the graph data (graceful fallback instead of empty state)
    let nodesToShow = filteredNodes.length > 0 ? filteredNodes : data.graph.nodes;

    // Filter out inactive ReplicaSets (spec.replicas = 0) if setting is enabled
    // Always include ReplicaSets that are part of the selected causal path
    if (hideInactiveReplicaSets) {
      nodesToShow = nodesToShow.filter(node => {
        // Always show nodes in causal path
        if (causalPathNodeUids?.has(node.uid)) return true;
        // For ReplicaSets, check if spec.replicas > 0
        if (node.kind === 'ReplicaSet') {
          const replicas = node.latestEvent?.specReplicas;
          // If specReplicas is undefined, we don't know the replica count - show the node
          // If specReplicas is 0, hide the node
          // If specReplicas > 0, show the node
          return replicas === undefined || replicas > 0;
        }
        // Non-ReplicaSet nodes are always shown
        return true;
      });
    }

    const nodeUids = new Set(nodesToShow.map(n => n.uid));

    // Filter edges to only include those between visible nodes
    const filteredEdges = data.graph.edges.filter(edge =>
      nodeUids.has(edge.source) && nodeUids.has(edge.target)
    );

    return {
      ...data,
      graph: { nodes: nodesToShow, edges: filteredEdges },
      metadata: { ...data.metadata, nodeCount: nodesToShow.length, edgeCount: filteredEdges.length }
    };
  }, [data, selectedKinds, selectedCausalPathId, hideInactiveReplicaSets]);

  // Get anomalies for the selected node
  const selectedNodeAnomalies = useMemo(() => {
    if (!selectedNode || !data?.anomalies) return [];
    return data.anomalies.filter(a => a.node.uid === selectedNode.uid);
  }, [selectedNode, data?.anomalies]);

  // Compute highlighted node UIDs from selected causal path
  const highlightedNodeUids = useMemo(() => {
    if (!selectedCausalPathId || !data?.causalPaths) return null;
    const path = data.causalPaths.find(p => p.id === selectedCausalPathId);
    if (!path) return null;
    return new Set(path.steps.map(step => step.node.resource.uid));
  }, [selectedCausalPathId, data?.causalPaths]);

  // Focus on causal path nodes when a path is selected
  useEffect(() => {
    if (highlightedNodeUids && highlightedNodeUids.size > 0) {
      // Small delay to allow the graph to update with new nodes first
      const timeoutId = setTimeout(() => {
        graphRef.current?.focusOnNodes(highlightedNodeUids);
      }, 100);
      return () => clearTimeout(timeoutId);
    }
  }, [highlightedNodeUids]);

  // Handle causal path selection (toggle behavior)
  const handlePathSelect = useCallback((pathId: string | null) => {
    setSelectedCausalPathId(prev => prev === pathId ? null : pathId);
  }, []);

  // Keyboard shortcuts
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // Don't trigger shortcuts when typing in input fields
      if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) {
        return;
      }

      switch (e.key) {
        case 'Escape':
          // Escape: deselect node
          if (selectedNode) {
            setSelectedNode(null);
          }
          break;
        case '+':
        case '=':
          // +/=: zoom in
          if (selectedNamespace) {
            graphRef.current?.zoomIn();
          }
          break;
        case '-':
        case '_':
          // -/_: zoom out
          if (selectedNamespace) {
            graphRef.current?.zoomOut();
          }
          break;
        case 'f':
        case 'F':
          // F: fit to view
          if (selectedNamespace && !e.ctrlKey && !e.metaKey) {
            graphRef.current?.fitToView();
          }
          break;
        case '0':
          // 0: reset zoom
          if (selectedNamespace && !e.ctrlKey && !e.metaKey) {
            graphRef.current?.resetZoom();
          }
          break;
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [selectedNode, selectedNamespace]);

  // Show loading state while fetching namespaces
  if (namespacesLoading) {
    return (
      <div className="h-full bg-[var(--color-app-bg)]">
        <LoadingSpinner message="Loading namespaces..." />
      </div>
    );
  }

  // Show error if namespaces failed to load
  if (namespacesError) {
    return (
      <div className="h-full bg-[var(--color-app-bg)]">
        <ErrorMessage error={namespacesError} />
      </div>
    );
  }

  // Show empty state if no namespaces available
  if (sortedNamespaces.length === 0) {
    return (
      <div className="h-full bg-[var(--color-app-bg)]">
        <EmptyState />
      </div>
    );
  }

  // Render graph view
  return (
    <div className="h-full flex flex-col bg-[var(--color-app-bg)]">
      {/* Controls bar */}
      <NamespaceGraphControls
        namespace={selectedNamespace ?? sortedNamespaces[0]}
        namespaces={sortedNamespaces}
        onNamespaceChange={setSelectedNamespace}
        kinds={selectedKinds}
        availableKinds={availableKinds}
        onKindsChange={setKinds}
        timestampExpression={timestampExpression}
        onTimestampChange={setTimestampExpression}
        resolvedTimestamp={graphTimestamp}
        lookback={lookback}
        onLookbackChange={setLookback}
      />

      {/* Main content area */}
      <div className="flex-1 flex overflow-hidden min-h-0">
        {/* Graph area */}
        <div className="flex-1 relative overflow-hidden min-w-0">
          {isLoading && !data && (
            <LoadingSpinner message="Loading graph..." />
          )}

          {error && !data && (
            <ErrorMessage error={error} onRetry={refetch} />
          )}

          {filteredData && (
            <>
              <NamespaceGraph
                ref={graphRef}
                data={filteredData}
                onNodeClick={setSelectedNode}
                selectedNodeId={selectedNode?.uid}
                highlightedNodeUids={highlightedNodeUids}
              />
              <ZoomControls
                onZoomIn={() => graphRef.current?.zoomIn()}
                onZoomOut={() => graphRef.current?.zoomOut()}
                onFitToView={() => graphRef.current?.fitToView()}
                onResetZoom={() => graphRef.current?.resetZoom()}
              />
              <GraphLegend />
              {/* Overlay loading indicator when refreshing with existing data */}
              {isLoading && data && (
                <div className="absolute inset-0 bg-[var(--color-app-bg)]/60 backdrop-blur-sm flex items-center justify-center z-20">
                  <div className="ui-popover flex items-center gap-2.5 px-4 py-3">
                    <div className="w-4 h-4 rounded-full border-2 border-[var(--color-border-strong)] border-t-[var(--color-accent)] animate-spin" />
                    <span className="text-[12.5px] text-[var(--color-text-secondary)]">Updating graph...</span>
                  </div>
                </div>
              )}
            </>
          )}
        </div>

        {/* Node detail panel (conditionally rendered) */}
        {selectedNode && (
          <NodeDetailPanel
            node={selectedNode}
            anomalies={selectedNodeAnomalies}
            onClose={() => setSelectedNode(null)}
          />
        )}

        {/* Causal Paths sidebar (always visible when data is loaded) */}
        {data && (
          <CausalPathsSidebar
            causalPaths={data.causalPaths || []}
            selectedPathId={selectedCausalPathId}
            onPathSelect={handlePathSelect}
            isLoading={isLoading}
          />
        )}
      </div>

      {/* Metadata footer */}
      {filteredData && (
        <footer className="ui-statusbar">
          <span className="tabular-nums">
            {filteredData.metadata.nodeCount} resources
            {hasMore && ' (loading more...)'}
          </span>
          <span className="tabular-nums">{filteredData.metadata.edgeCount} relationships</span>
          {data && data.anomalies && data.anomalies.length > 0 && (
            <span className="tabular-nums"><i className="dot" style={{ background: 'var(--color-status-err)' }} />{data.anomalies.length} anomalies</span>
          )}
          {data && data.causalPaths && data.causalPaths.length > 0 && (
            <span className="tabular-nums"><i className="dot" style={{ background: 'var(--color-accent)' }} />{data.causalPaths.length} causal paths</span>
          )}
          {isLoadingMore && (
            <span className="flex items-center gap-1.5 text-[var(--color-text-secondary)]">
              <span className="w-3 h-3 rounded-full border-2 border-[var(--color-border-strong)] border-t-[var(--color-accent)] animate-spin" />
              Loading page {pagesLoaded + 1}...
            </span>
          )}
          <span className="ml-auto flex items-center gap-3 tabular-nums">
            {isLiveMode ? (
              <span className="flex items-center gap-1.5">
                <span className="w-1.5 h-1.5 rounded-full bg-[var(--color-accent)] animate-pulse" />
                Live
              </span>
            ) : (
              <span>Snapshot {formatDateTimeForDisplay(graphTimestamp)}</span>
            )}
            <span>query {filteredData.metadata.queryExecutionMs} ms</span>
          </span>
        </footer>
      )}
    </div>
  );
}
