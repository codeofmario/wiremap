import { useRef, useMemo, useCallback } from 'react';
import { ContainerInfo, NetworkInfo, TopologyNode, TopologyLink, TopologyGroup } from '../../../types/docker';

export interface TopologyCanvasProps {
  containers: ContainerInfo[];
  networks: NetworkInfo[];
  selectedContainerId: string | null;
  onSelectContainer: (id: string | null) => void;
  className?: string;
}

export const useTopologyCanvas = ({ containers, networks }: TopologyCanvasProps) => {
  const positionsRef = useRef<Map<string, { x: number; y: number }>>(new Map());

  const { nodes, links, groups } = useMemo(() => {
    const saved = positionsRef.current;

    const nodes: TopologyNode[] = containers.map((c) => {
      const pos = saved.get(c.id);
      return {
        id: c.id,
        name: (c.names[0] || '').replace(/^\//, ''),
        image: c.image,
        state: c.state,
        networks: Object.keys(c.networks || {}),
        ...(pos ? { x: pos.x, y: pos.y } : {}),
      };
    });

    const nodeIds = new Set(nodes.map((n) => n.id));
    const links: TopologyLink[] = [];
    const groups: TopologyGroup[] = [];

    for (const net of networks) {
      if (!net.containers || Object.keys(net.containers).length === 0) continue;

      const containerIds = Object.keys(net.containers).filter((id) => nodeIds.has(id));
      groups.push({
        id: net.id,
        name: net.name,
        driver: net.driver || '',
        scope: net.scope || '',
        subnet: net.subnet || '',
        gateway: net.gateway || '',
        containerCount: containerIds.length,
        nodes: containerIds,
      });

      for (let i = 0; i < containerIds.length; i++) {
        for (let j = i + 1; j < containerIds.length; j++) {
          links.push({
            source: containerIds[i],
            target: containerIds[j],
            network: net.name,
          });
        }
      }
    }

    return { nodes, links, groups };
  }, [containers, networks]);

  const savePositions = useCallback((currentNodes: TopologyNode[]) => {
    const map = new Map<string, { x: number; y: number }>();
    for (const n of currentNodes) {
      if (n.x != null && n.y != null) {
        map.set(n.id, { x: n.x, y: n.y });
      }
    }
    positionsRef.current = map;
  }, []);

  return { nodes, links, groups, savePositions };
};

const NETWORK_COLORS = [
  '#6366f1', '#22c55e', '#f59e0b', '#ef4444', '#3b82f6',
  '#ec4899', '#14b8a6', '#f97316', '#8b5cf6', '#06b6d4',
];

export const getNetworkColor = (index: number): string => {
  return NETWORK_COLORS[index % NETWORK_COLORS.length];
};
