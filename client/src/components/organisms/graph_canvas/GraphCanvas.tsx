import { useRef, useEffect } from 'react';
import * as d3 from 'd3';
import {
  useGraphCanvas, GraphCanvasProps, computeLayout, nodeRadius, kindStyle, getGroupColor,
  STATUS_COLORS, EDGE_STYLES,
} from './GraphCanvas.vm';
import { KubeGraphEdge, KubeGraphNode } from '../../../types/kubernetes';
import { Canvas } from '../../atoms/canvas/Canvas';

const GROUP_PADDING = { x: 70, top: 50, bottom: 80 };
const DOUBLE_CLICK_MS = 250;

const truncate = (text: string, max: number) => (text.length > max ? text.slice(0, max - 1) + '…' : text);

const tooltipText = (d: KubeGraphNode) => [
  `${d.kind} ${d.name}`,
  d.summary,
  ...Object.entries(d.details || {}).filter(([, v]) => v).map(([k, v]) => `${k}: ${v}`),
  d.drillable ? 'Double-click to open' : '',
].filter(Boolean).join('\n');

export const GraphCanvas = (props: GraphCanvasProps) => {
  const { nodes, edges, groups, level, savePositions, fittedViewRef } = useGraphCanvas(props);
  const svgRef = useRef<SVGSVGElement>(null);
  const selectedRef = useRef(props.selectedId);
  const onSelectRef = useRef(props.onSelect);
  const onDrillRef = useRef(props.onDrill);

  selectedRef.current = props.selectedId;
  onSelectRef.current = props.onSelect;
  onDrillRef.current = props.onDrill;

  useEffect(() => {
    if (!svgRef.current?.parentElement) return;
    const svgEl = svgRef.current;
    const svg = d3.select(svgEl);
    svg.selectAll('*').remove();

    const width = svgEl.parentElement!.clientWidth;
    const height = svgEl.parentElement!.clientHeight;
    svg.attr('viewBox', `0 0 ${width} ${height}`);

    const isNewView = fittedViewRef.current !== props.viewKey;
    const radius = (d: KubeGraphNode) => nodeRadius(d, level);

    // Keep the current pan/zoom across refreshes of the same view
    const g = svg.append('g').attr('transform', isNewView ? '' : d3.zoomTransform(svgEl).toString());
    const zoom = d3.zoom<SVGSVGElement, unknown>()
      .scaleExtent([0.15, 4])
      .on('zoom', (event) => g.attr('transform', event.transform));
    svg.call(zoom).on('dblclick.zoom', null);
    if (isNewView) svg.call(zoom.transform, d3.zoomIdentity);

    svg.on('click', (event) => {
      if (event.target === svgEl) onSelectRef.current(null);
    });

    const defs = svg.append('defs');
    Object.entries(EDGE_STYLES).forEach(([kind, style]) => {
      defs.append('marker')
        .attr('id', `graph-arrow-${kind}`)
        .attr('viewBox', '0 -4 8 8')
        .attr('refX', 8)
        .attr('markerWidth', 7)
        .attr('markerHeight', 7)
        .attr('orient', 'auto')
        .append('path')
        .attr('d', 'M0,-4L8,0L0,4')
        .attr('fill', style.color);
    });

    // The stage fades/zooms in when entering a new level
    const stage = g.append('g');

    const nodeMap = new Map(nodes.map((n) => [n.id, n]));

    const groupRects = groups.map((grp, i) => {
      const groupG = stage.append('g');
      const rect = groupG.append('rect')
        .attr('rx', 12).attr('ry', 12)
        .attr('fill', getGroupColor(i)).attr('fill-opacity', 0.06)
        .attr('stroke', getGroupColor(i)).attr('stroke-opacity', 0.3)
        .attr('stroke-width', 1.5).attr('stroke-dasharray', '6 3');
      rect.append('title').text(`${grp.kind} ${grp.name}`);
      const label = groupG.append('text')
        .text(`${grp.kind === 'Node' ? 'node ' : ''}${grp.name}`)
        .attr('fill', getGroupColor(i)).attr('fill-opacity', 0.8)
        .attr('font-size', '11px').attr('font-weight', '600');
      const members = nodes.filter((n) => n.group === grp.id);
      return { rect, label, members };
    });

    const link = stage.append('g')
      .selectAll('line')
      .data(edges)
      .join('line')
      .attr('stroke', (d) => EDGE_STYLES[d.kind].color)
      .attr('stroke-opacity', 0.55)
      .attr('stroke-width', 1.5)
      .attr('stroke-dasharray', (d) => EDGE_STYLES[d.kind].dash)
      .attr('marker-end', (d) => `url(#graph-arrow-${d.kind})`);

    let clickTimer: ReturnType<typeof setTimeout> | undefined;
    const node = stage.append('g')
      .selectAll<SVGGElement, KubeGraphNode>('g')
      .data(nodes, (d) => d.id)
      .join('g')
      .attr('class', 'graph-node')
      .attr('cursor', 'pointer')
      .on('click', (event, d) => {
        event.stopPropagation();
        clearTimeout(clickTimer);
        if (event.detail > 1) return;
        const select = () => onSelectRef.current(d.id === selectedRef.current ? null : d);
        // Opening the side panel resizes the canvas, so wait until a double-click is ruled out
        if (d.drillable) clickTimer = setTimeout(select, DOUBLE_CLICK_MS);
        else select();
      })
      .on('dblclick', (event, d) => {
        event.stopPropagation();
        clearTimeout(clickTimer);
        if (d.drillable) onDrillRef.current(d);
      });

    node.append('title').text(tooltipText);

    node.filter((d) => d.drillable).append('circle')
      .attr('r', (d) => radius(d) + 5)
      .attr('fill', 'none')
      .attr('stroke', (d) => kindStyle(d.kind).color)
      .attr('stroke-opacity', 0.5)
      .attr('stroke-dasharray', '3 3');

    node.append('circle')
      .attr('class', 'graph-node__shape')
      .attr('r', radius)
      .attr('fill', '#111827')
      .attr('stroke', (d) => STATUS_COLORS[d.status])
      .attr('stroke-width', 2);

    node.append('text')
      .text((d) => kindStyle(d.kind).abbr)
      .attr('text-anchor', 'middle')
      .attr('dominant-baseline', 'central')
      .attr('fill', (d) => kindStyle(d.kind).color)
      .attr('font-size', (d) => (radius(d) > 30 ? '13px' : '10px'))
      .attr('font-weight', '700')
      .attr('pointer-events', 'none');

    node.append('text')
      .text((d) => truncate(d.name, 24))
      .attr('text-anchor', 'middle')
      .attr('y', (d) => radius(d) + 16)
      .attr('fill', '#e1e4ed')
      .attr('font-size', '11px')
      .attr('pointer-events', 'none');

    node.append('text')
      .text((d) => truncate(d.summary, 30))
      .attr('text-anchor', 'middle')
      .attr('y', (d) => radius(d) + 29)
      .attr('fill', '#8b8fa7')
      .attr('font-size', '9px')
      .attr('pointer-events', 'none');

    const { targets, groupCenters } = computeLayout({ level, nodes, edges, groups }, width, height);
    const anchorOf = (n: KubeGraphNode) => (n.group ? groupCenters.get(n.group) : targets.get(n.id));

    nodes.forEach((n) => {
      if (n.x != null && n.y != null) return;
      const anchor = anchorOf(n) || { x: width / 2, y: height / 2 };
      n.x = anchor.x + (Math.random() - 0.5) * 120;
      n.y = anchor.y + (Math.random() - 0.5) * 120;
    });

    const anchorForce = (alpha: number) => {
      for (const n of nodes) {
        const anchor = anchorOf(n);
        if (!anchor || n.x == null || n.y == null) continue;
        const strength = n.group ? 0.5 : 0.3;
        n.vx = (n.vx || 0) + (anchor.x - n.x) * strength * alpha;
        n.vy = (n.vy || 0) + (anchor.y - n.y) * strength * 0.6 * alpha;
      }
    };

    const free = (n: KubeGraphNode) => !anchorOf(n);
    const simulation = d3.forceSimulation<KubeGraphNode>(nodes)
      .force('link', d3.forceLink<KubeGraphNode, KubeGraphEdge>(edges).id((d) => d.id).distance(140).strength(0.1))
      .force('charge', d3.forceManyBody().strength(level === 'cluster' ? -500 : -250))
      .force('collision', d3.forceCollide<KubeGraphNode>().radius((d) => radius(d) + 34))
      .force('anchor', anchorForce)
      .force('x', d3.forceX<KubeGraphNode>(width / 2).strength((d) => (free(d) ? 0.06 : 0)))
      .force('y', d3.forceY<KubeGraphNode>(height / 2).strength((d) => (free(d) ? 0.06 : 0)));

    if (!isNewView && nodes.every((n) => n.vx === undefined)) simulation.alpha(0.1);

    node.call(d3.drag<SVGGElement, KubeGraphNode>()
      .on('start', (event, d) => {
        if (!event.active) simulation.alphaTarget(0.3).restart();
        d.fx = d.x;
        d.fy = d.y;
      })
      .on('drag', (event, d) => {
        d.fx = event.x;
        d.fy = event.y;
      })
      .on('end', (event, d) => {
        if (!event.active) simulation.alphaTarget(0);
        d.fx = null;
        d.fy = null;
      }));

    simulation.on('tick', () => {
      groupRects.forEach(({ rect, label, members }) => {
        const placed = members.filter((m) => m.x != null && m.y != null);
        if (placed.length === 0) return;
        const minX = d3.min(placed, (m) => m.x!)! - GROUP_PADDING.x;
        const minY = d3.min(placed, (m) => m.y!)! - GROUP_PADDING.top;
        const maxX = d3.max(placed, (m) => m.x!)! + GROUP_PADDING.x;
        const maxY = d3.max(placed, (m) => m.y!)! + GROUP_PADDING.bottom;
        rect.attr('x', minX).attr('y', minY).attr('width', maxX - minX).attr('height', maxY - minY);
        label.attr('x', minX + 10).attr('y', minY + 18);
      });

      // Stop edges at the target's border so the arrowhead stays visible
      link.each(function (d) {
        const s = d.source as KubeGraphNode;
        const t = d.target as KubeGraphNode;
        const dx = t.x! - s.x!;
        const dy = t.y! - s.y!;
        const dist = Math.hypot(dx, dy) || 1;
        const sourceGap = radius(s) / dist;
        const targetGap = (radius(t) + 6) / dist;
        d3.select(this)
          .attr('x1', s.x! + dx * sourceGap)
          .attr('y1', s.y! + dy * sourceGap)
          .attr('x2', t.x! - dx * targetGap)
          .attr('y2', t.y! - dy * targetGap);
      });

      node.attr('transform', (d) => `translate(${d.x},${d.y})`);
    });

    const fitToView = () => {
      const minX = d3.min(nodes, (n) => n.x! - radius(n))! - 60;
      const maxX = d3.max(nodes, (n) => n.x! + radius(n))! + 60;
      const minY = d3.min(nodes, (n) => n.y! - radius(n))! - 50;
      const maxY = d3.max(nodes, (n) => n.y! + radius(n))! + 70;
      const scale = Math.min(width / (maxX - minX), height / (maxY - minY), 1.2);
      const transform = d3.zoomIdentity
        .translate(width / 2, height / 2)
        .scale(scale)
        .translate(-(minX + maxX) / 2, -(minY + maxY) / 2);
      svg.transition().duration(450).call(zoom.transform, transform);
    };

    simulation.on('end', () => {
      savePositions(nodes);
      if (fittedViewRef.current !== props.viewKey && nodes.length > 0) {
        fittedViewRef.current = props.viewKey;
        fitToView();
      }
    });

    if (isNewView) {
      const cx = width / 2;
      const cy = height / 2;
      stage
        .attr('opacity', 0)
        .attr('transform', `translate(${cx},${cy}) scale(0.85) translate(${-cx},${-cy})`)
        .transition().duration(400).ease(d3.easeCubicOut)
        .attr('opacity', 1)
        .attr('transform', 'translate(0,0) scale(1)');
      // Settle the layout quickly on entry so fit-to-view happens soon after
      simulation.alphaDecay(0.05);
    }

    const kinds = [...new Set(nodes.map((n) => n.kind))];
    const legend = svg.append('g').attr('transform', `translate(16, ${height - 20 - kinds.length * 18})`);
    kinds.forEach((kind, i) => {
      const row = legend.append('g').attr('transform', `translate(0, ${i * 18})`);
      row.append('text').text(kindStyle(kind).abbr)
        .attr('fill', kindStyle(kind).color).attr('font-size', '10px').attr('font-weight', '700');
      row.append('text').text(kind).attr('x', 34).attr('fill', '#8b8fa7').attr('font-size', '10px');
    });

    return () => {
      clearTimeout(clickTimer);
      savePositions(nodes);
      simulation.stop();
    };
  }, [nodes, edges, groups, level, props.viewKey, savePositions, fittedViewRef]);

  useEffect(() => {
    if (!svgRef.current) return;
    d3.select(svgRef.current)
      .selectAll<SVGGElement, KubeGraphNode>('.graph-node')
      .classed('graph-node--selected', (d) => d.id === props.selectedId);
  }, [props.selectedId, nodes]);

  return <Canvas svgRef={svgRef} />;
};
