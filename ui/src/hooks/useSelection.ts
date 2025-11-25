import { useCallback, useState } from 'react';
import { SelectedPoint } from '../types';
import { K8sResource } from '../types';

interface UseSelectionResult {
  selectedPoint: SelectedPoint | null;
  setSelectedPoint: (point: SelectedPoint | null) => void;
  selectedResource: K8sResource | null;
  selectedSegmentIndex: number | null;
  clearSelection: () => void;
  selectSegment: (resource: K8sResource, segmentIndex: number) => void;
  nextSegment: (resource: K8sResource) => boolean;
  previousSegment: (resource: K8sResource) => boolean;
}

export const useSelection = (resources: K8sResource[]): UseSelectionResult => {
  const [selectedPoint, setSelectedPoint] = useState<SelectedPoint | null>(null);

  const selectedResource = selectedPoint
    ? resources.find(r => r.id === selectedPoint.resourceId) ?? null
    : null;

  const selectedSegmentIndex = selectedPoint?.index ?? null;

  const clearSelection = useCallback(() => {
    setSelectedPoint(null);
  }, []);

  const selectSegment = useCallback((resource: K8sResource, segmentIndex: number) => {
    // Clamp index to valid range
    const maxIndex = resource.statusSegments.length - 1;
    const validIndex = Math.max(0, Math.min(segmentIndex, maxIndex));

    setSelectedPoint({
      resourceId: resource.id,
      index: validIndex
    });
  }, []);

  const nextSegment = useCallback((resource: K8sResource): boolean => {
    if (!selectedPoint || selectedPoint.resourceId !== resource.id) {
      return false;
    }

    const nextIndex = selectedPoint.index + 1;
    if (nextIndex >= resource.statusSegments.length) {
      return false; // Already at last segment
    }

    selectSegment(resource, nextIndex);
    return true;
  }, [selectedPoint, selectSegment]);

  const previousSegment = useCallback((resource: K8sResource): boolean => {
    if (!selectedPoint || selectedPoint.resourceId !== resource.id) {
      return false;
    }

    const prevIndex = selectedPoint.index - 1;
    if (prevIndex < 0) {
      return false; // Already at first segment
    }

    selectSegment(resource, prevIndex);
    return true;
  }, [selectedPoint, selectSegment]);

  return {
    selectedPoint,
    setSelectedPoint,
    selectedResource,
    selectedSegmentIndex,
    clearSelection,
    selectSegment,
    nextSegment,
    previousSegment
  };
};
