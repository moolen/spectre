import React, { useEffect, useRef, useMemo } from 'react';
import * as d3 from 'd3';
import { K8sResource, SelectedPoint } from '../types';
import { STATUS_COLORS } from '../constants';

interface TimelineProps {
  resources: K8sResource[];
  width: number;
  height: number;
  onSegmentClick: (resource: K8sResource, index: number) => void;
  selectedPoint: SelectedPoint | null;
  highlightedEventIds?: string[];
  sidebarWidth?: number;
}

const MARGIN = { top: 40, right: 30, bottom: 20, left: 240 };
const ROW_HEIGHT = 48;

export const Timeline: React.FC<TimelineProps> = ({ 
    resources, 
    width, 
    height, 
    onSegmentClick, 
    selectedPoint,
    highlightedEventIds = [],
    sidebarWidth = 0
}) => {
  const svgRef = useRef<SVGSVGElement>(null);
  
  // Track domain to know when to reset zoom
  const timeDomain = useMemo(() => {
    let min = new Date();
    let max = new Date(0);
    
    if (resources.length === 0) {
      const now = new Date();
      return [new Date(now.getTime() - 3600000), now] as [Date, Date];
    }

    resources.forEach(r => {
      r.statusSegments.forEach(s => {
        if (s.start < min) min = s.start;
        if (s.end > max) max = s.end;
      });
    });
    const duration = max.getTime() - min.getTime();
    return [new Date(min.getTime() - duration * 0.05), new Date(max.getTime() + duration * 0.05)] as [Date, Date];
  }, [resources]);

  const prevDomain = useRef(timeDomain);
  
  // Scales (memoized to prevent flicker during non-data updates)
  const innerWidth = width - MARGIN.left - MARGIN.right;
  const contentHeight = resources.length * ROW_HEIGHT;
  const minInnerHeight = height - MARGIN.top - MARGIN.bottom;
  const innerHeight = Math.max(minInnerHeight, contentHeight);

  const xScale = useMemo(() => d3.scaleTime()
      .domain(timeDomain)
      .range([0, innerWidth]), [timeDomain, innerWidth]);

  const yScale = useMemo(() => d3.scaleBand()
      .domain(resources.map(r => r.id))
      .range([0, contentHeight])
      .padding(0.4), [resources, contentHeight]);

  // Define Zoom Behavior
  const zoom = useMemo(() => d3.zoom<SVGSVGElement, unknown>()
      .scaleExtent([0.1, 1000]) // Allow zooming out further for context
      .translateExtent([[0, 0], [innerWidth * 10, height]]) 
      .extent([[0, 0], [innerWidth, height]]), [innerWidth, height]);

  // Main Draw Effect: Handles structure, data binding, and initial render
  useEffect(() => {
    if (!svgRef.current || resources.length === 0) return;

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

    const xAxisGroup = mainGroup.append('g')
      .attr('class', 'x-axis')
      .attr('transform', `translate(0, -10)`);

    const labelGroup = svg.append('g')
      .attr('transform', `translate(0, ${MARGIN.top})`);

    // Sidebar Background
    labelGroup.append('rect')
        .attr('width', MARGIN.left - 2)
        .attr('height', Math.max(height, contentHeight + MARGIN.top + MARGIN.bottom)) 
        .attr('y', -MARGIN.top)
        .attr('fill', '#0d1117') 
        .attr('stroke', 'none');

    labelGroup.append('line')
        .attr('x1', MARGIN.left - 1)
        .attr('x2', MARGIN.left - 1)
        .attr('y1', -MARGIN.top)
        .attr('y2', Math.max(height, contentHeight + MARGIN.top + MARGIN.bottom))
        .attr('stroke', '#374151')
        .attr('stroke-width', 1);

    // --- Draw Labels ---
    const labels = labelGroup.selectAll('.label')
      .data(resources)
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
        .attr('fill', 'transparent')
        .attr('class', 'group-hover:fill-gray-800 transition-colors');

    labels.append('text')
      .text(d => d.name)
      .attr('x', 0)
      .attr('y', yScale.bandwidth() / 2 - 6)
      .attr('fill', '#f1f5f9') 
      .style('font-size', '13px')
      .style('font-weight', '600')
      .style('dominant-baseline', 'middle');

    labels.append('text')
      .text(d => `${d.kind} • ${d.namespace}`)
      .attr('x', 0)
      .attr('y', yScale.bandwidth() / 2 + 10)
      .attr('fill', '#94a3b8')
      .style('font-size', '11px')
      .style('dominant-baseline', 'middle');
    
    labels.append('line')
        .attr('x1', 0)
        .attr('x2', MARGIN.left - 40)
        .attr('y1', yScale.bandwidth() + yScale.padding() * yScale.step() / 2)
        .attr('y2', yScale.bandwidth() + yScale.padding() * yScale.step() / 2)
        .attr('stroke', '#1e293b')
        .attr('stroke-width', 1);

    // --- Draw Content ---
    const rows = contentGroup.selectAll('.resource-row')
        .data(resources)
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
        .attr('stroke', '#334155')
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
        .attr('fill', s => STATUS_COLORS[s.status]);
        // Note: stroke/selection is handled in separate effect

    // Events
    rows.selectAll('.event-dot')
        .data(d => d.events)
        .enter()
        .append('circle')
        .attr('class', 'event-dot pointer-events-none')
        .attr('cy', yScale.bandwidth() / 2)
        .attr('r', 5)
        .attr('fill', '#f8fafc')
        .attr('stroke', '#0f172a')
        .attr('stroke-width', 2);

    // --- Zoom Update Function ---
    const updateChart = (transform: d3.ZoomTransform) => {
        const newXScale = transform.rescaleX(xScale);

        const axis = d3.axisTop(newXScale)
            .ticks(Math.max(width / 120, 2))
            .tickSizeOuter(0)
            .tickFormat(d3.timeFormat('%H:%M:%S') as any); 

        xAxisGroup.call(axis)
            .call(g => g.select('.domain').remove())
            .call(g => g.selectAll('.tick line').attr('stroke', '#475569').attr('stroke-width', 2).attr('y2', -5))
            .call(g => g.selectAll('.tick text')
                .attr('fill', '#cbd5e1')
                .attr('font-weight', '600')
                .attr('font-size', '12px')
                .attr('dy', '-8px')
            );

        const gridAxis = d3.axisBottom(newXScale)
            .ticks(Math.max(width / 120, 2))
            .tickSize(innerHeight)
            .tickFormat(() => '');

        gridGroup.call(gridAxis)
            .call(g => g.select('.domain').remove())
            .call(g => g.selectAll('.tick line')
                .attr('stroke', '#334155')
                .attr('stroke-opacity', 0.3)
                .attr('stroke-dasharray', '2,2')
            );

        contentGroup.selectAll<SVGRectElement, any>('.segment')
            .attr('x', d => newXScale(d.start))
            .attr('width', d => Math.max(4, newXScale(d.end) - newXScale(d.start)));

        contentGroup.selectAll<SVGCircleElement, any>('.event-dot')
            .attr('cx', d => newXScale(d.timestamp));
    };

    zoom.on('zoom', (event) => updateChart(event.transform));

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
            zoom.translateBy(svg, -event.deltaX, 0); 
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

            svg.transition().duration(750)
               .call(zoom.transform, d3.zoomIdentity.translate(tx, 0).scale(k));
        });

    brushGroup.call(brush);

    // Manual Hit Testing for Brush Overlay
    brushGroup.select('.overlay')
        .on('click', (event) => {
             const [x, y] = d3.pointer(event);
             const eachBand = yScale.step();
             const index = Math.floor(y / eachBand);
             const resource = resources[index];
             
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
             const resource = resources[index];
             let isOverSegment = false;
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
                 }
             }
             d3.select(this).style('cursor', isOverSegment ? 'pointer' : 'crosshair');
        });

    // Apply initial transform if exists
    svg.call(zoom.transform, currentTransform);

  }, [resources, width, height, timeDomain]); // Only re-run if data/layout changes

  // Secondary Effect: Style Updates (Selection & Highlights)
  // This runs whenever selection changes, without re-drawing the whole chart
  useEffect(() => {
    if (!svgRef.current) return;
    const svg = d3.select(svgRef.current);

    // Update Segments
    svg.selectAll('.segment')
        .attr('stroke', (d: any) => {
             const isSel = selectedPoint && d.resourceId === selectedPoint.resourceId && d.index === selectedPoint.index;
             return isSel ? '#ffffff' : '#0f172a';
        })
        .attr('stroke-width', (d: any) => {
             const isSel = selectedPoint && d.resourceId === selectedPoint.resourceId && d.index === selectedPoint.index;
             return isSel ? 3 : 1;
        });

    // Update Events
    svg.selectAll('.event-dot')
        .attr('fill', (d: any) => highlightedEventIds.includes(d.id) ? '#fbbf24' : '#f8fafc') // amber-400 vs slate-50
        .attr('r', (d: any) => highlightedEventIds.includes(d.id) ? 7 : 5)
        .attr('stroke', (d: any) => highlightedEventIds.includes(d.id) ? '#ffffff' : '#0f172a')
        .attr('stroke-width', (d: any) => highlightedEventIds.includes(d.id) ? 2 : 2)
        // Bring highlighted events to front
        .filter((d: any) => highlightedEventIds.includes(d.id))
        .raise();

  }, [selectedPoint, highlightedEventIds]);

  // Tertiary Effect: Auto-Center Navigation
  useEffect(() => {
    if (selectedPoint && svgRef.current) {
        const resource = resources.find(r => r.id === selectedPoint.resourceId);
        if (resource) {
            const segment = resource.statusSegments[selectedPoint.index];
            if (segment) {
                const startTime = segment.start;
                const endTime = segment.end;
                const midTime = new Date((startTime.getTime() + endTime.getTime()) / 2);

                const svg = d3.select(svgRef.current);
                const currentTransform = d3.zoomTransform(svg.node()!);
                
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
  }, [selectedPoint, sidebarWidth, resources, width, xScale, zoom]);

  return (
    <div className="w-full h-full bg-gray-950 rounded-lg shadow-inner border border-gray-800 custom-scrollbar relative overflow-y-auto overflow-x-hidden">
      <div className="sticky top-0 left-0 right-0 z-20 pointer-events-none flex justify-end p-2 bg-gradient-to-b from-gray-900/50 to-transparent">
        <div className="text-xs text-gray-500 bg-gray-900/80 backdrop-blur px-2 py-1 rounded border border-gray-800 shadow-sm mr-4">
          Drag to Zoom • Shift+Scroll to Pan Time • Double-click Reset • Arrow Keys to Navigate
        </div>
      </div>
      <svg 
        ref={svgRef} 
        width={width} 
        height={Math.max(height, resources.length * ROW_HEIGHT + MARGIN.top + MARGIN.bottom)}
        className="block"
      />
    </div>
  );
};