import { useRef, useMemo, useCallback } from 'react';
import { KubeEdgeKind, KubeGraph, KubeGraphEdge, KubeGraphNode, KubeLevel, KubeNodeStatus } from '../../../types/kubernetes';

export interface GraphCanvasProps {
  graph: KubeGraph;
  /** Changes when the user moves to another level; triggers the enter animation and fit-to-view. */
  viewKey: string;
  selectedId: string | null;
  onSelect: (node: KubeGraphNode | null) => void;
  onDrill: (node: KubeGraphNode) => void;
}

export const STATUS_COLORS: Record<KubeNodeStatus, string> = {
  healthy: '#22c55e',
  warning: '#f59e0b',
  error: '#ef4444',
  idle: '#8b8fa7',
};

export const KIND_STYLES: Record<string, { abbr: string; color: string }> = {
  Namespace: { abbr: 'NS', color: '#6366f1' },
  Node: { abbr: 'NO', color: '#06b6d4' },
  Ingress: { abbr: 'ING', color: '#ec4899' },
  Gateway: { abbr: 'GW', color: '#db2777' },
  HTTPRoute: { abbr: 'HR', color: '#f472b6' },
  GRPCRoute: { abbr: 'GR', color: '#f472b6' },
  TLSRoute: { abbr: 'TLS', color: '#f472b6' },
  TCPRoute: { abbr: 'TCP', color: '#f472b6' },
  UDPRoute: { abbr: 'UDP', color: '#f472b6' },
  Service: { abbr: 'SVC', color: '#3b82f6' },
  Deployment: { abbr: 'DEP', color: '#8b5cf6' },
  StatefulSet: { abbr: 'STS', color: '#a855f7' },
  DaemonSet: { abbr: 'DS', color: '#14b8a6' },
  ReplicaSet: { abbr: 'RS', color: '#818cf8' },
  ReplicationController: { abbr: 'RC', color: '#818cf8' },
  Job: { abbr: 'JOB', color: '#f97316' },
  CronJob: { abbr: 'CJ', color: '#fb923c' },
  Pod: { abbr: 'POD', color: '#22c55e' },
  Container: { abbr: 'CTR', color: '#10b981' },
  HorizontalPodAutoscaler: { abbr: 'HPA', color: '#84cc16' },
  PodDisruptionBudget: { abbr: 'PDB', color: '#65a30d' },
  ConfigMap: { abbr: 'CM', color: '#eab308' },
  Secret: { abbr: 'SEC', color: '#ef4444' },
  PersistentVolumeClaim: { abbr: 'PVC', color: '#0ea5e9' },
  PersistentVolume: { abbr: 'PV', color: '#0284c7' },
  StorageClass: { abbr: 'SC', color: '#0369a1' },
  ResourceQuota: { abbr: 'RQ', color: '#a3a3a3' },
  LimitRange: { abbr: 'LR', color: '#a3a3a3' },
  NetworkPolicy: { abbr: 'NP', color: '#f43f5e' },
  ServiceAccount: { abbr: 'SA', color: '#c084fc' },
  RoleBinding: { abbr: 'RB', color: '#d946ef' },
  ClusterRoleBinding: { abbr: 'CRB', color: '#d946ef' },
  Role: { abbr: 'ROL', color: '#e879f9' },
  ClusterRole: { abbr: 'CR', color: '#e879f9' },
  User: { abbr: 'USR', color: '#38bdf8' },
  Group: { abbr: 'GRP', color: '#38bdf8' },
};

export const EDGE_STYLES: Record<KubeEdgeKind, { color: string; dash: string | null }> = {
  routes: { color: '#ec4899', dash: null },
  selects: { color: '#3b82f6', dash: null },
  owns: { color: '#8b8fa7', dash: null },
  uses: { color: '#eab308', dash: '4 3' },
  bound: { color: '#0ea5e9', dash: '4 3' },
  scales: { color: '#84cc16', dash: '2 3' },
  protects: { color: '#65a30d', dash: '2 3' },
  applies: { color: '#f43f5e', dash: '2 3' },
  allows: { color: '#f43f5e', dash: null },
  'runs-as': { color: '#c084fc', dash: '4 3' },
  binds: { color: '#d946ef', dash: '4 3' },
  grants: { color: '#e879f9', dash: '4 3' },
};

const GROUP_COLORS = ['#6366f1', '#22c55e', '#f59e0b', '#3b82f6', '#ec4899', '#14b8a6', '#f97316', '#8b5cf6'];

export const getGroupColor = (index: number) => GROUP_COLORS[index % GROUP_COLORS.length];

export const kindStyle = (kind: string) => KIND_STYLES[kind] || { abbr: kind.slice(0, 3).toUpperCase(), color: '#8b8fa7' };

/** Bubbles at cluster level scale with pod count; everything else is a fixed size. */
export const nodeRadius = (node: KubeGraphNode, level: KubeLevel) =>
  level === 'cluster' ? Math.min(26 + Math.sqrt(node.weight) * 5, 64) : 22;

/**
 * Left-to-right columns at namespace level: traffic enters on the left, flows
 * through Services to workloads, then out to what workloads depend on.
 * Unknown kinds (custom controllers) sit with the workloads.
 */
const NAMESPACE_COLUMNS: Record<string, number> = {
  Ingress: 0, Gateway: 0,
  HTTPRoute: 1, GRPCRoute: 1, TLSRoute: 1, TCPRoute: 1, UDPRoute: 1,
  Service: 2,
  HorizontalPodAutoscaler: 3, PodDisruptionBudget: 3, NetworkPolicy: 3, ResourceQuota: 3, LimitRange: 3,
  ConfigMap: 5, Secret: 5, PersistentVolumeClaim: 5, ServiceAccount: 5,
  PersistentVolume: 6, RoleBinding: 6, ClusterRoleBinding: 6,
  StorageClass: 7, Role: 7, ClusterRole: 7,
};
const WORKLOAD_COLUMN = 4;

/** Cluster-wide storage lens: claims → volumes → classes. */
const STORAGE_COLUMNS: Record<string, number> = { PersistentVolumeClaim: 0, PersistentVolume: 1, StorageClass: 2 };

/** Cluster-wide access lens: who → binding → role. */
const ACCESS_COLUMNS: Record<string, number> = {
  User: 0, Group: 0, ServiceAccount: 0,
  RoleBinding: 1, ClusterRoleBinding: 1,
  Role: 2, ClusterRole: 2,
};

/** Depth along ownership edges, e.g. Deployment 0 → ReplicaSet 1 → Pod 2. */
const ownershipDepth = (graph: KubeGraph): Map<string, number> => {
  const depth = new Map<string, number>();
  const targets = new Set(graph.edges.map((e) => e.target as string));
  let frontier = graph.nodes.filter((n) => !targets.has(n.id)).map((n) => n.id);
  frontier.forEach((id) => depth.set(id, 0));

  while (frontier.length > 0) {
    const next: string[] = [];
    for (const edge of graph.edges) {
      const source = edge.source as string;
      const target = edge.target as string;
      if (frontier.includes(source) && !depth.has(target)) {
        depth.set(target, depth.get(source)! + 1);
        next.push(target);
      }
    }
    frontier = next;
  }
  return depth;
};

const nodeColumns = (graph: KubeGraph): Map<string, number> => {
  if (graph.level === 'namespace') {
    // Drop empty columns so a namespace without e.g. Gateways isn't padded
    const raw = graph.nodes.map((n) => [n.id, NAMESPACE_COLUMNS[n.kind] ?? WORKLOAD_COLUMN] as const);
    const used = [...new Set(raw.map(([, col]) => col))].sort((a, b) => a - b);
    return new Map(raw.map(([id, col]) => [id, used.indexOf(col)]));
  }
  if (graph.level === 'object') return ownershipDepth(graph);
  if (graph.level === 'kind') return new Map();
  if (graph.nodes.some((n) => n.kind in STORAGE_COLUMNS)) {
    return new Map(graph.nodes.map((n) => [n.id, STORAGE_COLUMNS[n.kind] ?? 0]));
  }
  if (graph.nodes.some((n) => n.kind === 'RoleBinding' || n.kind === 'ClusterRoleBinding')) {
    return new Map(graph.nodes.map((n) => [n.id, ACCESS_COLUMNS[n.kind] ?? 0]));
  }
  return new Map();
};

export interface LayoutTarget {
  x: number;
  y: number;
}

/**
 * Anchor points the force simulation pulls each node toward: columns for
 * layered levels, a grid of cells for grouped nodes, nothing for free bubbles.
 */
export const computeLayout = (graph: KubeGraph, width: number, height: number) => {
  const columns = nodeColumns(graph);
  const targets = new Map<string, LayoutTarget>();
  const groupCenters = new Map<string, LayoutTarget>();

  const ungroupedColumns = [...new Set(graph.nodes.filter((n) => !n.group && columns.has(n.id)).map((n) => columns.get(n.id)!))];
  const columnCount = Math.max(...ungroupedColumns, -1) + 1 + (graph.groups.length > 0 ? 1 : 0);
  const columnWidth = columnCount > 1 ? Math.min(Math.max(240, (width - 200) / (columnCount - 1)), 380) : 0;
  const columnX = (col: number) => (columnCount > 1 ? 100 + col * columnWidth : width / 2);

  // Stack each column's nodes vertically, sorted by name
  const byColumn = new Map<number, KubeGraphNode[]>();
  for (const n of graph.nodes) {
    if (n.group || !columns.has(n.id)) continue;
    const col = columns.get(n.id)!;
    byColumn.set(col, [...(byColumn.get(col) || []), n]);
  }
  byColumn.forEach((nodes, col) => {
    nodes.sort((a, b) => a.name.localeCompare(b.name));
    const spacing = Math.max(110, height / (nodes.length + 1));
    const top = height / 2 - (spacing * (nodes.length - 1)) / 2;
    nodes.forEach((n, i) => targets.set(n.id, { x: columnX(col), y: top + i * spacing }));
  });

  // Groups fill a grid to the right of the columns (or the whole canvas)
  if (graph.groups.length > 0) {
    const left = ungroupedColumns.length > 0 ? columnX(Math.max(...ungroupedColumns) + 1) - 140 : 0;
    const areaWidth = ungroupedColumns.length > 0 ? 280 * Math.ceil(graph.groups.length / 3) : Math.max(width, 300);
    // Next to columns, stack groups vertically so ownership edges stay short
    const cols = ungroupedColumns.length > 0
      ? Math.ceil(graph.groups.length / 3)
      : Math.ceil(Math.sqrt(graph.groups.length));
    const rows = Math.ceil(graph.groups.length / cols);
    const cellW = Math.max(areaWidth / cols, 280);
    const cellH = Math.max(height / rows, 260);
    graph.groups.forEach((grp, i) => {
      groupCenters.set(grp.id, {
        x: left + cellW * ((i % cols) + 0.5),
        y: cellH * (Math.floor(i / cols) + 0.5),
      });
    });
  }

  return { targets, groupCenters };
};

export const useGraphCanvas = ({ graph }: GraphCanvasProps) => {
  const positionsRef = useRef<Map<string, { x: number; y: number }>>(new Map());
  const fittedViewRef = useRef<string | null>(null);

  // Fresh copies each time: d3 mutates nodes and replaces edge endpoints with objects
  const { nodes, edges } = useMemo(() => {
    const saved = positionsRef.current;
    const nodes: KubeGraphNode[] = graph.nodes.map((n) => {
      const pos = saved.get(n.id);
      return { ...n, ...(pos ? { x: pos.x, y: pos.y } : {}) };
    });
    const edges: KubeGraphEdge[] = graph.edges.map((e) => ({ ...e }));
    return { nodes, edges };
  }, [graph]);

  const savePositions = useCallback((current: KubeGraphNode[]) => {
    for (const n of current) {
      if (n.x != null && n.y != null) positionsRef.current.set(n.id, { x: n.x, y: n.y });
    }
  }, []);

  return { nodes, edges, groups: graph.groups, level: graph.level, savePositions, fittedViewRef };
};
