/**
 * Timeline Utilities for D3-based Gantt chart
 * Provides scale creation, coordinate calculations, and viewport optimization
 */

import * as d3 from 'd3';
import { K8sResource } from '../types';

export const TIMELINE_CONSTANTS = {
  MARGIN: { top: 40, right: 30, bottom: 20, left: 240 },
  ROW_HEIGHT: 48,
};

/**
 * Calculate the time domain for all resources
 * Returns min/max timestamps with padding
 */
export function calculateTimeDomain(resources: K8sResource[]): [Date, Date] {
  let min = new Date();
  let max = new Date(0);

  if (resources.length === 0) {
    const now = new Date();
    return [new Date(now.getTime() - 3600000), now];
  }

  resources.forEach(r => {
    r.statusSegments.forEach(s => {
      if (s.start < min) min = s.start;
      if (s.end > max) max = s.end;
    });
  });

  const duration = max.getTime() - min.getTime();
  return [
    new Date(min.getTime() - duration * 0.05),
    new Date(max.getTime() + duration * 0.05),
  ];
}

/**
 * Create a D3 time scale for X-axis
 */
export function createTimeScale(
  domain: [Date, Date],
  rangeWidth: number
): d3.ScaleTime<number, number> {
  return d3.scaleTime().domain(domain).range([0, rangeWidth]);
}

/**
 * Create a D3 band scale for Y-axis (resource rows)
 */
export function createBandScale(
  resources: K8sResource[],
  totalHeight: number
): d3.ScaleBand<string> {
  return d3
    .scaleBand<string>()
    .domain(resources.map(r => r.id))
    .range([0, totalHeight])
    .padding(0.4);
}

/**
 * Calculate inner dimensions given container and margin
 */
export function calculateDimensions(
  containerWidth: number,
  containerHeight: number,
  resourceCount: number,
  margin: { top: number; right: number; bottom: number; left: number } = TIMELINE_CONSTANTS.MARGIN
) {
  const innerWidth = containerWidth - margin.left - margin.right;
  const contentHeight = resourceCount * TIMELINE_CONSTANTS.ROW_HEIGHT;
  const minInnerHeight = containerHeight - margin.top - margin.bottom;
  const innerHeight = Math.max(minInnerHeight, contentHeight);

  return {
    innerWidth,
    innerHeight,
    contentHeight,
    margin,
  };
}

/**
 * Calculate whether a segment is visible in the current viewport
 * Used for viewport culling to optimize rendering
 */
export function isSegmentVisible(
  segmentStart: Date,
  segmentEnd: Date,
  viewportStart: Date,
  viewportEnd: Date
): boolean {
  // Segment is visible if it overlaps with viewport
  return segmentStart < viewportEnd && segmentEnd > viewportStart;
}

/**
 * Filter resources to show only visible ones in current viewport
 * Considers both time range and vertical scroll position
 */
export function getVisibleResources(
  resources: K8sResource[],
  yScale: d3.ScaleBand<string>,
  containerHeight: number,
  scrollTop: number = 0
): K8sResource[] {
  const bandWidth = yScale.bandwidth();

  return resources.filter(resource => {
    const y = yScale(resource.id);
    if (y === undefined) return false;

    // Check if resource row is visible in vertical viewport
    return y + bandWidth > scrollTop && y < scrollTop + containerHeight;
  });
}

/**
 * Create zoom behavior for D3
 */
export function createZoomBehavior(
  innerWidth: number,
  containerHeight: number
): d3.Zoom<SVGSVGElement, unknown> {
  return d3
    .zoom<SVGSVGElement, unknown>()
    .scaleExtent([0.1, 1000])
    .translateExtent([[0, 0], [innerWidth * 10, containerHeight]])
    .extent([[0, 0], [innerWidth, containerHeight]]);
}

/**
 * Convert time to pixel position on X-axis
 */
export function timeToPixel(
  time: Date,
  scale: d3.ScaleTime<number, number>
): number {
  const pixel = scale(time);
  return isNaN(pixel) ? 0 : pixel;
}

/**
 * Convert pixel position to time on X-axis (inverse mapping)
 */
export function pixelToTime(
  pixel: number,
  scale: d3.ScaleTime<number, number>
): Date {
  return scale.invert(pixel);
}

/**
 * Calculate center and zoom for a segment to fit in viewport
 */
export interface SegmentViewConfig {
  targetX: number; // Center X position
  targetZoom: number; // Zoom level
}

export function calculateSegmentView(
  segmentStart: Date,
  segmentEnd: Date,
  xScale: d3.ScaleTime<number, number>,
  containerWidth: number,
  padding: number = 0.1
): SegmentViewConfig {
  const startPixel = timeToPixel(segmentStart, xScale);
  const endPixel = timeToPixel(segmentEnd, xScale);
  const segmentWidth = Math.abs(endPixel - startPixel);

  const viewportWidth = xScale.range()[1] - xScale.range()[0];
  const availableWidth = viewportWidth * (1 - padding * 2);

  // If segment fits in viewport, use 1x zoom
  let zoom = 1;
  if (segmentWidth > availableWidth && segmentWidth > 0) {
    zoom = availableWidth / segmentWidth;
  }

  const centerPixel = (startPixel + endPixel) / 2;

  return {
    targetX: centerPixel,
    targetZoom: zoom,
  };
}

/**
 * Smooth transition configuration for D3
 */
export const TRANSITION_CONFIG = {
  duration: 300,
  easing: d3.easeCubicInOut,
};
