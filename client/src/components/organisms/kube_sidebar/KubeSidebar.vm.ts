import { useMemo } from 'react';
import { useKubeCounts } from '../../../hooks/useKubeCounts';
import { pluralKind } from '../../../services/format';
import { KubeKind, KubeRoot } from '../../../types/kubernetes';

export interface KubeSidebarProps {
  cluster: string;
  kinds: KubeKind[];
  namespace: string;
  root: KubeRoot;
  onSelect: (root: KubeRoot) => void;
}

interface SectionEntry {
  label: string;
  root: KubeRoot;
  /** "group/Kind" whose count is shown */
  countKey?: string;
}

const kindKey = (group: string, kind: string) => `${group}/${kind}`;

const kindRoot = (key: string): KubeRoot => {
  const [group, kind] = key.split('/');
  return { type: 'kind', group, kind };
};

const kindEntry = (key: string): SectionEntry => ({ label: pluralKind(key.split('/')[1]), root: kindRoot(key), countKey: key });

// Kinds pinned to the sidebar, grouped by what they're for
const SECTIONS: { label: string; entries: SectionEntry[] }[] = [
  {
    label: 'Cluster',
    entries: [
      { label: 'Nodes', root: { type: 'lens', lens: 'nodes' }, countKey: '/Node' },
      kindEntry('/Namespace'),
      { label: 'Storage map', root: { type: 'lens', lens: 'storage' } },
      { label: 'Access map', root: { type: 'lens', lens: 'access' } },
    ],
  },
  {
    label: 'Workloads',
    entries: ['/Pod', 'apps/Deployment', 'apps/StatefulSet', 'apps/DaemonSet', 'apps/ReplicaSet', 'batch/Job', 'batch/CronJob'].map(kindEntry),
  },
  {
    label: 'Network',
    entries: ['/Service', 'networking.k8s.io/Ingress', 'networking.k8s.io/NetworkPolicy'].map(kindEntry),
  },
  {
    label: 'Config & storage',
    entries: ['/ConfigMap', '/Secret', '/PersistentVolumeClaim', '/PersistentVolume', 'storage.k8s.io/StorageClass'].map(kindEntry),
  },
  {
    label: 'Scaling & policy',
    entries: ['autoscaling/HorizontalPodAutoscaler', 'policy/PodDisruptionBudget', '/ResourceQuota', '/LimitRange'].map(kindEntry),
  },
  {
    label: 'Access control',
    entries: [
      '/ServiceAccount', 'rbac.authorization.k8s.io/Role', 'rbac.authorization.k8s.io/RoleBinding',
      'rbac.authorization.k8s.io/ClusterRole', 'rbac.authorization.k8s.io/ClusterRoleBinding',
    ].map(kindEntry),
  },
];

const PINNED = new Set(SECTIONS.flatMap((s) => s.entries.map((e) => e.countKey).filter(Boolean)));

const sameRoot = (a: KubeRoot, b: KubeRoot) =>
  a.type === b.type
  && (a.type !== 'lens' || b.type !== 'lens' || a.lens === b.lens)
  && (a.type !== 'kind' || b.type !== 'kind' || (a.group === b.group && a.kind === b.kind));

export const useKubeSidebar = ({ cluster, kinds, namespace, root, onSelect }: KubeSidebarProps) => {
  const served = useMemo(() => new Set(kinds.map((k) => kindKey(k.group, k.kind))), [kinds]);
  const countKeys = useMemo(() => [...PINNED].filter((key) => served.has(key!)) as string[], [served]);
  const counts = useKubeCounts(cluster, namespace, countKeys);

  const toItem = (entry: SectionEntry) => ({
    id: entry.label,
    label: entry.label,
    count: entry.countKey ? counts[entry.countKey] : undefined,
    active: sameRoot(entry.root, root),
    select: () => onSelect(entry.root),
  });

  const sections = SECTIONS
    .map((section) => ({
      label: section.label,
      items: section.entries.filter((e) => !e.countKey || served.has(e.countKey)).map(toItem),
    }))
    .filter((section) => section.items.length > 0);

  const others = kinds
    .filter((k) => !PINNED.has(kindKey(k.group, k.kind)))
    .map((k) => ({
      ...toItem({ label: pluralKind(k.kind), root: { type: 'kind', group: k.group, kind: k.kind } }),
      id: kindKey(k.group, k.kind),
      group: k.group,
    }));

  return {
    overview: toItem({ label: namespace ? `Namespace ${namespace}` : 'Cluster overview', root: { type: 'overview' } }),
    sections,
    others,
  };
};
