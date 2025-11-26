import React, { useState, useMemo, useEffect } from 'react';
import { FilterBar } from './components/FilterBar';
import { Timeline } from './components/Timeline';
import { DetailPanel } from './components/DetailPanel';
import { useTimeline } from './hooks/useTimeline';
import { K8sResource, FilterState, SelectedPoint } from './types';

function App() {
  // Selection State
  // We track the specific resource AND the index of the segment selected
  const [selectedPoint, setSelectedPoint] = useState<SelectedPoint | null>(null);

  // Initial Filters
  const [filters, setFilters] = useState<FilterState>({
    kinds: [], // Empty means all
    namespaces: [], // Empty means all
    search: ''
  });

  // Fetch timeline data from backend API
  const { resources, loading, error } = useTimeline({
    filters: filters.namespaces.length > 0 || filters.kinds.length > 0
      ? {
          namespace: filters.namespaces.length > 0 ? filters.namespaces[0] : undefined,
          kind: filters.kinds.length > 0 ? filters.kinds[0] : undefined,
        }
      : undefined
  });

  // Filter Logic
  const filteredResources = useMemo(() => {
    return resources.filter(r => {
      const matchesSearch = r.name.toLowerCase().includes(filters.search.toLowerCase());
      const matchesNs = filters.namespaces.length === 0 || filters.namespaces.includes(r.namespace);
      const matchesKind = filters.kinds.length === 0 || filters.kinds.includes(r.kind);
      return matchesSearch && matchesNs && matchesKind;
    });
  }, [resources, filters]);

  // Derived selected resource object
  const selectedResource = useMemo(() => {
    if (!selectedPoint) return null;
    return resources.find(r => r.id === selectedPoint.resourceId) || null;
  }, [selectedPoint, resources]);

  const relevantEventIds = useMemo(() => {
    if (!selectedResource || !selectedPoint) return [];
    const segment = selectedResource.statusSegments[selectedPoint.index];
    if (!segment) return [];
    return selectedResource.events
      .filter(e => e.timestamp >= segment.start && e.timestamp <= segment.end)
      .map(e => e.id);
  }, [selectedResource, selectedPoint]);

  const handleSegmentClick = (resource: K8sResource, index: number) => {
    setSelectedPoint({ resourceId: resource.id, index });
  };

  const handleClosePanel = () => {
    setSelectedPoint(null);
  }

  // Keyboard Navigation
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (!selectedPoint || !selectedResource) return;

      if (e.key === 'ArrowLeft') {
        e.preventDefault();
        // Move to previous segment
        if (selectedPoint.index > 0) {
          setSelectedPoint({ ...selectedPoint, index: selectedPoint.index - 1 });
        }
      } else if (e.key === 'ArrowRight') {
        e.preventDefault();
        // Move to next segment
        if (selectedPoint.index < selectedResource.statusSegments.length - 1) {
          setSelectedPoint({ ...selectedPoint, index: selectedPoint.index + 1 });
        }
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [selectedPoint, selectedResource]);

  // Responsive D3 container
  const [containerSize, setContainerSize] = useState({ width: 0, height: 0 });
  const containerRef = React.useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handleResize = () => {
      if (containerRef.current) {
        setContainerSize({
          width: containerRef.current.clientWidth,
          height: containerRef.current.clientHeight
        });
      }
    };
    
    window.addEventListener('resize', handleResize);
    handleResize(); // Init

    return () => window.removeEventListener('resize', handleResize);
  }, []);

  return (
    <div className="flex flex-col h-screen bg-gray-950 text-white overflow-hidden font-sans">
      <FilterBar
        filters={filters}
        setFilters={setFilters}
      />

      <div className="flex-1 flex overflow-hidden relative">
        <div className="flex-1 p-4 overflow-hidden flex flex-col" ref={containerRef}>
          {error ? (
            <div className="flex-1 flex items-center justify-center text-red-400 flex-col gap-4">
              <svg className="w-16 h-16 opacity-50" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="1" d="M12 9v2m0 4v2m0 0v2m0-6V9m0 0V7m0 2V5m0 0h-2m2 0h2m-6 0h2m-2 0h-2m0 6h2m-2 0h-2m0 0v2m0-2v-2m0 0h2m-2 0h-2m6 0h2m-2 0h-2" />
              </svg>
              <div className="text-center">
                <p className="text-lg font-semibold mb-2">Failed to load resources</p>
                <p className="text-sm text-gray-400 mb-4">{error.message}</p>
                <button
                  onClick={() => window.location.reload()}
                  className="px-4 py-2 bg-blue-600 hover:bg-blue-700 rounded text-sm transition-colors"
                >
                  Retry
                </button>
              </div>
            </div>
          ) : loading ? (
            <div className="flex-1 flex items-center justify-center text-gray-400 flex-col gap-4">
              <div className="animate-spin">
                <svg className="w-12 h-12" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.5" d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2z" />
                </svg>
              </div>
              <p>Loading resources...</p>
            </div>
          ) : filteredResources.length > 0 ? (
             <Timeline
                resources={filteredResources}
                width={Math.max(containerSize.width - 32, 600)} // Ensure min width
                height={containerSize.height - 32}
                onSegmentClick={handleSegmentClick}
                selectedPoint={selectedPoint}
                highlightedEventIds={relevantEventIds}
                sidebarWidth={selectedPoint ? 384 : 0}
             />
          ) : (
            <div className="flex-1 flex items-center justify-center text-gray-500 flex-col">
                <svg className="w-16 h-16 mb-4 opacity-50" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="1" d="M19.428 15.428a2 2 0 00-1.022-.547l-2.387-.477a6 6 0 00-3.86.517l-.318.158a6 6 0 01-3.86.517L6.05 15.21a2 2 0 00-1.806.547M8 4h8l-1 1v5.172a2 2 0 00.586 1.414l5 5c1.26 1.26.367 3.414-1.415 3.414H4.828c-1.782 0-2.674-2.154-1.414-3.414l5-5A2 2 0 009 10.172V5L8 4z"></path></svg>
                <p>No resources match your filters.</p>
            </div>
          )}
        </div>

        <DetailPanel
            resource={selectedResource}
            selectedIndex={selectedPoint?.index}
            onClose={handleClosePanel}
        />
      </div>
    </div>
  );
}

export default App;