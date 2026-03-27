import { useRef, useEffect } from 'react';
import * as d3 from 'd3';
import { useStatsChart, StatsChartProps, formatBytes } from './StatsChart.vm';
import { StatCard } from '../../molecules/stat_card/StatCard';
import { Stack } from '../../atoms/stack/Stack';
import { Panel } from '../../atoms/panel/Panel';
import { Text } from '../../atoms/text/Text';
import { ScrollArea } from '../../atoms/scroll_area/ScrollArea';
import { Canvas } from '../../atoms/canvas/Canvas';

export const StatsChart = (props: StatsChartProps) => {
  const { current, history } = useStatsChart(props);
  const cpuRef = useRef<SVGSVGElement>(null);
  const memRef = useRef<SVGSVGElement>(null);

  useEffect(() => {
    if (history.length < 2) return;
    renderChart(cpuRef.current, history, (d) => d.cpuPercent, '#6366f1', '%');
    renderChart(memRef.current, history, (d) => d.memoryPercent, '#22c55e', '%');
  }, [history]);

  if (!current) {
    return (
      <Stack fullHeight align="center" justify="center">
        <Text variant="secondary" size="sm">Waiting for stats...</Text>
      </Stack>
    );
  }

  return (
    <ScrollArea flex>
      <Stack gap="lg" padding="md">
        <Stack direction="row" gap="sm">
          <StatCard label="CPU" value={`${current.cpuPercent}`} unit="%" />
          <StatCard label="Memory" value={formatBytes(current.memoryUsage)} unit={`/ ${formatBytes(current.memoryLimit)}`} />
          <StatCard label="Net RX" value={formatBytes(current.networkRx)} />
          <StatCard label="Net TX" value={formatBytes(current.networkTx)} />
        </Stack>
        <Stack gap="md">
          <Panel variant="bordered" padding="sm">
            <Stack gap="xs">
              <Text variant="label" size="xs">CPU Usage</Text>
              <Canvas svgRef={cpuRef} />
            </Stack>
          </Panel>
          <Panel variant="bordered" padding="sm">
            <Stack gap="xs">
              <Text variant="label" size="xs">Memory Usage</Text>
              <Canvas svgRef={memRef} />
            </Stack>
          </Panel>
        </Stack>
      </Stack>
    </ScrollArea>
  );
};

const renderChart = (
  svgEl: SVGSVGElement | null,
  data: any[],
  accessor: (d: any) => number,
  color: string,
  _unit: string,
) => {
  if (!svgEl) return;

  const svg = d3.select(svgEl);
  svg.selectAll('*').remove();

  const parent = svgEl.parentElement;
  if (!parent) return;

  const width = parent.clientWidth;
  const height = 100;
  svg.attr('viewBox', `0 0 ${width} ${height}`);

  const x = d3.scaleLinear().domain([0, data.length - 1]).range([0, width]);
  const y = d3.scaleLinear().domain([0, Math.max(d3.max(data, accessor) || 1, 1) * 1.1]).range([height, 0]);

  const area = d3.area<any>()
    .x((_, i) => x(i))
    .y0(height)
    .y1((d) => y(accessor(d)))
    .curve(d3.curveMonotoneX);

  const line = d3.line<any>()
    .x((_, i) => x(i))
    .y((d) => y(accessor(d)))
    .curve(d3.curveMonotoneX);

  svg.append('path')
    .datum(data)
    .attr('fill', color)
    .attr('fill-opacity', 0.1)
    .attr('d', area);

  svg.append('path')
    .datum(data)
    .attr('fill', 'none')
    .attr('stroke', color)
    .attr('stroke-width', 1.5)
    .attr('d', line);
};
