import { useState, useMemo } from 'react';
import { ContainerInfo, NetworkInfo } from '../../../types/docker';

export interface ContainerListProps {
  containers: ContainerInfo[];
  networks: NetworkInfo[];
  selectedContainerId: string | null;
  onSelectContainer: (id: string | null) => void;
}

export interface NetworkGroup {
  id: string;
  name: string;
  driver: string;
  subnet: string;
  containerCount: number;
  color: string;
  containers: ContainerInfo[];
}

const NETWORK_COLORS = [
  '#6366f1', '#22c55e', '#f59e0b', '#ef4444', '#3b82f6',
  '#ec4899', '#14b8a6', '#f97316', '#8b5cf6', '#06b6d4',
];

export type StateFilter = 'all' | 'running' | 'exited';

export const STATE_FILTER_OPTIONS = [
  { value: 'all', label: 'All' },
  { value: 'running', label: 'Running' },
  { value: 'exited', label: 'Exited' },
];

export const useContainerList = ({ containers, networks }: ContainerListProps) => {
  const [search, setSearch] = useState('');
  const [stateFilter, setStateFilter] = useState<StateFilter>('all');

  const filtered = useMemo(() => {
    const q = search.toLowerCase().trim();
    return containers.filter((c) => {
      if (stateFilter !== 'all' && c.state !== stateFilter) return false;
      if (q) {
        const name = (c.names[0] || '').toLowerCase();
        const image = c.image.toLowerCase();
        if (!name.includes(q) && !image.includes(q)) return false;
      }
      return true;
    });
  }, [containers, search, stateFilter]);

  const { groups, ungrouped } = useMemo(() => {
    const containerMap = new Map<string, ContainerInfo>();
    filtered.forEach((c) => containerMap.set(c.id, c));

    const assignedIds = new Set<string>();
    const groups: NetworkGroup[] = [];

    networks.forEach((net, i) => {
      if (!net.containers || Object.keys(net.containers).length === 0) return;

      const netContainers: ContainerInfo[] = [];
      for (const cid of Object.keys(net.containers)) {
        const c = containerMap.get(cid);
        if (c) {
          netContainers.push(c);
          assignedIds.add(cid);
        }
      }

      if (netContainers.length === 0) return;

      groups.push({
        id: net.id,
        name: net.name,
        driver: net.driver,
        subnet: net.subnet || '',
        containerCount: netContainers.length,
        color: NETWORK_COLORS[i % NETWORK_COLORS.length],
        containers: netContainers,
      });
    });

    const ungrouped = filtered.filter((c) => !assignedIds.has(c.id));

    return { groups, ungrouped };
  }, [filtered, networks]);

  return { groups, ungrouped, search, setSearch, stateFilter, setStateFilter };
};

export const containerDisplayName = (c: ContainerInfo): string =>
  (c.names[0] || '').replace(/^\//, '');
