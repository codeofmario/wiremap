export type KubeLevel = 'cluster' | 'namespace' | 'object' | 'node' | 'kind';
export type KubeLens = 'namespaces' | 'nodes' | 'storage' | 'access';
export type KubeNodeStatus = 'idle' | 'healthy' | 'warning' | 'error';

export interface KubeCluster {
  name: string;
  connected: boolean;
}

export interface KubeGraphNode {
  id: string;
  kind: string;
  /** API group of the kind, "" or absent for core kinds */
  apiGroup?: string;
  name: string;
  namespace?: string;
  status: KubeNodeStatus;
  summary: string;
  drillable: boolean;
  group?: string;
  weight: number;
  details?: Record<string, string>;
  x?: number;
  y?: number;
  vx?: number;
  vy?: number;
  fx?: number | null;
  fy?: number | null;
}

export type KubeEdgeKind =
  | 'routes' | 'selects' | 'uses' | 'owns' | 'bound'
  | 'scales' | 'protects' | 'applies' | 'allows'
  | 'runs-as' | 'binds' | 'grants';

export interface KubeGraphEdge {
  source: string | KubeGraphNode;
  target: string | KubeGraphNode;
  kind: KubeEdgeKind;
}

export interface KubeGraphGroup {
  id: string;
  name: string;
  kind: string;
}

export interface KubeGraph {
  level: KubeLevel;
  nodes: KubeGraphNode[];
  edges: KubeGraphEdge[];
  groups: KubeGraphGroup[];
}

/** What the sidebar shows on the map: the namespace (or cluster) overview, a cluster lens, or every object of one kind. */
export type KubeRoot =
  | { type: 'overview' }
  | { type: 'lens'; lens: Exclude<KubeLens, 'namespaces'> }
  | { type: 'kind'; group: string; kind: string };

/** A topology level: the overview and lenses, or one drill-down step into an object or node. */
export interface KubeView {
  level: KubeLevel;
  lens?: KubeLens;
  namespace?: string;
  kind?: string;
  group?: string;
  name?: string;
}

export interface KubeEvent {
  type: string;
  reason: string;
  message: string;
  count: number;
  lastSeen: string;
  object: string;
}

export interface KubeResource {
  kind: string;
  group?: string;
  name: string;
  namespace?: string;
  yaml: string;
  events: KubeEvent[];
  actions: KubeActions;
}

/** What the UI can do to an object; absent fields mean the action doesn't apply. */
export interface KubeActions {
  edit: boolean;
  delete: boolean;
  restart: boolean;
  trigger: boolean;
  replicas?: number;
  suspended?: boolean;
  unschedulable?: boolean;
}

export interface KubeContainer {
  name: string;
  image: string;
  ready: boolean;
  restartCount: number;
  state: string;
  reason?: string;
  init: boolean;
}

export interface KubePod {
  name: string;
  namespace: string;
  node: string;
  phase: string;
  status: KubeNodeStatus;
  podIp: string;
  hostIp: string;
  startTime?: string;
  labels: Record<string, string> | null;
  containers: KubeContainer[];
}

/** Identifies a pod container for log/stats/exec streams. */
export interface PodTarget {
  cluster: string;
  namespace: string;
  pod: string;
  container: string;
}

export const podTargetKey = (t: PodTarget): string => `${t.cluster}/${t.namespace}/${t.pod}/${t.container}`;

/** A listable resource type served by the cluster, built-in or custom. */
export interface KubeKind {
  group: string;
  version: string;
  resource: string;
  kind: string;
  namespaced: boolean;
  /** Aliases such as "po" or "deploy" */
  shortNames?: string[];
}

export interface KubeObject {
  name: string;
  namespace?: string;
  kind: string;
  group?: string;
  status: KubeNodeStatus;
  summary: string;
  created: string;
  drillable: boolean;
}

export interface KubeObjectList {
  items: KubeObject[];
  /** Token for the next page; absent on the last one */
  continue?: string;
}

/** A hotkey asking the side panel to show one of its views; seq makes repeats distinct. */
export interface PanelRequest {
  action: 'logs' | 'shell' | 'yaml' | 'describe';
  seq: number;
}
