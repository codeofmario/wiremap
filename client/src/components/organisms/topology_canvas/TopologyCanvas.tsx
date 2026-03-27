import { useRef, useEffect, useCallback } from 'react';
import * as d3 from 'd3';
import { useTopologyCanvas, TopologyCanvasProps, getNetworkColor } from './TopologyCanvas.vm';
import { TopologyNode, TopologyLink, TopologyGroup } from '../../../types/docker';
import { Canvas } from '../../atoms/canvas/Canvas';

export const TopologyCanvas = (props: TopologyCanvasProps) => {
  const { nodes, links, groups, savePositions } = useTopologyCanvas(props);
  const svgRef = useRef<SVGSVGElement>(null);
  const simulationRef = useRef<d3.Simulation<TopologyNode, TopologyLink> | null>(null);
  const selectedRef = useRef(props.selectedContainerId);
  const onSelectRef = useRef(props.onSelectContainer);

  selectedRef.current = props.selectedContainerId;
  onSelectRef.current = props.onSelectContainer;

  const computeGroupBounds = useCallback((group: TopologyGroup, nodeMap: Map<string, TopologyNode>) => {
    // Padding accounts for node visuals: circle r=22, status dot at y=-15, label at y=36+font
    const paddingX = 70;
    const paddingTop = 50;
    const paddingBottom = 80; // extra room for name label below node center
    let minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
    let found = 0;

    for (const id of group.nodes) {
      const n = nodeMap.get(id);
      if (n?.x != null && n?.y != null) {
        minX = Math.min(minX, n.x);
        minY = Math.min(minY, n.y);
        maxX = Math.max(maxX, n.x);
        maxY = Math.max(maxY, n.y);
        found++;
      }
    }

    if (found === 0) return null;
    return {
      x: minX - paddingX,
      y: minY - paddingTop,
      width: maxX - minX + paddingX * 2,
      height: maxY - minY + paddingTop + paddingBottom,
    };
  }, []);

  useEffect(() => {
    if (!svgRef.current) return;
    const svgEl = svgRef.current;
    const svg = d3.select(svgEl);
    svg.selectAll('*').remove();

    const parent = svgEl.parentElement;
    if (!parent) return;

    const width = parent.clientWidth;
    const height = parent.clientHeight;
    svg.attr('viewBox', `0 0 ${width} ${height}`);

    const g = svg.append('g');

    const zoom = d3.zoom<SVGSVGElement, unknown>()
      .scaleExtent([0.2, 4])
      .on('zoom', (event) => g.attr('transform', event.transform));
    svg.call(zoom as any);

    svg.on('click', (event) => {
      if (event.target === svgEl) {
        onSelectRef.current(null);
      }
    });

    const networkMap = new Map<string, number>();
    groups.forEach((grp, i) => networkMap.set(grp.name, i));

    const nodeMap = new Map<string, TopologyNode>();
    nodes.forEach((n) => nodeMap.set(n.id, n));

    // Network group rectangles
    const groupLayer = g.append('g').attr('class', 'topology-groups');
    const groupRects = groups.map((grp, i) => {
      const groupG = groupLayer.append('g');
      const rect = groupG.append('rect')
        .attr('rx', 12)
        .attr('ry', 12)
        .attr('fill', getNetworkColor(i))
        .attr('fill-opacity', 0.06)
        .attr('stroke', getNetworkColor(i))
        .attr('stroke-opacity', 0.25)
        .attr('stroke-width', 1.5)
        .attr('stroke-dasharray', '6 3');

      const label = groupG.append('text')
        .text(grp.name)
        .attr('fill', getNetworkColor(i))
        .attr('fill-opacity', 0.7)
        .attr('font-size', '11px')
        .attr('font-weight', '600')
        .attr('letter-spacing', '0.5px');

      // Tooltip on hover
      rect.on('mouseenter', (event: MouseEvent) => {
        const tooltip = document.getElementById('topology-tooltip');
        if (!tooltip) return;
        tooltip.textContent = '';
        const title = document.createElement('strong');
        title.textContent = grp.name;
        tooltip.appendChild(title);
        const lines = [
          `Driver: ${grp.driver}`,
          `Scope: ${grp.scope}`,
          ...(grp.subnet ? [`Subnet: ${grp.subnet}`] : []),
          ...(grp.gateway ? [`Gateway: ${grp.gateway}`] : []),
          `Containers: ${grp.containerCount}`,
        ];
        for (const text of lines) {
          const div = document.createElement('div');
          div.textContent = text;
          tooltip.appendChild(div);
        }
        tooltip.style.display = 'block';
        tooltip.style.left = `${event.clientX + 12}px`;
        tooltip.style.top = `${event.clientY + 12}px`;
      });
      rect.on('mousemove', (event: MouseEvent) => {
        const tooltip = document.getElementById('topology-tooltip');
        if (!tooltip) return;
        tooltip.style.left = `${event.clientX + 12}px`;
        tooltip.style.top = `${event.clientY + 12}px`;
      });
      rect.on('mouseleave', () => {
        const tooltip = document.getElementById('topology-tooltip');
        if (tooltip) tooltip.style.display = 'none';
      });

      return { group: grp, rect, label, index: i };
    });

    // Links
    const link = g.append('g')
      .attr('class', 'topology-links')
      .selectAll('line')
      .data(links)
      .join('line')
      .attr('stroke', (d) => getNetworkColor(networkMap.get(d.network) || 0))
      .attr('stroke-opacity', 0.3)
      .attr('stroke-width', 1.5);

    // Nodes
    const node = g.append('g')
      .attr('class', 'topology-nodes')
      .selectAll('g')
      .data(nodes)
      .join('g')
      .attr('class', 'topology-node')
      .attr('cursor', 'pointer')
      .on('click', (_event, d) => {
        onSelectRef.current(d.id === selectedRef.current ? null : d.id);
      });

    node.call(d3.drag<any, TopologyNode>()
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
      })
    );

    // Node visuals
    node.append('circle')
      .attr('r', 22)
      .attr('fill', (d) => d.state === 'running' ? '#111827' : '#1f1215')
      .attr('stroke', (d) => d.state === 'running' ? '#22c55e' : '#ef4444')
      .attr('stroke-width', 2);

    node.append('circle')
      .attr('r', 5)
      .attr('cx', 15)
      .attr('cy', -15)
      .attr('fill', (d) => d.state === 'running' ? '#22c55e' : '#ef4444');

    node.append('text')
      .text((d) => d.name.slice(0, 2).toUpperCase())
      .attr('text-anchor', 'middle')
      .attr('dominant-baseline', 'central')
      .attr('fill', '#e1e4ed')
      .attr('font-size', '12px')
      .attr('font-weight', '700')
      .attr('pointer-events', 'none');

    node.append('text')
      .text((d) => d.name.length > 18 ? d.name.slice(0, 16) + '...' : d.name)
      .attr('text-anchor', 'middle')
      .attr('y', 36)
      .attr('fill', '#8b8fa7')
      .attr('font-size', '10px')
      .attr('pointer-events', 'none');

    // Compute group centers spread in a grid with generous spacing
    const groupCenters = new Map<string, { x: number; y: number }>();
    const totalGroups = groups.length || 1;
    const cols = Math.ceil(Math.sqrt(totalGroups));
    const rows = Math.ceil(totalGroups / cols);
    const cellW = Math.max(width / (cols + 1), 250);
    const cellH = Math.max(height / (rows + 1), 250);

    groups.forEach((grp, i) => {
      const col = i % cols;
      const row = Math.floor(i / cols);
      groupCenters.set(grp.id, {
        x: cellW * (col + 1),
        y: cellH * (row + 1),
      });
    });

    // Map node → group
    const nodeGroupMap = new Map<string, string>();
    groups.forEach((grp) => {
      grp.nodes.forEach((nid) => nodeGroupMap.set(nid, grp.id));
    });

    // Set initial positions near group centers for unpositioned nodes
    nodes.forEach((n) => {
      if (n.x != null && n.y != null) return;
      const gid = nodeGroupMap.get(n.id);
      const center = gid ? groupCenters.get(gid) : null;
      const cx = center?.x ?? width / 2;
      const cy = center?.y ?? height / 2;
      n.x = cx + (Math.random() - 0.5) * 160;
      n.y = cy + (Math.random() - 0.5) * 160;
    });

    // Custom clustering force: pull nodes toward their group center
    // Stronger pull keeps exited/isolated containers close to their network group
    const clusterForce = (alpha: number) => {
      const strength = 0.5;
      for (const n of nodes) {
        const gid = nodeGroupMap.get(n.id);
        if (!gid) continue;
        const center = groupCenters.get(gid);
        if (!center || n.x == null || n.y == null) continue;
        n.vx = (n.vx || 0) + (center.x - n.x) * strength * alpha;
        n.vy = (n.vy || 0) + (center.y - n.y) * strength * alpha;
      }
    };

    // Ungrouped nodes go to the center
    const ungroupedForce = (alpha: number) => {
      for (const n of nodes) {
        if (nodeGroupMap.has(n.id)) continue;
        if (n.x == null || n.y == null) continue;
        n.vx = (n.vx || 0) + (width / 2 - n.x) * 0.1 * alpha;
        n.vy = (n.vy || 0) + (height / 2 - n.y) * 0.1 * alpha;
      }
    };

    // Simulation
    const hasPositions = nodes.some((n) => n.x != null && n.y != null);
    const simulation = d3.forceSimulation<TopologyNode>(nodes)
      .force('link', d3.forceLink<TopologyNode, TopologyLink>(links)
        .id((d) => d.id)
        .distance(140))
      .force('charge', d3.forceManyBody().strength(-400))
      .force('collision', d3.forceCollide().radius(65))
      .force('cluster', clusterForce)
      .force('ungrouped', ungroupedForce);

    if (hasPositions) {
      simulation.alpha(0.1);
    }

    simulationRef.current = simulation;

    simulation.on('tick', () => {
      // Update group rects
      groupRects.forEach(({ group: grp, rect, label }) => {
        const bounds = computeGroupBounds(grp, nodeMap);
        if (bounds) {
          rect.attr('x', bounds.x).attr('y', bounds.y)
            .attr('width', bounds.width).attr('height', bounds.height);
          label.attr('x', bounds.x + 8).attr('y', bounds.y + 16);
        }
      });

      link
        .attr('x1', (d: any) => d.source.x)
        .attr('y1', (d: any) => d.source.y)
        .attr('x2', (d: any) => d.target.x)
        .attr('y2', (d: any) => d.target.y);

      node.attr('transform', (d) => `translate(${d.x},${d.y})`);
    });

    simulation.on('end', () => {
      savePositions(nodes);
    });

    // Legend
    const legend = svg.append('g').attr('transform', 'translate(16, 16)');
    groups.forEach((grp, i) => {
      const row = legend.append('g').attr('transform', `translate(0, ${i * 22})`);
      row.append('rect')
        .attr('width', 14).attr('height', 14).attr('rx', 3)
        .attr('fill', getNetworkColor(i)).attr('fill-opacity', 0.2)
        .attr('stroke', getNetworkColor(i)).attr('stroke-width', 1);
      row.append('text')
        .text(grp.name).attr('x', 20).attr('y', 11)
        .attr('fill', '#8b8fa7').attr('font-size', '11px');
    });

    return () => {
      savePositions(nodes);
      simulation.stop();
      simulationRef.current = null;
    };
  }, [nodes, links, groups, savePositions, computeGroupBounds]);

  return (
    <Canvas svgRef={svgRef}>
      <div id="topology-tooltip" className="canvas__tooltip" />
    </Canvas>
  );
};
