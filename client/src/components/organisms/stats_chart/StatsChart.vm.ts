import { ContainerStats } from '../../../types/docker';

export interface StatsChartProps {
  current: ContainerStats | null;
  history: ContainerStats[];
  /** Kubernetes metrics-server reports no network counters */
  showNetwork?: boolean;
  className?: string;
}

export const useStatsChart = ({ current, history, showNetwork = true }: StatsChartProps) => {
  return { current, history, showNetwork };
};

export const formatBytes = (bytes: number): string => {
  if (bytes === 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`;
};
