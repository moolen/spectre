import React, { useState, useRef, useCallback, useMemo } from 'react';
import {
  ObservatoryGraph,
  ObservatoryGraphHandle,
  ObservatoryZoomControls,
  ObservatoryNodeDetail,
  ObservatoryLegend,
} from '../components/Observatory';
import { useObservatoryGraph } from '../hooks/useObservatoryGraph';
import { D3ObservatoryNode, ObservatoryNodeType } from '../types/observatoryGraph';
import { SelectDropdown } from '../components/SelectDropdown';

// All available node types for the filter
const NODE_TYPE_OPTIONS: ObservatoryNodeType[] = [
  'SignalAnchor',
  'Alert',
  'Dashboard',
  'Panel',
  'Query',
  'Metric',
  'Service',
  'Workload',
  'SignalBaseline',
];

/**
 * Observatory page for visualizing SignalAnchors, Alerts, Dashboards, and their relationships
 */
export default function ObservatoryPage() {
  const [selectedNode, setSelectedNode] = useState<D3ObservatoryNode | null>(null);
  const [nodeSearch, setNodeSearch] = useState<string>('');
  const [namespace, setNamespace] = useState<string | null>(null);
  const [workload, setWorkload] = useState<string | null>(null);
  const [includeBaselines, setIncludeBaselines] = useState(false);
  const [selectedNodeTypes, setSelectedNodeTypes] = useState<string[]>([]);
  const graphRef = useRef<ObservatoryGraphHandle>(null);

  const { data, isLoading, error, refetch } = useObservatoryGraph({
    namespace: namespace || undefined,
    workload: workload || undefined,
    includeBaselines,
    limit: 200,
  });

  // Extract available namespaces from Workload nodes
  const availableNamespaces = useMemo(() => {
    if (!data?.graph?.nodes) return [];
    const namespaces = new Set<string>();
    for (const node of data.graph.nodes) {
      if (node.type === 'Workload' && node.properties?.namespace) {
        namespaces.add(node.properties.namespace as string);
      }
    }
    return Array.from(namespaces).sort();
  }, [data]);

  // Extract available workloads for the selected namespace from Workload nodes
  const availableWorkloads = useMemo(() => {
    if (!data?.graph?.nodes) return [];
    const workloads = new Set<string>();
    for (const node of data.graph.nodes) {
      if (node.type === 'Workload' && node.label) {
        // If namespace is selected, only show workloads for that namespace
        if (!namespace || node.properties?.namespace === namespace) {
          workloads.add(node.label);
        }
      }
    }
    return Array.from(workloads).sort();
  }, [data, namespace]);

  // Reset workload when namespace changes
  const handleNamespaceChange = useCallback((value: string | string[] | null) => {
    const newNamespace = value as string | null;
    setNamespace(newNamespace);
    // Reset workload filter when namespace changes
    setWorkload(null);
  }, []);

  const handleNodeClick = useCallback((node: D3ObservatoryNode | null) => {
    setSelectedNode(node);
  }, []);

  // Filter graph data based on selected node types and search query
  const filteredData = useMemo(() => {
    if (!data) return null;

    const searchLower = nodeSearch.toLowerCase().trim();
    const hasTypeFilter = selectedNodeTypes.length > 0;
    const hasSearchFilter = searchLower.length > 0;

    // If no filters, show all
    if (!hasTypeFilter && !hasSearchFilter) {
      return data;
    }

    // Filter nodes by type and/or search query
    const visibleNodes = data.graph.nodes.filter(node => {
      const matchesType = !hasTypeFilter || selectedNodeTypes.includes(node.type);
      const matchesSearch = !hasSearchFilter || node.label.toLowerCase().includes(searchLower);
      return matchesType && matchesSearch;
    });
    const visibleNodeIds = new Set(visibleNodes.map(n => n.id));

    // Filter edges to only include those between visible nodes
    const visibleEdges = data.graph.edges.filter(edge =>
      visibleNodeIds.has(edge.source) && visibleNodeIds.has(edge.target)
    );

    return {
      ...data,
      graph: {
        nodes: visibleNodes,
        edges: visibleEdges,
      },
      metadata: {
        ...data.metadata,
        nodeCount: visibleNodes.length,
        edgeCount: visibleEdges.length,
      },
    };
  }, [data, selectedNodeTypes, nodeSearch]);

  return (
    <div className="h-full flex flex-col bg-[#111111]">
      {/* Control bar */}
      <div className="px-4 py-3 border-b border-[#2a2a2a] bg-[#1a1a1a] flex items-center gap-4">
        <input
          type="text"
          value={nodeSearch}
          onChange={e => setNodeSearch(e.target.value)}
          placeholder="Search nodes..."
          className="px-3 py-1.5 bg-[#111111] border border-[#2a2a2a] rounded text-sm text-white placeholder-gray-500 focus:outline-none focus:border-purple-500 w-48"
        />
        <SelectDropdown
          label="All Namespaces"
          options={availableNamespaces}
          selected={namespace}
          onChange={handleNamespaceChange}
          multiple={false}
          minWidth="160px"
        />
        <SelectDropdown
          label="All Workloads"
          options={availableWorkloads}
          selected={workload}
          onChange={(value) => setWorkload(value as string | null)}
          multiple={false}
          minWidth="160px"
        />
        <SelectDropdown
          label="All Types"
          options={NODE_TYPE_OPTIONS}
          selected={selectedNodeTypes}
          onChange={(value) => setSelectedNodeTypes(value as string[])}
          multiple={true}
          minWidth="160px"
        />
        <label className="flex items-center gap-2 text-sm text-gray-400 cursor-pointer">
          <input
            type="checkbox"
            checked={includeBaselines}
            onChange={e => setIncludeBaselines(e.target.checked)}
            className="rounded border-gray-600 bg-[#111111] text-purple-500 focus:ring-purple-500"
          />
          Include Baselines
        </label>
        <button
          onClick={refetch}
          className="px-3 py-1.5 bg-purple-600 hover:bg-purple-700 text-white rounded text-sm font-medium transition-colors"
        >
          Refresh
        </button>
        {isLoading && (
          <div className="flex items-center gap-2 text-gray-400 text-sm">
            <svg className="animate-spin w-4 h-4" viewBox="0 0 24 24" fill="none">
              <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
              <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
            </svg>
            Loading...
          </div>
        )}
      </div>

      {/* Main content */}
      <div className="flex-1 flex overflow-hidden min-h-0">
        {/* Graph area */}
        <div className="flex-1 relative overflow-hidden min-w-0">
          {/* Loading state - shown when loading and no data yet */}
          {isLoading && !filteredData && !error && (
            <div className="absolute inset-0 flex items-center justify-center">
              <div className="text-center">
                <svg className="animate-spin w-12 h-12 text-purple-500 mx-auto mb-4" viewBox="0 0 24 24" fill="none">
                  <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                  <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
                </svg>
                <h3 className="text-gray-400 font-semibold mb-2">Loading Observatory Graph</h3>
                <p className="text-gray-500 text-sm">Fetching signal anchors, alerts, and dashboards...</p>
              </div>
            </div>
          )}
          {error && (
            <div className="absolute inset-0 flex items-center justify-center">
              <div className="bg-red-900/20 border border-red-500/50 rounded-lg p-4 max-w-md">
                <h3 className="text-red-400 font-semibold mb-2">Error loading graph</h3>
                <p className="text-gray-400 text-sm">{error.message}</p>
                <button
                  onClick={refetch}
                  className="mt-3 px-3 py-1.5 bg-red-600 hover:bg-red-700 text-white rounded text-sm font-medium transition-colors"
                >
                  Retry
                </button>
              </div>
            </div>
          )}
          {/* Fallback for unexpected null data after loading */}
          {!isLoading && !error && !filteredData && (
            <div className="absolute inset-0 flex items-center justify-center">
              <div className="text-center">
                <div className="text-4xl mb-4">⚠️</div>
                <h3 className="text-gray-400 font-semibold mb-2">Unable to load data</h3>
                <p className="text-gray-500 text-sm max-w-md mb-4">
                  The Observatory data could not be loaded. This might be a temporary issue.
                </p>
                <button
                  onClick={refetch}
                  className="px-3 py-1.5 bg-purple-600 hover:bg-purple-700 text-white rounded text-sm font-medium transition-colors"
                >
                  Try Again
                </button>
              </div>
            </div>
          )}
          {!error && filteredData && filteredData.graph.nodes.length === 0 && (
            <div className="absolute inset-0 flex items-center justify-center">
              <div className="text-center">
                <div className="text-4xl mb-4">🔭</div>
                {(selectedNodeTypes.length > 0 || nodeSearch.trim()) ? (
                  <>
                    <h3 className="text-gray-400 font-semibold mb-2">No matching nodes</h3>
                    <p className="text-gray-500 text-sm max-w-md">
                      No nodes match the current filters. Try adjusting your search or type selection.
                    </p>
                  </>
                ) : (
                  <>
                    <h3 className="text-gray-400 font-semibold mb-2">No Observatory data found</h3>
                    <p className="text-gray-500 text-sm max-w-md">
                      No SignalAnchors, Dashboards, or Alerts have been synced yet.
                      Configure a Grafana integration and run a sync to populate the graph.
                    </p>
                  </>
                )}
              </div>
            </div>
          )}
          {!error && filteredData && filteredData.graph.nodes.length > 0 && (
            <>
              <ObservatoryGraph
                ref={graphRef}
                data={filteredData}
                onNodeClick={handleNodeClick}
                selectedNodeId={selectedNode?.id}
              />
              <ObservatoryZoomControls
                onZoomIn={() => graphRef.current?.zoomIn()}
                onZoomOut={() => graphRef.current?.zoomOut()}
                onFitToView={() => graphRef.current?.fitToView()}
                onResetZoom={() => graphRef.current?.resetZoom()}
              />
              <div className="absolute bottom-4 left-4">
                <ObservatoryLegend />
              </div>
            </>
          )}
        </div>

        {/* Detail panel */}
        {selectedNode && (
          <ObservatoryNodeDetail
            node={selectedNode}
            onClose={() => setSelectedNode(null)}
          />
        )}
      </div>

      {/* Footer */}
      <div className="px-4 py-2 border-t border-[#2a2a2a] bg-[#1a1a1a] flex items-center justify-between text-xs text-gray-500">
        <div>
          {filteredData && (
            <>
              {filteredData.metadata.nodeCount} nodes, {filteredData.metadata.edgeCount} edges
              {(selectedNodeTypes.length > 0 || nodeSearch.trim()) && data && (
                <span className="text-gray-600 ml-1">
                  (filtered from {data.metadata.nodeCount} total)
                </span>
              )}
            </>
          )}
        </div>
        <div>
          {data && (
            <>Query: {data.metadata.queryExecutionMs}ms</>
          )}
        </div>
      </div>
    </div>
  );
}
