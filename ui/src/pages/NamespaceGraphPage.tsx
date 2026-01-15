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
  <div className="flex flex-col items-center justify-center h-full">
    <div className="w-10 h-10 border-4 border-blue-500 border-t-transparent rounded-full animate-spin mb-4" />
    <span className="text-[var(--color-text-muted)]">{message}</span>
  </div>
);

/**
 * Error message component
 */
const ErrorMessage: React.FC<{ error: Error; onRetry?: () => void }> = ({ error, onRetry }) => (
  <div className="flex flex-col items-center justify-center h-full p-8">
    <svg className="w-16 h-16 text-red-500 mb-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} 
            d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
    </svg>
    <h3 className="text-lg font-medium text-[var(--color-text-primary)] mb-2">Failed to load graph</h3>
    <p className="text-sm text-[var(--color-text-muted)] mb-4 text-center max-w-md">
      {error.message}
    </p>
    {onRetry && (
      <button
        onClick={onRetry}
        className="px-4 py-2 rounded-md bg-blue-600 hover:bg-blue-700 text-white text-sm transition-colors"
      >
        Try Again
      </button>
    )}
  </div>
);

/**
 * Empty state component when no namespaces are available
 */
const EmptyState: React.FC = () => (
  <div className="flex flex-col items-center justify-center h-full p-8">
    <svg className="w-16 h-16 text-[var(--color-text-muted)] opacity-50 mb-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} 
            d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4" />
    </svg>
    <h3 className="text-lg font-medium text-[var(--color-text-primary)] mb-2">No Namespaces Found</h3>
    <p className="text-sm text-[var(--color-text-muted)] text-center max-w-md">
      No namespaces are available in the cluster. Make sure resources are being monitored.
    </p>
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

  // Sort namespaces alphabetically
  const sortedNamespaces = useMemo(() => [...namespaces].sort(), [namespaces]);

  // Persisted namespace from localStorage
  const { namespace: persistedNamespace, setNamespace: setPersistedNamespace } = usePersistedGraphNamespace(sortedNamespaces);

  // Parse URL parameters - namespace from URL takes priority
  const urlNamespace = searchParams.get('namespace');
  
  // Effective namespace: URL param > persisted > null
  const selectedNamespace = urlNamespace || persistedNamespace;

  // Update URL and persist when namespace changes
  const setSelectedNamespace = useCallback((ns: string | null) => {
    // Update URL
    if (ns) {
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
    if (!urlNamespace && persistedNamespace && sortedNamespaces.includes(persistedNamespace)) {
      setSearchParams({ namespace: persistedNamespace }, { replace: true });
    }
  }, [urlNamespace, persistedNamespace, sortedNamespaces, setSearchParams]);

  // Get default kinds from settings
  const { defaultKinds } = useSettings();

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
  }, [timestampExpression, selectedNamespace]);

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

  // Filter graph data by selected kinds (client-side)
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
    const filteredNodes = selectedKinds.length > 0
      ? data.graph.nodes.filter(node => 
          selectedKinds.includes(node.kind) || causalPathNodeUids?.has(node.uid)
        )
      : [];
    
    // If no kinds are selected OR selected kinds don't match any graph nodes,
    // show ALL nodes from the graph data (graceful fallback instead of empty state)
    const nodesToShow = filteredNodes.length > 0 ? filteredNodes : data.graph.nodes;
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
  }, [data, selectedKinds, selectedCausalPathId]);

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
        namespace={selectedNamespace || sortedNamespaces[0]}
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
              {/* Overlay loading indicator when refreshing with existing data */}
              {isLoading && data && (
                <div className="absolute inset-0 bg-[var(--color-app-bg)]/60 backdrop-blur-sm flex items-center justify-center z-20">
                  <div className="flex flex-col items-center gap-3 p-4 rounded-lg bg-[var(--color-surface-elevated)] border border-[var(--color-border-soft)] shadow-xl">
                    <div className="w-8 h-8 border-3 border-blue-500 border-t-transparent rounded-full animate-spin" />
                    <span className="text-sm text-[var(--color-text-muted)]">Updating graph...</span>
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
        <div className="px-4 py-2 border-t border-[var(--color-border-soft)] bg-[var(--color-surface-muted)]
                        text-xs text-[var(--color-text-muted)] flex items-center gap-4">
          <span>
            {filteredData.metadata.nodeCount} resources
            {hasMore && ' (loading more...)'}
          </span>
          <span>{filteredData.metadata.edgeCount} relationships</span>
          {data && data.anomalies && data.anomalies.length > 0 && (
            <span className="text-red-400">{data.anomalies.length} anomalies</span>
          )}
          {data && data.causalPaths && data.causalPaths.length > 0 && (
            <span className="text-blue-400">{data.causalPaths.length} causal paths</span>
          )}
          {isLoadingMore && (
            <span className="text-blue-400 flex items-center gap-1">
              <svg className="w-3 h-3 animate-spin" viewBox="0 0 24 24" fill="none">
                <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"/>
                <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
              </svg>
              Loading page {pagesLoaded + 1}...
            </span>
          )}
          <span className="ml-auto flex items-center gap-3">
            {isLiveMode ? (
              <span className="flex items-center gap-1">
                <span className="w-2 h-2 rounded-full bg-green-400 animate-pulse" />
                Live
              </span>
            ) : (
              <span>Snapshot: {formatDateTimeForDisplay(graphTimestamp)}</span>
            )}
            <span>Query: {filteredData.metadata.queryExecutionMs}ms</span>
          </span>
        </div>
      )}
    </div>
  );
}
