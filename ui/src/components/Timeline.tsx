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
}

const MARGIN = { top: 40, right: 30, bottom: 20, left: 240 };
const ROW_HEIGHT_DEFAULT = 48;
const ROW_HEIGHT_COMPACT = 24;

export const Timeline: React.FC<TimelineProps> = ({
    resources,
    onSegmentClick,
    selectedPoint,
    highlightedEventIds = [],
    sidebarWidth = 0,
    timeRange,
    onVisibleTimeRangeChange,
    onZoomDetected
}) => {
  const svgRef = useRef<SVGSVGElement>(null);
  const stickyAxisRef = useRef<SVGSVGElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const [containerSize, setContainerSize] = useState({ width: 0, height: 0 });
  const [tooltip, setTooltip] = useState<{ x: number; y: number; content: string } | null>(null);
  const { compactMode, formatTime, theme } = useSettings();
  const debounceTimeoutRef = useRef<NodeJS.Timeout | null>(null);

  const rowHeight = compactMode ? ROW_HEIGHT_COMPACT : ROW_HEIGHT_DEFAULT;
  const bandPadding = compactMode ? 0.15 : 0.4;

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
  useEffect(() => {
    if (!containerRef.current) return;

    const resizeObserver = new ResizeObserver((entries) => {
      for (const entry of entries) {
        const { width, height } = entry.contentRect;
        setContainerSize({ width, height });
      }
    });

    resizeObserver.observe(containerRef.current);
    return () => resizeObserver.disconnect();
  }, []);

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
        .attr('width', MARGIN.left - 2)
        .attr('height', Math.max(height, contentHeight + MARGIN.top + MARGIN.bottom))
        .attr('y', -MARGIN.top)
        .attr('fill', sidebarBg)
        .attr('stroke', 'none');

    // Border color - read from CSS to ensure theme is applied
    labelGroup.append('line')
        .attr('data-element', 'sidebar-border')
        .attr('x1', MARGIN.left - 1)
        .attr('x2', MARGIN.left - 1)
        .attr('y1', -MARGIN.top)
        .attr('y2', Math.max(height, contentHeight + MARGIN.top + MARGIN.bottom))
        .attr('stroke', borderColor)
        .attr('stroke-width', 1);

    // --- Draw Labels ---
    const labels = labelGroup.selectAll('.label')
      .data(sortedResources)
      .enter()
      .append('g')
      .attr('class', 'label cursor-pointer group')
      .attr('transform', d => `translate(20, ${yScale(d.id)})`)
      .on('click', (event, d) => {
        if (d.statusSegments.length > 0) {
            onSegmentClick(d, d.statusSegments.length - 1);
        }
      });

    labels.append('rect')
        .attr('x', -20)
        .attr('y', 0)
        .attr('width', MARGIN.left - 2)
        .attr('height', yScale.bandwidth())
        .attr('fill', 'transparent');

    // Calculate max width for text (leave padding on right)
    const maxTextWidth = MARGIN.left - 40;

    // Text colors - re-read from CSS to ensure theme is applied
    const textPrimary = getComputedStyle(document.documentElement).getPropertyValue('--color-text-primary').trim() || themeColors.textPrimary;
    const textMuted = getComputedStyle(document.documentElement).getPropertyValue('--color-text-muted').trim() || themeColors.textMuted;

    // Resource name with truncation and tooltip
    const nameFontSize = compactMode ? '10px' : '13px';
    const kindFontSize = compactMode ? '8px' : '11px';
    const nameY = compactMode ? yScale.bandwidth() / 2 - 2 : yScale.bandwidth() / 2 - 6;
    const kindY = compactMode ? yScale.bandwidth() / 2 + 6 : yScale.bandwidth() / 2 + 10;

    const nameText = labels.append('text')
      .attr('data-element', 'label-name')
      .attr('x', 0)
      .attr('y', nameY)
      .attr('fill', textPrimary)
      .style('font-size', nameFontSize)
      .style('font-weight', '600')
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
      .text(d => `${d.kind} • ${d.namespace}`)
      .attr('x', 0)
      .attr('y', kindY)
      .attr('fill', textMuted)
      .style('font-size', kindFontSize)
      .style('dominant-baseline', 'middle');

    labels.append('line')
        .attr('x1', 0)
        .attr('x2', MARGIN.left - 40)
        .attr('y1', yScale.bandwidth() + yScale.padding() * yScale.step() / 2)
        .attr('y2', yScale.bandwidth() + yScale.padding() * yScale.step() / 2)
        .attr('stroke', borderColor)
        .attr('stroke-width', 1);

    // --- Draw Content ---
    const rows = contentGroup.selectAll('.resource-row')
        .data(sortedResources)
        .enter()
        .append('g')
        .attr('class', 'resource-row')
        .attr('transform', d => `translate(0, ${yScale(d.id)})`);

    rows.append('line')
        .attr('class', 'row-guide')
        .attr('x1', -10000)
        .attr('x2', 10000)
        .attr('y1', yScale.bandwidth() / 2)
        .attr('y2', yScale.bandwidth() / 2)
        .attr('stroke', themeColors.grid)
        .attr('stroke-dasharray', '4,4')
        .attr('opacity', 0.5);

    // Segments
    rows.selectAll('.segment')
        .data(d => d.statusSegments.map((s, i) => ({ ...s, resourceId: d.id, index: i })))
        .enter()
        .append('rect')
        .attr('class', 'segment')
        // Restrict transition to stroke properties to avoid lag on x/width updates during zoom
        .style('transition', 'stroke 0.2s, stroke-width 0.2s')
        .attr('y', 0)
        .attr('height', yScale.bandwidth())
        .attr('rx', 4)
        .attr('fill', s => STATUS_COLORS[s.status])
        .attr('stroke', 'none')
        .attr('stroke-width', 0);
        // Note: stroke/selection is handled in separate effect

    // Events
    const eventDots = rows.selectAll('.event-dot')
        .data(d => d.events.map(e => ({ ...e, resourceId: d.id })))
        .enter()
        .append('circle')
        .attr('class', 'event-dot')
        .attr('cy', yScale.bandwidth() / 2)
        .attr('r', 5)
        .attr('fill', '#f8fafc')
        .attr('stroke', '#1e293b') // Dark outline for visibility in both themes
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
                .call(g => g.selectAll('.tick line').attr('stroke', themeColors.grid).attr('stroke-width', 2).attr('y2', -5))
                .call(g => g.selectAll('.tick text')
                    .attr('fill', themeColors.textPrimary)
                    .attr('font-weight', '600')
                    .attr('font-size', '12px')
                    .attr('dy', '-8px')
                );
        }

        const gridAxis = d3.axisBottom(newXScale)
            .ticks(Math.max(width / 120, 2))
            .tickSize(innerHeight)
            .tickFormat(() => '');

        gridGroup.call(gridAxis)
            .call(g => g.select('.domain').remove())
            .call(g => g.selectAll('.tick line')
                .attr('stroke', themeColors.grid)
                .attr('stroke-opacity', 0.5)
                .attr('stroke-dasharray', '2,2')
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

        contentGroup.selectAll<SVGCircleElement, any>('.event-dot')
            .attr('cx', d => newXScale(d.timestamp));
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
                    const eventRadius = 8; // Slightly larger hit area for easier hovering
                    const eventDot = resource.events.find(e => {
                        const eventX = currentXScale(e.timestamp);
                        const eventY = bandTop + bandHeight / 2;
                        const dx = x - eventX;
                        const dy = y - eventY;
                        return Math.sqrt(dx * dx + dy * dy) <= eventRadius;
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

  }, [sortedResources, width, height, timeDomain, themeColors, theme, rowHeight, bandPadding]); // Only re-run if data/layout or display settings change

  // Theme Update Effect: Update all theme-dependent colors when theme changes
  useEffect(() => {
    if (!svgRef.current) return;

    // Use requestAnimationFrame to ensure CSS has been applied
    const frame = requestAnimationFrame(() => {
      const svg = d3.select(svgRef.current);

      // Read current CSS values directly from document
      const styles = getComputedStyle(document.documentElement);
      const sidebarBg = styles.getPropertyValue('--color-sidebar-bg').trim();
      const borderColor = styles.getPropertyValue('--color-border-soft').trim();
      const textPrimary = styles.getPropertyValue('--color-text-primary').trim();
      const textMuted = styles.getPropertyValue('--color-text-muted').trim();
      const gridLine = styles.getPropertyValue('--color-grid-line').trim();
      const appBg = styles.getPropertyValue('--color-app-bg').trim();

      // Update sidebar background
      svg.selectAll('[data-element="sidebar-bg"]')
        .attr('fill', sidebarBg || themeColors.sidebar);

      // Update sidebar border
      svg.selectAll('[data-element="sidebar-border"]')
        .attr('stroke', borderColor || themeColors.border);

      // Update sidebar text colors
      svg.selectAll('[data-element="label-name"]')
        .attr('fill', textPrimary || themeColors.textPrimary);

      svg.selectAll('[data-element="label-kind-ns"]')
        .attr('fill', textMuted || themeColors.textMuted);

      // Update axis tick lines and text
      svg.selectAll('.tick line')
        .attr('stroke', gridLine || themeColors.grid);

      svg.selectAll('.tick text')
        .attr('fill', textPrimary || themeColors.textPrimary);

      // Update sticky axis text color
      if (stickyAxisRef.current) {
        d3.select(stickyAxisRef.current).selectAll('.tick text')
          .attr('fill', textPrimary || themeColors.textPrimary);
      }

      // Update row guide lines
      svg.selectAll('.row-guide')
        .attr('stroke', borderColor || themeColors.border);

      // Update event dot appearance
      svg.selectAll('.event-dot')
        .attr('stroke', appBg || themeColors.appBg);
    });

    return () => cancelAnimationFrame(frame);
  }, [theme]);

  // Secondary Effect: Style Updates (Selection & Highlights)
  // This runs whenever selection changes, without re-drawing the whole chart
  useEffect(() => {
    if (!svgRef.current) return;
    const svg = d3.select(svgRef.current);

    // Update Segments
    // Use theme-aware outline color: dark in dark theme, dark in light theme for visibility
    const outlineColor = theme === 'light' ? '#1e293b' : '#ffffff';
    svg.selectAll('.segment')
        .attr('stroke', (d: any) => {
          const isSel = selectedPoint && d.resourceId === selectedPoint.resourceId && d.index === selectedPoint.index;
          return isSel ? outlineColor : 'none';
        })
        .attr('stroke-width', (d: any) => {
          const isSel = selectedPoint && d.resourceId === selectedPoint.resourceId && d.index === selectedPoint.index;
          return isSel ? 3 : 0;
        });

    // Update Events
    svg.selectAll('.event-dot')
        .attr('fill', (d: any) => highlightedEventIds.includes(d.id) ? '#fbbf24' : '#f8fafc') // amber-400 vs slate-50
        .attr('r', (d: any) => highlightedEventIds.includes(d.id) ? 7 : 5)
        .attr('stroke', (d: any) => highlightedEventIds.includes(d.id) ? '#ffffff' : '#1e293b') // Dark outline for visibility in both themes
        .attr('stroke-width', (d: any) => highlightedEventIds.includes(d.id) ? 2 : 2)
        // Bring highlighted events to front
        .filter((d: any) => highlightedEventIds.includes(d.id))
        .raise();

  }, [selectedPoint, highlightedEventIds, themeColors, theme]);

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

  return (
    <div
      ref={containerRef}
      className="w-full h-full bg-[var(--color-surface-secondary)] rounded-lg shadow-inner border border-[var(--color-border-soft)] custom-scrollbar relative overflow-y-auto overflow-x-hidden transition-colors duration-300"
    >
      <svg
        ref={stickyAxisRef}
        width={width}
        height={MARGIN.top}
        className="sticky top-0 left-0 right-0 block bg-[var(--color-surface-secondary)] border-b border-[var(--color-border-soft)] z-10"
        style={{ pointerEvents: 'none' }}
      >
        <g
          id="sticky-axis"
          transform={`translate(${MARGIN.left},${MARGIN.top})`}
        />
      </svg>

      {/* Zoom Controls */}
      <div className="fixed flex gap-2 z-[10]" style={{ top: `120px`, right: '10px' }}>
        <button
          onClick={() => {
            if (!svgRef.current) return;
            const svg = d3.select(svgRef.current);
            const currentTransform = d3.zoomTransform(svg.node()!);
            const newScale = Math.max(1, currentTransform.k * 0.8); // Zoom out by 20%
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
          }}
          title="Zoom Out"
          className="p-2 rounded-md border border-[var(--color-border-soft)] bg-[var(--color-surface-muted)] text-[var(--color-text-primary)] hover:bg-[var(--color-surface-active)] transition-colors shadow-md"
        >
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0zM13 10H7" />
          </svg>
        </button>

        <button
          onClick={() => {
            if (!svgRef.current) return;
            const svg = d3.select(svgRef.current);
            const currentTransform = d3.zoomTransform(svg.node()!);
            const newScale = Math.min(30000, currentTransform.k * 1.25); // Zoom in by 25%
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
          }}
          title="Zoom In"
          className="p-2 rounded-md border border-[var(--color-border-soft)] bg-[var(--color-surface-muted)] text-[var(--color-text-primary)] hover:bg-[var(--color-surface-active)] transition-colors shadow-md"
        >
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0zM10 7v6m3-3H7" />
          </svg>
        </button>

        <button
          onClick={() => {
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
          }}
          title="Reset Zoom"
          className="p-2 rounded-md border border-[var(--color-border-soft)] bg-[var(--color-surface-muted)] text-[var(--color-text-primary)] hover:bg-[var(--color-surface-active)] transition-colors shadow-md"
        >
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M3 8V6a2 2 0 012-2h2M3 16v2a2 2 0 002 2h2m10-16h2a2 2 0 012 2v2m0 8v2a2 2 0 01-2 2h-2" />
          </svg>
        </button>
      </div>
      <svg
        ref={svgRef}
        width={width}
        height={Math.max(height, sortedResources.length * rowHeight + MARGIN.top + MARGIN.bottom)}
        className="block"
        style={{ overflow: 'visible' }}
      />
      {tooltip && (
        <div
          className="fixed z-50 bg-[var(--color-surface-elevated)] border border-[var(--color-border-soft)] rounded-lg shadow-xl p-3 pointer-events-none max-w-sm"
          style={{
            left: `${tooltip.x + 10}px`,
            top: `${tooltip.y + 10}px`,
            transform: 'translate(0, 0)'
          }}
        >
          <pre className="text-xs text-[var(--color-text-primary)] whitespace-pre-wrap font-sans">
            {tooltip.content}
          </pre>
        </div>
      )}
    </div>
  );
};