import { KubeGraphNode, KubeNodeStatus } from '../../../types/kubernetes';

export interface KubeObjectListProps {
  nodes: KubeGraphNode[];
  selectedId: string | null;
  onSelect: (node: KubeGraphNode | null) => void;
  hasMore: boolean;
  loadMore: () => void;
  loadingMore: boolean;
}

// StatusDot states for each object status
const STATUS_DOTS: Record<KubeNodeStatus, string> = { healthy: 'running', error: 'exited', warning: 'other', idle: 'idle' };

export const useKubeObjectList = ({ nodes, selectedId, onSelect, hasMore, loadMore, loadingMore }: KubeObjectListProps) => ({
  rows: nodes.map((node) => ({
    node,
    dot: STATUS_DOTS[node.status],
    active: node.id === selectedId,
    subtitle: [node.namespace, node.summary, node.details?.Age].filter(Boolean).join(' · '),
    toggle: () => onSelect(node.id === selectedId ? null : node),
  })),
  hasMore,
  loadMore,
  loadingMore,
});
