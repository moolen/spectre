import React from 'react';
import { K8sResource } from '../types';

interface DetailPanelProps {
  resource: K8sResource | null;
  selectedIndex?: number;
  onClose: () => void;
}

const ConfigDiff = ({ current, previous }: { current: Record<string, any>, previous: Record<string, any> }) => {
    // Get all unique keys
    const allKeys = Array.from(new Set([...Object.keys(current), ...Object.keys(previous)]));

    return (
        <div className="space-y-1 bg-gray-950 p-2 rounded border border-gray-800 font-mono text-xs overflow-x-auto">
            {allKeys.map(key => {
                const prevVal = previous[key];
                const currVal = current[key];
                const prevStr = JSON.stringify(prevVal);
                const currStr = JSON.stringify(currVal);

                if (prevVal === undefined) {
                    // Added
                    return (
                        <div key={key} className="flex gap-2 text-green-400 bg-green-900/20 px-1 rounded">
                            <span className="w-4 select-none">+</span>
                            <span className="font-semibold">{key}:</span>
                            <span>{currStr}</span>
                        </div>
                    );
                } else if (currVal === undefined) {
                    // Removed
                    return (
                        <div key={key} className="flex gap-2 text-red-400 bg-red-900/20 px-1 rounded">
                            <span className="w-4 select-none">-</span>
                            <span className="font-semibold">{key}:</span>
                            <span>{prevStr}</span>
                        </div>
                    );
                } else if (prevStr !== currStr) {
                    // Changed
                    return (
                        <div key={key} className="flex gap-2 text-yellow-400 bg-yellow-900/20 px-1 rounded">
                            <span className="w-4 select-none">~</span>
                            <span className="font-semibold">{key}:</span>
                            <span className="line-through opacity-70">{prevStr}</span>
                            <span>&rarr;</span>
                            <span>{currStr}</span>
                        </div>
                    );
                } else {
                    // Unchanged
                    return (
                        <div key={key} className="flex gap-2 text-gray-500 px-1">
                            <span className="w-4 select-none opacity-0"> </span>
                            <span className="font-semibold">{key}:</span>
                            <span>{currStr}</span>
                        </div>
                    );
                }
            })}
        </div>
    );
};

export const DetailPanel: React.FC<DetailPanelProps> = ({ resource, selectedIndex = 0, onClose }) => {
  if (!resource) return null;

  const currentSegment = resource.statusSegments[selectedIndex] || resource.statusSegments[0];
  const previousSegment = selectedIndex > 0 ? resource.statusSegments[selectedIndex - 1] : null;

  return (
    <div className="fixed inset-y-0 right-0 w-96 bg-gray-900 border-l border-gray-700 shadow-2xl transform transition-transform duration-300 ease-in-out z-50 overflow-y-auto flex flex-col">
      <div className="p-6 flex-1">
        {/* Header */}
        <div className="flex justify-between items-start mb-6">
          <div>
            <h2 className="text-xl font-bold text-gray-100 break-all">{resource.name}</h2>
            <div className="flex items-center gap-2 mt-1">
                <span className="text-xs font-mono bg-gray-800 px-2 py-0.5 rounded text-gray-400 border border-gray-700">{resource.kind}</span>
                <span className="text-xs text-gray-500">{resource.namespace}</span>
            </div>
          </div>
          <button onClick={onClose} className="text-gray-400 hover:text-white transition-colors p-1 hover:bg-gray-800 rounded">
            <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M6 18L18 6M6 6l12 12"></path></svg>
          </button>
        </div>

        {/* Selected Segment Details */}
        <div className="mb-6">
            <div className="flex items-center justify-between mb-2">
                <div className="text-sm text-gray-500 uppercase tracking-wider font-semibold">
                    Version {selectedIndex + 1} of {resource.statusSegments.length}
                </div>
                <div className="flex gap-1">
                     <span className={`w-2 h-2 rounded-full ${
                        currentSegment.status === 'Ready' ? 'bg-emerald-500' : 
                        currentSegment.status === 'Error' ? 'bg-red-500' : 'bg-amber-500'
                     }`}></span>
                     <span className="text-xs text-gray-400">{currentSegment.status}</span>
                </div>
            </div>
            
            <div className="bg-gray-800 rounded p-3 border border-gray-700 space-y-2 mb-4">
                 <div className="text-xs text-gray-400 flex justify-between">
                    <span>Start:</span>
                    <span className="text-gray-200">{currentSegment.start.toLocaleTimeString()}</span>
                 </div>
                 <div className="text-xs text-gray-400 flex justify-between">
                    <span>End:</span>
                    <span className="text-gray-200">{currentSegment.end.toLocaleTimeString()}</span>
                 </div>
                 <div className="text-xs text-gray-300 italic border-t border-gray-700 pt-2 mt-2">
                    {currentSegment.message}
                 </div>
            </div>

            {/* Config Diff */}
            <div className="mb-4">
                <div className="text-sm text-gray-500 uppercase tracking-wider font-semibold mb-2">Configuration Changes</div>
                {previousSegment ? (
                    <ConfigDiff current={currentSegment.config} previous={previousSegment.config} />
                ) : (
                     <div className="bg-gray-800/50 p-2 rounded text-xs text-gray-500 italic border border-gray-800">
                        Initial state (no previous version)
                        <div className="mt-2 text-gray-400 not-italic">
                            {Object.entries(currentSegment.config).map(([k,v]) => (
                                <div key={k}>{k}: {JSON.stringify(v)}</div>
                            ))}
                        </div>
                     </div>
                )}
            </div>
        </div>

        {/* Audit Logs */}
        <div>
            <div className="text-sm text-gray-500 uppercase tracking-wider font-semibold mb-2">Relevant Events</div>
            <div className="space-y-3">
                {resource.events.filter(e => e.timestamp >= currentSegment.start && e.timestamp <= currentSegment.end).length === 0 ? (
                    <div className="text-gray-600 text-sm italic">No events in this time window.</div>
                ) : (
                    resource.events
                    .filter(e => e.timestamp >= currentSegment.start && e.timestamp <= currentSegment.end)
                    .sort((a,b) => b.timestamp.getTime() - a.timestamp.getTime())
                    .map((event) => (
                        <div key={event.id} className="bg-gray-800 p-3 rounded border-l-4 border-gray-600 hover:bg-gray-750 transition-colors">
                            <div className="flex justify-between items-start mb-1">
                                <span className="text-xs font-mono text-gray-500">{event.timestamp.toLocaleTimeString()}</span>
                                <span className={`text-xs px-2 py-0.5 rounded uppercase font-bold ${
                                    event.verb === 'delete' ? 'bg-red-900/50 text-red-200' :
                                    event.verb === 'create' || event.verb === 'patch' || event.verb === 'update' ? 'bg-blue-900/50 text-blue-200' :
                                    'bg-gray-700 text-gray-300'
                                }`}>{event.verb}</span>
                            </div>
                            <div className="text-sm text-gray-200 mb-1 leading-snug">{event.message}</div>
                            <div className="text-xs text-gray-500 flex justify-between">
                                <span>User: {event.user}</span>
                            </div>
                        </div>
                    ))
                )}
            </div>
        </div>
      </div>
      
      {/* Footer Hint */}
      <div className="p-3 bg-gray-950 border-t border-gray-800 text-center text-xs text-gray-600">
        Use Arrow Left/Right to navigate history
      </div>
    </div>
  );
};