import React, { useEffect, useRef, useMemo, useState } from 'react';
import * as d3 from 'd3';
import { K8sResource, SelectedPoint, TimeRange } from '../types';
import { STATUS_COLORS } from '../constants';
import { useSettings } from '../hooks/useSettings';

interface TimelineProps {
  resources: K8sResource[];
  onSegmentClick: (resource: K8sResource, index: number) => void;
  selectedPoint: SelectedPoint | null;
  highlightedEventIds?: string[];
  sidebarWidth?: number;
  timeRange: TimeRange;
  onVisibleTimeRangeChange?: (timeRange: TimeRange) => void;
  onZoomDetected?: () => void;
  hasMore?: boolean;
  loadingMore?: boolean;
  onLoadMore?: () => void;
}

const MARGIN = { top: 36, right: 24, bottom: 20, left: 300 };
const ROW_HEIGHT_DEFAULT = 40;
const ROW_HEIGHT_COMPACT = 26;
// Status bars are drawn thinner than the row; the full row stays clickable.
const BAR_HEIGHT_DEFAULT = 8;
const BAR_HEIGHT_COMPACT = 6;
const LABEL_PADDING_X = 20;

export const Timeline: React.FC<TimelineProps> = ({
    resources,
    onSegmentClick,
    selectedPoint,
    highlightedEventIds = [],
    sidebarWidth = 0,
    timeRange,
    onVisibleTimeRangeChange,
    onZoomDetected,
    hasMore = false,
    loadingMore = false,
    onLoadMore,
}) => {
  const svgRef = useRef<SVGSVGElement>(null);
  const stickyAxisRef = useRef<SVGSVGElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const [containerSize, setContainerSize] = useState({ width: 0, height: 0 });
  const [tooltip, setTooltip] = useState<{ x: number; y: number; content: string } | null>(null);
  const { compactMode, formatTime, theme } = useSettings();
  const debounceTimeoutRef = useRef<NodeJS.Timeout | null>(null);

  const rowHeight = compactMode ? ROW_HEIGHT_COMPACT : ROW_HEIGHT_DEFAULT;
  const barHeight = compactMode ? BAR_HEIGHT_COMPACT : BAR_HEIGHT_DEFAULT;
  const eventRadius = compactMode ? 3 : 4;
  const bandPadding = 0;

  const themeColors = useMemo(() => {
    if (typeof window === 'undefined') {
      return {
        sidebar: '#0d1117',
        border: '#374151',
        textPrimary: '#f8fafc',
        textMuted: '#94a3b8',
        grid: '#334155',
        appBg: '#0f172a',
        surfaceMuted: '#1f2937'
      };
    }
    const styles = getComputedStyle(document.documentElement);
    return {
      sidebar: styles.getPropertyValue('--color-sidebar-bg').trim() || '#0d1117',
      border: styles.getPropertyValue('--color-border-soft').trim() || '#374151',
      textPrimary: styles.getPropertyValue('--color-text-primary').trim() || '#f8fafc',
      textMuted: styles.getPropertyValue('--color-text-muted').trim() || '#94a3b8',
      grid: styles.getPropertyValue('--color-grid-line').trim() || '#334155',
      appBg: styles.getPropertyValue('--color-app-bg').trim() || '#0f172a',
      surfaceMuted: styles.getPropertyValue('--color-surface-muted').trim() || '#1f2937'
    };
  }, [theme]);

  // Measure container size with ResizeObserver
  // Ignore width changes during sidebar animation to prevent layout thrashing
  const resizeTimeoutRef = useRef<NodeJS.Timeout | null>(null);
  const lastSizeRef = useRef({ width: 0, height: 0 });
  const isInitializedRef = useRef(false);
  
  useEffect(() => {
    if (!containerRef.current) return;

    const resizeObserver = new ResizeObserver((entries) => {
      for (const entry of entries) {
        const { width, height } = entry.contentRect;
        
        // Always update on first observation
        if (!isInitializedRef.current) {
          isInitializedRef.current = true;
          lastSizeRef.current = { width, height };
          setContainerSize({ width, height });
          return;
        }
        
        // Only respond to height changes, not width changes
        // Width changes are typically caused by sidebar animation and shouldn't trigger re-render
        // The SVG will stretch/shrink naturally via CSS
        const heightDiff = Math.abs(height - lastSizeRef.current.height);
        if (heightDiff < 1) return;
        
        // Clear any pending resize update
        if (resizeTimeoutRef.current) {
          clearTimeout(resizeTimeoutRef.current);
        }
        
        // Debounce height updates
        resizeTimeoutRef.current = setTimeout(() => {
          lastSizeRef.current = { width: lastSizeRef.current.width, height };
          setContainerSize(prev => ({ ...prev, height }));
          resizeTimeoutRef.current = null;
        }, 50);
      }
    });

    resizeObserver.observe(containerRef.current);
    return () => {
      resizeObserver.disconnect();
      if (resizeTimeoutRef.current) {
        clearTimeout(resizeTimeoutRef.current);
      }
    };
  }, []);

  // Scroll detection for lazy loading
  useEffect(() => {
    if (!containerRef.current || !hasMore || !onLoadMore) return;
    const container = containerRef.current;
    const handleScroll = () => {
      const { scrollTop, scrollHeight, clientHeight } = container;
      const scrolledToBottom = scrollHeight - scrollTop - clientHeight < 200; // 200px threshold
      if (scrolledToBottom && !loadingMore) {
        onLoadMore();
      }
    };

    container.addEventListener('scroll', handleScroll);
    return () => container.removeEventListener('scroll', handleScroll);
  }, [hasMore, loadingMore, onLoadMore]);

  const width = containerSize.width || 800;
  const height = containerSize.height || 600;

  // Use the user-selected time range as the domain
  const timeDomain = useMemo(() => {
    // Add small padding (2%) to the selected range for visual breathing room
    const duration = timeRange.end.getTime() - timeRange.start.getTime();
    const padding = duration * 0.02;
    return [
      new Date(timeRange.start.getTime() - padding),
      new Date(timeRange.end.getTime() + padding)
    ] as [Date, Date];
  }, [timeRange]);

  const prevDomain = useRef(timeDomain);

  // API already provides resources sorted by namespace, kind, and name
  // No need for client-side sorting - this improves performance significantly
  const sortedResources = resources;
  const timelineStartMs = timeRange.start.getTime();

  // Scales (memoized to prevent flicker during non-data updates)
  const innerWidth = width - MARGIN.left - MARGIN.right;
  const contentHeight = sortedResources.length * rowHeight;
  const minInnerHeight = height - MARGIN.top - MARGIN.bottom;
  const innerHeight = Math.max(minInnerHeight, contentHeight);

  const xScale = useMemo(() => d3.scaleTime()
      .domain(timeDomain)
      .range([0, innerWidth]), [timeDomain, innerWidth]);

  const yScale = useMemo(() => d3.scaleBand()
      .domain(sortedResources.map(r => r.id))
      .range([0, contentHeight])
      .padding(bandPadding), [sortedResources, contentHeight, bandPadding]);

  // Define Zoom Behavior - constrained to the configured time range
  const zoom = useMemo(() => d3.zoom<SVGSVGElement, unknown>()
      .scaleExtent([1, 30000]) // Min scale 1 = can't zoom out beyond full view, max 30000 for millisecond precision
      .translateExtent([[0, 0], [innerWidth, height]]) // Limit panning to content area
      .extent([[0, 0], [innerWidth, height]]), [innerWidth, height]);

  // Notify about visible time range changes with debouncing
  // This updates the URL with the current visible range (including padding)
  const notifyVisibleTimeRange = (transform: d3.ZoomTransform) => {
    if (!onVisibleTimeRangeChange) return;

    if (debounceTimeoutRef.current) {
      clearTimeout(debounceTimeoutRef.current);
    }

    debounceTimeoutRef.current = setTimeout(() => {
      const rescaledXScale = transform.rescaleX(xScale);
      // Get the visible range including the timeline's 2% padding
      // This matches the actual domain being displayed
      const visibleStart = rescaledXScale.invert(0);
      const visibleEnd = rescaledXScale.invert(innerWidth);
      onVisibleTimeRangeChange({ start: visibleStart, end: visibleEnd });
      debounceTimeoutRef.current = null;
    }, 300);
  };

  // Cleanup debounce timer on unmount
  useEffect(() => {
    return () => {
      if (debounceTimeoutRef.current) {
        clearTimeout(debounceTimeoutRef.current);
      }
    };
  }, []);

  // Main Draw Effect: Handles structure, data binding, and initial render
  useEffect(() => {
    if (!svgRef.current || sortedResources.length === 0) return;

    const svg = d3.select(svgRef.current);

    // Check if domain changed to decide on zoom reset
    const domainChanged = prevDomain.current !== timeDomain;
    prevDomain.current = timeDomain;

    let currentTransform = d3.zoomIdentity;
    if (!domainChanged) {
       currentTransform = d3.zoomTransform(svg.node()!);
    }

    svg.selectAll("*").remove(); // Clear previous render

    // --- Definitions ---
    const defs = svg.append('defs');
    defs.append('clipPath')
        .attr('id', 'chart-clip')
        .append('rect')
        .attr('x', 0)
        .attr('y', 0)
        .attr('width', innerWidth)
        .attr('height', innerHeight);

    // --- Main Groups ---
    const mainGroup = svg.append('g')
      .attr('transform', `translate(${MARGIN.left},${MARGIN.top})`);

    const gridGroup = mainGroup.append('g')
        .attr('class', 'grid-group')
        .attr('clip-path', 'url(#chart-clip)');

    const contentGroup = mainGroup.append('g')
      .attr('class', 'content-group')
      .attr('clip-path', 'url(#chart-clip)');

    const brushGroup = mainGroup.append('g')
      .attr('class', 'brush');

    const labelGroup = svg.append('g')
      .attr('transform', `translate(0, ${MARGIN.top})`);

    // Read sidebar colors from CSS to ensure theme is applied
    const styles = getComputedStyle(document.documentElement);
    const sidebarBg = styles.getPropertyValue('--color-sidebar-bg').trim() || themeColors.sidebar;
    const borderColor = styles.getPropertyValue('--color-border-soft').trim() || themeColors.border;

    // Sidebar Background - read from CSS to ensure theme is applied
    labelGroup.append('rect')
        .attr('data-element', 'sidebar-bg')
        .attr('width', MARGIN.left - 1)
        .attr('height', Math.max(height, contentHeight + MARGIN.top + MARGIN.bottom))
        .attr('y', -MARGIN.top)
        .attr('fill', sidebarBg)
        .style('fill', 'var(--color-sidebar-bg)')
        .attr('stroke', 'none');

    // Border color - read from CSS to ensure theme is applied
    labelGroup.append('line')
        .attr('data-element', 'sidebar-border')
        .attr('x1', MARGIN.left - 1)
        .attr('x2', MARGIN.left - 1)
        .attr('y1', -MARGIN.top)
        .attr('y2', Math.max(height, contentHeight + MARGIN.top + MARGIN.bottom))
        .attr('stroke', borderColor)
        .style('stroke', 'var(--color-border-soft)')
        .attr('stroke-width', 1);

    // --- Draw Labels ---
    const labels = labelGroup.selectAll('.label')
      .data(sortedResources)
      .enter()
      .append('g')
      .attr('class', 'label cursor-pointer group')
      .attr('transform', d => `translate(${LABEL_PADDING_X}, ${yScale(d.id)})`)
      .on('click', (event, d) => {
        if (d.statusSegments.length > 0) {
            onSegmentClick(d, d.statusSegments.length - 1);
        }
      });

    labels.append('rect')
        .attr('class', 'label-bg')
        .attr('x', -LABEL_PADDING_X)
        .attr('y', 0)
        .attr('width', MARGIN.left - 1)
        .attr('height', yScale.bandwidth())
        .attr('fill', 'transparent');

    // Accent edge for the selected row (toggled in the selection effect)
    labels.append('rect')
        .attr('class', 'label-accent')
        .attr('x', -LABEL_PADDING_X)
        .attr('y', 0)
        .attr('width', 2)
        .attr('height', yScale.bandwidth())
        .style('fill', 'var(--color-accent)')
        .attr('opacity', 0);

    // Calculate max width for text (leave padding on right)
    const maxTextWidth = MARGIN.left - LABEL_PADDING_X * 2;

    // Text colors - re-read from CSS to ensure theme is applied
    const textPrimary = getComputedStyle(document.documentElement).getPropertyValue('--color-text-primary').trim() || themeColors.textPrimary;
    const textMuted = getComputedStyle(document.documentElement).getPropertyValue('--color-text-muted').trim() || themeColors.textMuted;

    // Resource name with truncation and tooltip
    const nameFontSize = compactMode ? '11px' : '12.5px';
    const kindFontSize = compactMode ? '9.5px' : '11px';
    const nameY = compactMode ? yScale.bandwidth() / 2 - 5 : yScale.bandwidth() / 2 - 7;
    const kindY = compactMode ? yScale.bandwidth() / 2 + 6 : yScale.bandwidth() / 2 + 8;

    const nameText = labels.append('text')
      .attr('data-element', 'label-name')
      .attr('x', 0)
      .attr('y', nameY)
      .attr('fill', textPrimary)
      .style('fill', 'var(--color-text-primary)')
      .style('font-size', nameFontSize)
      .style('font-weight', '500')
      .style('dominant-baseline', 'middle')
      .each(function(d) {
        const text = d3.select(this);
        const textNode = this;
        const fullName = d.name;

        // Set initial text
        text.text(fullName);

        // Measure and truncate if needed
        const bbox = textNode.getBBox();
        if (bbox.width > maxTextWidth) {
          // Truncate with ellipsis
          let truncated = fullName;
          while (truncated.length > 0) {
            text.text(truncated + '...');
            const newBbox = textNode.getBBox();
            if (newBbox.width <= maxTextWidth || truncated.length <= 1) {
              break;
            }
            truncated = truncated.slice(0, -1);
          }
        }
      });

    // Add tooltip to show full name on hover
    nameText.append('title')
      .text(d => d.name);

    labels.append('text')
      .attr('data-element', 'label-kind-ns')
      .text(d => `${d.kind} · ${d.namespace}${d.preExisting ? ' · pre-existing' : ''}`)
      .attr('x', 0)
      .attr('y', kindY)
      .attr('fill', textMuted)
      .style('fill', 'var(--color-text-faint)')
      .style('font-size', kindFontSize)
      .style('dominant-baseline', 'middle');

    // --- Draw Content ---
    const rows = contentGroup.selectAll('.resource-row')
        .data(sortedResources)
        .enter()
        .append('g')
        .attr('class', 'resource-row')
        .attr('transform', d => `translate(0, ${yScale(d.id)})`);
    // Row background, highlighted when the row is selected
    rows.append('rect')
        .attr('class', 'row-bg')
        .attr('x', -MARGIN.left)
        .attr('y', 0)
        .attr('width', 100000)
        .attr('height', yScale.bandwidth())
        .attr('fill', 'transparent')
        .attr('pointer-events', 'none');

    const barY = (yScale.bandwidth() - barHeight) / 2;

    // Segments
    rows.selectAll('.segment')
        .data(d => d.statusSegments.map((s, i) => ({
          ...s,
          resourceId: d.id,
          index: i,
          preExisting: d.preExisting,
          showContinuationCue: Boolean(d.preExisting && i === 0 && s.start.getTime() <= timelineStartMs),
        })))
        .enter()
        .append('rect')
        .attr('class', 'segment')
        // Restrict transition to stroke properties to avoid lag on x/width updates during zoom
        .style('transition', 'stroke 0.2s, stroke-width 0.2s')
        .attr('y', barY)
        .attr('height', barHeight)
        .attr('rx', 2)
        .attr('data-status', s => s.status)
        .style('fill', s => STATUS_COLORS[s.status])
        .attr('opacity', 1)
        .attr('stroke', 'none')
        .attr('stroke-width', 0);
        // Note: stroke/selection is handled in separate effect

    rows.selectAll('.segment-continuation')
        .data(d => d.statusSegments
          .map((s, i) => ({
            ...s,
            resourceId: d.id,
            index: i,
            showContinuationCue: Boolean(d.preExisting && i === 0 && s.start.getTime() <= timelineStartMs),
          }))
          .filter(s => s.showContinuationCue))
        .enter()
        .append('rect')
        .attr('class', 'segment-continuation')
        .attr('y', barY)
        .attr('height', barHeight)
        .attr('rx', 2)
        .style('fill', s => STATUS_COLORS[s.status])
        .attr('opacity', 0.35)
        .attr('pointer-events', 'none')
        .each(function() {
          d3.select(this)
            .append('title')
            .text('Continues before selected range');
        });

    // Events
    const eventDots = rows.selectAll('.event-dot')
        .data(d => d.events.map(e => ({ ...e, resourceId: d.id })))
        .enter()
        .append('circle')
        .attr('class', 'event-dot')
        .attr('cy', yScale.bandwidth() / 2)
        .attr('r', eventRadius)
        .style('fill', 'var(--color-text-primary)')
        .style('stroke', 'var(--color-app-bg)')
        .attr('stroke-width', 2)
        .style('cursor', 'pointer')
        .style('pointer-events', 'all')
        .on('mouseenter', function(event, d) {
          const time = formatTime(d.timestamp);
          const typeLabel = d.type === 'Warning' ? '⚠️ Warning' : d.type === 'Normal' ? '✓ Normal' : d.type;
          const content = `${time}\n${typeLabel}: ${d.reason}\n${d.message}`;
          const [x, y] = d3.pointer(event, containerRef.current);
          setTooltip({ x, y, content });
        })
        .on('mousemove', function(event) {
          const [x, y] = d3.pointer(event, containerRef.current);
          setTooltip(prev => prev ? { ...prev, x, y } : null);
        })
        .on('mouseleave', () => {
          setTooltip(null);
        });

    // --- Zoom Update Function ---
    const updateChart = (transform: d3.ZoomTransform) => {
        const newXScale = transform.rescaleX(xScale);

        const axis = d3.axisTop(newXScale)
            .ticks(Math.max(width / 120, 2))
            .tickSizeOuter(0)
            .tickFormat((date: any) => formatTime(date));

        // Update sticky axis
        if (stickyAxisRef.current) {
            const stickyAxis = d3.select(stickyAxisRef.current);
            const stickyAxisGroup = stickyAxis.select('g#sticky-axis');

            stickyAxisGroup.call(axis)
                .call(g => g.select('.domain').remove())
                .call(g => g.selectAll('.tick line').remove())
                .call(g => g.selectAll('.tick text')
                    .style('fill', 'var(--color-text-faint)')
                    .attr('font-weight', '400')
                    .attr('font-size', '11px')
                    .style('font-family', 'var(--font-mono)')
                    .attr('dy', '-12px')
                );
        }

        const gridAxis = d3.axisBottom(newXScale)
            .ticks(Math.max(width / 120, 2))
            .tickSize(innerHeight)
            .tickFormat(() => '');

        gridGroup.call(gridAxis)
            .call(g => g.select('.domain').remove())
            .call(g => g.selectAll('.tick line')
                .style('stroke', 'var(--color-grid-line)')
                .attr('stroke-opacity', 1)
            );

        contentGroup.selectAll<SVGRectElement, any>('.segment')
            .attr('x', d => newXScale(d.start))
            .attr('width', d => {
                const calculatedWidth = newXScale(d.end) - newXScale(d.start);
                // Give terminating/deleted resources a minimal visible length
                if (d.status === 'Terminating') {
                    return 2;
                }
                return Math.max(4, calculatedWidth);
            });

        contentGroup.selectAll<SVGRectElement, any>('.segment-continuation')
            .attr('x', d => newXScale(d.start))
            .attr('width', d => {
                const segmentWidth = Math.max(4, newXScale(d.end) - newXScale(d.start));
                return Math.min(10, segmentWidth);
            });

        contentGroup.selectAll<SVGCircleElement, any>('.event-dot')
            .attr('cx', d => newXScale(d.timestamp));

        clusterEvents();
    };

    // Collapse event dots that would overlap at the current zoom level into a
    // single pill with a count (e.g. a crash-looping pod), instead of drawing
    // dozens of overlapping dots.
    const clusterMinGap = eventRadius * 2 + 2;
    const clusterEvents = () => {
        contentGroup.selectAll<SVGGElement, K8sResource>('.resource-row').each(function() {
            const row = d3.select(this);
            const items = row.selectAll<SVGCircleElement, unknown>('.event-dot').nodes()
                .map(node => ({ node, x: Number(node.getAttribute('cx')) }))
                .sort((a, b) => a.x - b.x);

            const clusters: Array<{ x0: number; x1: number; count: number; nodes: SVGCircleElement[] }> = [];
            for (const item of items) {
                const current = clusters[clusters.length - 1];
                if (current && item.x - current.x1 < clusterMinGap) {
                    current.x1 = item.x;
                    current.count++;
                    current.nodes.push(item.node);
                } else {
                    clusters.push({ x0: item.x, x1: item.x, count: 1, nodes: [item.node] });
                }
            }
            for (const cluster of clusters) {
                const display = cluster.count > 1 ? 'none' : null;
                cluster.nodes.forEach(node => d3.select(node).style('display', display));
            }

            const pillHeight = Math.max(12, eventRadius * 2 + 6);
            const pills = row.selectAll<SVGGElement, (typeof clusters)[number]>('.event-cluster')
                .data(clusters.filter(c => c.count > 1))
                .join(enter => {
                    const g = enter.append('g').attr('class', 'event-cluster').style('pointer-events', 'none');
                    g.append('rect')
                        .attr('height', pillHeight)
                        .attr('rx', pillHeight / 2)
                        .style('fill', 'var(--color-app-bg)')
                        .attr('fill-opacity', 0.6)
                        .style('stroke', 'var(--color-text-secondary)')
                        .attr('stroke-width', 1);
                    g.append('text')
                        .attr('text-anchor', 'middle')
                        .attr('dominant-baseline', 'central')
                        .style('fill', 'var(--color-text-primary)')
                        .style('font-size', compactMode ? '9px' : '10px')
                        .style('font-weight', '600');
                    return g;
                });
            pills.attr('transform', `translate(0, ${(yScale.bandwidth() - pillHeight) / 2})`);
            pills.select('rect')
                .attr('x', c => c.x0 - pillHeight / 2)
                .attr('width', c => Math.max(c.x1 - c.x0 + pillHeight, String(c.count).length * 6 + 14));
            pills.select('text')
                .attr('x', c => c.x0 - pillHeight / 2 + Math.max(c.x1 - c.x0 + pillHeight, String(c.count).length * 6 + 14) / 2)
                .attr('y', pillHeight / 2)
                .text(c => `${c.count}`);
        });
    };

    zoom.on('zoom', (event) => {
        updateChart(event.transform);
        notifyVisibleTimeRange(event.transform);
        onZoomDetected?.();
    });

    svg.call(zoom)
       .on("wheel.zoom", null)
       .on("mousedown.zoom", null)
       .on("dblclick.zoom", () => {
           svg.transition().duration(750).call(zoom.transform, d3.zoomIdentity);
       });

    svg.on("wheel", (event) => {
        const isHorizontal = Math.abs(event.deltaX) > Math.abs(event.deltaY);
        if (isHorizontal) {
            event.preventDefault();
            const t = d3.zoomTransform(svg.node()!);
            // Apply aggressive zoom-aware damping: inversely proportional to zoom level
            const dampingFactor = Math.max(0.00001, 1 / t.k);
            const dampedDelta = event.deltaX * dampingFactor;
            // Cap maximum translation speed to prevent extreme jumps
            const maxDelta = 50;
            const finalDelta = Math.max(-maxDelta, Math.min(maxDelta, dampedDelta));
            zoom.translateBy(svg, -finalDelta, 0);
        }
    }, { passive: false });

    const brush = d3.brushX()
        .extent([[0, 0], [innerWidth, innerHeight]])
        .on("end", (event) => {
            if (!event.selection) return;
            const [x0, x1] = event.selection;
            brushGroup.call(brush.move, null);

            const t = d3.zoomTransform(svg.node()!);
            const currentXScale = t.rescaleX(xScale);
            const x0_orig = currentXScale.invert(x0);
            const x1_orig = currentXScale.invert(x1);
            const dx = xScale(x1_orig) - xScale(x0_orig);
            const k = innerWidth / dx;
            const tx = -xScale(x0_orig) * k;

            const newTransform = d3.zoomIdentity.translate(tx, 0).scale(k);
            svg.transition().duration(750)
               .call(zoom.transform, newTransform)
               .on('end', () => {
                 notifyVisibleTimeRange(newTransform);
               });
        });

    brushGroup.call(brush);

    // Manual Hit Testing for Brush Overlay
    brushGroup.select('.overlay')
        .on('click', (event) => {
             const [x, y] = d3.pointer(event);
             const eachBand = yScale.step();
             const index = Math.floor(y / eachBand);
             const resource = sortedResources[index];

             if (!resource) return;
             const bandTop = yScale(resource.id) || 0;
             const bandHeight = yScale.bandwidth();
             if (y < bandTop || y > bandTop + bandHeight) return;

             const t = d3.zoomTransform(svg.node()!);
             const currentXScale = t.rescaleX(xScale);
             const clickedTime = currentXScale.invert(x);

             const segmentIndex = resource.statusSegments.findIndex(s =>
                 clickedTime >= s.start && clickedTime <= s.end
             );

             if (segmentIndex !== -1) {
                 onSegmentClick(resource, segmentIndex);
             }
        })
        .on('mousemove', function(event) {
             const [x, y] = d3.pointer(event);
             const eachBand = yScale.step();
             const index = Math.floor(y / eachBand);
             const resource = sortedResources[index];
             let isOverSegment = false;
             let isOverEvent = false;

             if (resource) {
                 const bandTop = yScale(resource.id) || 0;
                 const bandHeight = yScale.bandwidth();
                 if (y >= bandTop && y <= bandTop + bandHeight) {
                    const t = d3.zoomTransform(svg.node()!);
                    const currentXScale = t.rescaleX(xScale);
                    const hoverTime = currentXScale.invert(x);
                    isOverSegment = resource.statusSegments.some(s =>
                        hoverTime >= s.start && hoverTime <= s.end
                    );

                    // Check if hovering over an event dot
                    const hitRadius = 8; // Slightly larger hit area for easier hovering
                    const eventDot = resource.events.find(e => {
                        const eventX = currentXScale(e.timestamp);
                        const eventY = bandTop + bandHeight / 2;
                        const dx = x - eventX;
                        const dy = y - eventY;
                        return Math.sqrt(dx * dx + dy * dy) <= hitRadius;
                    });

                    if (eventDot) {
                        isOverEvent = true;
                        const time = formatTime(eventDot.timestamp);
                        const typeLabel = eventDot.type === 'Warning' ? '⚠️ Warning' : eventDot.type === 'Normal' ? '✓ Normal' : eventDot.type;
                        const content = `${time}\n${typeLabel}: ${eventDot.reason}\n${eventDot.message}`;
                        // Get absolute coordinates relative to the container
                        const [absX, absY] = d3.pointer(event, containerRef.current);
                        setTooltip({ x: absX, y: absY, content });
                    } else {
                        setTooltip(null);
                    }
                 } else {
                     setTooltip(null);
                 }
             } else {
                 setTooltip(null);
             }

             d3.select(this).style('cursor', (isOverSegment || isOverEvent) ? 'pointer' : 'crosshair');
        })
        .on('mouseleave', () => {
             setTooltip(null);
        });

    // Apply initial transform if exists
    svg.call(zoom.transform, currentTransform);

  }, [sortedResources, width, height, timeDomain, themeColors, theme, rowHeight, bandPadding, barHeight, eventRadius]); // Only re-run if data/layout or display settings change

  // Theme changes need no redraw: every colour above resolves through CSS
  // variables (see src/theme.css).

  // Secondary Effect: Style Updates (Selection & Highlights)
  // This runs whenever selection changes, without re-drawing the whole chart
  useEffect(() => {
    if (!svgRef.current) return;
    const svg = d3.select(svgRef.current);

    const selectedResourceId = selectedPoint?.resourceId;

    // Selected row: subtle background plus an accent edge on the label
    svg.selectAll('.row-bg')
        .style('fill', (d: any) => d.id === selectedResourceId ? 'var(--color-surface-muted)' : 'transparent');
    svg.selectAll('.label-bg')
        .style('fill', (d: any) => d.id === selectedResourceId ? 'var(--color-surface-muted)' : 'transparent');
    svg.selectAll('.label-accent')
        .attr('opacity', (d: any) => d.id === selectedResourceId ? 1 : 0);

    // Selected segment: outlined and slightly taller
    const isSelected = (d: any) => Boolean(selectedPoint && d.resourceId === selectedPoint.resourceId && d.index === selectedPoint.index);
    const bandwidth = rowHeight;
    const baseY = (bandwidth - barHeight) / 2;
    svg.selectAll('.segment')
        .attr('y', (d: any) => isSelected(d) ? baseY - 2 : baseY)
        .attr('height', (d: any) => isSelected(d) ? barHeight + 4 : barHeight)
        // Set both the attribute and the style: the e2e tests read the attribute
        .attr('stroke', (d: any) => isSelected(d) ? 'var(--color-text-primary)' : 'none')
        .style('stroke', (d: any) => isSelected(d) ? 'var(--color-text-primary)' : 'none')
        .attr('stroke-width', (d: any) => isSelected(d) ? 1.5 : 0);

    // Events that belong to the selected segment are drawn in the accent colour
    svg.selectAll('.event-dot')
        .style('fill', (d: any) => highlightedEventIds.includes(d.id) ? 'var(--color-accent)' : 'var(--color-text-primary)')
        .attr('r', (d: any) => highlightedEventIds.includes(d.id) ? eventRadius + 1.5 : eventRadius)
        // Bring highlighted events to front
        .filter((d: any) => highlightedEventIds.includes(d.id))
        .raise();

  }, [selectedPoint, highlightedEventIds, themeColors, theme, sortedResources, rowHeight, barHeight, eventRadius]);

  // Track previous sidebar width to detect panel closing
  const prevSidebarWidth = useRef(0);

  // Separate effect to handle panel closing - preserves view position when zoomed
  useEffect(() => {
    if (!svgRef.current || width === 0) return;

    const wasPanelOpen = prevSidebarWidth.current > 0;
    const isPanelClosing = wasPanelOpen && sidebarWidth === 0;

    if (isPanelClosing) {
        const svg = d3.select(svgRef.current);
        const currentTransform = d3.zoomTransform(svg.node()!);
        const currentXScale = currentTransform.rescaleX(xScale);

        // Calculate the time at the right edge of the visible area BEFORE panel closes
        // The old inner width was smaller because the container was narrower
        const oldInnerWidth = innerWidth; // This is already the current (smaller) inner width
        const visibleRightX = oldInnerWidth - MARGIN.right;
        const rightEdgeTime = currentXScale.invert(visibleRightX);

        // Small delay to ensure container has resized
        setTimeout(() => {
            if (!svgRef.current || !containerRef.current) return;
            const svg = d3.select(svgRef.current);
            const updatedTransform = d3.zoomTransform(svg.node()!);

            // Get new dimensions after resize
            const newWidth = containerRef.current.clientWidth;
            const newInnerWidth = newWidth - MARGIN.left - MARGIN.right;

            // Create new base scale for the expanded container
            const newBaseXScale = d3.scaleTime()
                .domain(timeDomain)
                .range([0, newInnerWidth]);

            // Calculate where the right edge time is in the new scale
            const rightTimeX = newBaseXScale(rightEdgeTime);

            // We want this time to be at the right edge of the visible area
            // Target position: newInnerWidth - MARGIN.right (in chart coordinates)
            // Equation: rightTimeX * k + tx = targetX
            // Therefore: tx = targetX - rightTimeX * k
            const k = updatedTransform.k;
            const targetX = newInnerWidth - MARGIN.right;
            const newTx = targetX - (rightTimeX * k);

            // Clamp to valid range (can't pan past the timeline bounds)
            const maxTx = 0;
            const minTx = newInnerWidth - (newInnerWidth * k);
            const finalTx = Math.min(maxTx, Math.max(minTx, newTx));

            svg.transition()
               .duration(400)
               .ease(d3.easeCubicOut)
               .call(zoom.transform, d3.zoomIdentity.translate(finalTx, 0).scale(k));
        }, 50);
    }

    prevSidebarWidth.current = sidebarWidth;
  }, [sidebarWidth, width, zoom, timeDomain, xScale, innerWidth]);

  // Tertiary Effect: Auto-Center Navigation
  useEffect(() => {
    if (!svgRef.current) return;

    const svg = d3.select(svgRef.current);
    const currentTransform = d3.zoomTransform(svg.node()!);

    // Normal auto-center behavior when a segment is selected
    if (selectedPoint) {
        const resource = sortedResources.find(r => r.id === selectedPoint.resourceId);
        if (resource) {
            const segment = resource.statusSegments[selectedPoint.index];
            if (segment) {
                const startTime = segment.start;
                const endTime = segment.end;
                const midTime = new Date((startTime.getTime() + endTime.getTime()) / 2);

                // Calculate Available Width for Content
                // Visible area starts at MARGIN.left and ends at width - sidebarWidth
                const visibleLeft = MARGIN.left;
                const visibleRight = width - sidebarWidth;
                const availableWidth = Math.max(0, visibleRight - visibleLeft);

                // Determine target scale (k)
                // If segment width > availableWidth (minus padding), zoom out
                const padding = availableWidth * 0.1; // 10% total horizontal padding for context
                const maxSegmentWidth = availableWidth - padding;

                const x0 = xScale(startTime);
                const x1 = xScale(endTime);
                const segmentWidthUnzoomed = x1 - x0;

                let k = currentTransform.k;
                const projectedWidth = segmentWidthUnzoomed * k;

                // Only zoom out if the segment is larger than the visible space
                if (projectedWidth > maxSegmentWidth && segmentWidthUnzoomed > 0) {
                     k = maxSegmentWidth / segmentWidthUnzoomed;
                }

                // Calculate target translation (tx) to center the segment
                // Logic: ScreenCenterX = MARGIN.left + (UnzoomedCenterX * k + tx)
                const targetCenterX = visibleLeft + availableWidth / 2;
                const pointOriginalX = xScale(midTime);
                const newTx = targetCenterX - MARGIN.left - (pointOriginalX * k);

                svg.transition()
                   .duration(500)
                   .ease(d3.easeCubicOut)
                   .call(zoom.transform, d3.zoomIdentity.translate(newTx, 0).scale(k));
            }
        }
    }

    // Update ref at the end of the effect
    prevSidebarWidth.current = sidebarWidth;
  }, [selectedPoint, sidebarWidth, sortedResources, width, xScale, zoom, timeRange]);

  const zoomBy = (factor: number) => {
    if (!svgRef.current) return;
    const svg = d3.select(svgRef.current);
    const currentTransform = d3.zoomTransform(svg.node()!);
    const newScale = Math.min(30000, Math.max(1, currentTransform.k * factor));
    const centerX = innerWidth / 2;
    // Zoom around the center: keep the point at centerX fixed
    const newTx = centerX - (centerX - currentTransform.x) * (newScale / currentTransform.k);
    const newTransform = d3.zoomIdentity.translate(newTx, 0).scale(newScale);
    svg.transition()
      .duration(300)
      .ease(d3.easeCubicOut)
      .call(zoom.transform, newTransform)
      .on('end', () => {
        notifyVisibleTimeRange(newTransform);
        onZoomDetected?.();
      });
  };

  const resetZoom = () => {
    if (!svgRef.current) return;
    const svg = d3.select(svgRef.current);
    svg.transition()
      .duration(300)
      .ease(d3.easeCubicOut)
      .call(zoom.transform, d3.zoomIdentity)
      .on('end', () => {
        notifyVisibleTimeRange(d3.zoomIdentity);
        onZoomDetected?.();
      });
  };

  const zoomButtonClass =
    'w-[26px] h-[26px] grid place-items-center rounded-md text-[var(--color-text-faint)] hover:text-[var(--color-text-primary)] hover:bg-[var(--color-surface-muted)] transition-colors pointer-events-auto';

  return (
    <div
      ref={containerRef}
      className="w-full h-full bg-[var(--color-app-bg)] custom-scrollbar relative overflow-y-auto overflow-x-hidden"
    >
      {/* Sticky header: resource column title + zoom controls, and the time axis */}
      <div
        className="sticky top-0 left-0 right-0 z-10 bg-[var(--color-app-bg)] border-b border-[var(--color-border-soft)]"
        style={{ height: MARGIN.top }}
      >
        <svg
          ref={stickyAxisRef}
          width={width}
          height={MARGIN.top}
          className="block"
          style={{ pointerEvents: 'none' }}
        >
          <g
            id="sticky-axis"
            transform={`translate(${MARGIN.left},${MARGIN.top})`}
          />
        </svg>
        <div
          className="absolute top-0 left-0 h-full flex items-center justify-between border-r border-[var(--color-border-soft)] bg-[var(--color-app-bg)]"
          style={{ width: MARGIN.left, paddingLeft: LABEL_PADDING_X, paddingRight: 10 }}
        >
          <span className="text-[11.5px] text-[var(--color-text-faint)]">
            Resources <span className="opacity-70 tabular-nums">{sortedResources.length}{hasMore ? '+' : ''}</span>
          </span>
          <div className="flex items-center gap-0.5">
            <button type="button" onClick={() => zoomBy(0.8)} title="Zoom Out" aria-label="Zoom Out" className={zoomButtonClass}>
              <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" strokeWidth={1.75} viewBox="0 0 24 24">
                <circle cx="11" cy="11" r="7" /><path d="m20 20-3.5-3.5M8 11h6" strokeLinecap="round" />
              </svg>
            </button>
            <button type="button" onClick={() => zoomBy(1.25)} title="Zoom In" aria-label="Zoom In" className={zoomButtonClass}>
              <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" strokeWidth={1.75} viewBox="0 0 24 24">
                <circle cx="11" cy="11" r="7" /><path d="m20 20-3.5-3.5M11 8v6M8 11h6" strokeLinecap="round" />
              </svg>
            </button>
            <button type="button" onClick={resetZoom} title="Reset Zoom" aria-label="Reset Zoom" className={zoomButtonClass}>
              <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" strokeWidth={1.75} viewBox="0 0 24 24" strokeLinecap="round">
                <path d="M8 3H5a2 2 0 0 0-2 2v3M21 8V5a2 2 0 0 0-2-2h-3M3 16v3a2 2 0 0 0 2 2h3M16 21h3a2 2 0 0 0 2-2v-3" />
              </svg>
            </button>
          </div>
        </div>
      </div>

      <svg
        ref={svgRef}
        width={width}
        height={Math.max(height, sortedResources.length * rowHeight + MARGIN.top + MARGIN.bottom)}
        className="block"
        style={{ overflow: 'visible', marginTop: -MARGIN.top }}
      />
      {tooltip && (
        <div
          className="ui-popover fixed z-50 px-3 py-2.5 pointer-events-none max-w-sm"
          style={{
            left: `${tooltip.x + 10}px`,
            top: `${tooltip.y + 10}px`,
            transform: 'translate(0, 0)'
          }}
        >
          <pre className="text-xs leading-relaxed text-[var(--color-text-primary)] whitespace-pre-wrap font-sans">
            {tooltip.content}
          </pre>
        </div>
      )}
      {/* Loading indicator for pagination */}
      {loadingMore && (
        <div className="sticky bottom-0 left-0 right-0 flex items-center justify-center py-3 bg-[var(--color-app-bg)] border-t border-[var(--color-border-soft)] z-10">
          <div className="flex items-center gap-2 text-[var(--color-text-faint)]">
            <div className="w-3.5 h-3.5 rounded-full border-2 border-[var(--color-border-strong)] border-t-[var(--color-accent)] animate-spin" />
            <span className="text-xs">Loading more resources...</span>
          </div>
        </div>
      )}
    </div>
  );
};
