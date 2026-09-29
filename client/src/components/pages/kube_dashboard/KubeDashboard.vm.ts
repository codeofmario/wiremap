import { ReactNode, useState, useEffect, useCallback, useMemo } from 'react';
import { useKubeHotkeys } from '../../../hooks/useKubeHotkeys';
import { useKubeKinds } from '../../../hooks/useKubeKinds';
import { useKubeNavigation } from '../../../hooks/useKubeNavigation';
import { useKubeObjects } from '../../../hooks/useKubeObjects';
import { useKubeTopology } from '../../../hooks/useKubeTopology';
import { useResizablePanel } from '../../../hooks/useResizablePanel';
import { formatAge } from '../../../services/format';
import { resolveKubeCommand } from '../../../services/kubeCommand';
import { KubeGraph, KubeGraphNode, KubeObject, PanelRequest } from '../../../types/kubernetes';

export interface KubeDashboardProps {
  cluster: string;
  sourcePicker: ReactNode;
}

type Category = 'all' | 'workloads' | 'config' | 'security';
type Display = 'map' | 'list';
type Prompt = 'command' | 'filter';

const CONFIG_KINDS = new Set([
  'ConfigMap', 'Secret', 'PersistentVolumeClaim', 'PersistentVolume', 'StorageClass', 'ResourceQuota', 'LimitRange',
]);
const SECURITY_KINDS = new Set([
  'ServiceAccount', 'RoleBinding', 'ClusterRoleBinding', 'Role', 'ClusterRole', 'NetworkPolicy',
]);

// Kinds hidden by each namespace-level filter
const HIDDEN_KINDS: Record<Category, Set<string>> = {
  all: new Set(),
  workloads: new Set([...CONFIG_KINDS, ...SECURITY_KINDS]),
  config: SECURITY_KINDS,
  security: CONFIG_KINDS,
};

export const CATEGORY_OPTIONS = [
  { id: 'all', label: 'Everything' },
  { id: 'workloads', label: 'Workloads' },
  { id: 'config', label: 'Config & storage' },
  { id: 'security', label: 'Security' },
];

export const DISPLAY_OPTIONS = [
  { id: 'map', label: 'Map' },
  { id: 'list', label: 'List' },
];

const ALL_NAMESPACES = '';

const withoutNodes = (graph: KubeGraph, keep: (n: KubeGraphNode) => boolean): KubeGraph => {
  const visible = new Set(graph.nodes.filter(keep).map((n) => n.id));
  return {
    ...graph,
    nodes: graph.nodes.filter((n) => visible.has(n.id)),
    edges: graph.edges.filter((e) => visible.has(e.source as string) && visible.has(e.target as string)),
  };
};

/** Every object of one kind as a map without edges, grouped by namespace when showing all of them. */
export const kindGraph = (objects: KubeObject[], groupByNamespace: boolean): KubeGraph => {
  const namespaces = [...new Set(objects.map((o) => o.namespace || ''))].filter(Boolean);
  const groupId = (ns: string) => `Namespace//${ns}`;
  return {
    level: 'kind',
    edges: [],
    groups: groupByNamespace ? namespaces.map((ns) => ({ id: groupId(ns), name: ns, kind: 'Namespace' })) : [],
    nodes: objects.map((o) => ({
      id: `${o.kind}/${o.namespace || ''}/${o.name}`,
      kind: o.kind,
      apiGroup: o.group,
      name: o.name,
      namespace: o.namespace,
      status: o.status,
      summary: o.summary,
      drillable: o.drillable,
      group: groupByNamespace && o.namespace ? groupId(o.namespace) : undefined,
      weight: 1,
      details: { Age: formatAge(o.created) },
    })),
  };
};

const byKindAndName = (a: KubeGraphNode, b: KubeGraphNode) => a.kind.localeCompare(b.kind) || a.name.localeCompare(b.name);

const hasShell = (node: KubeGraphNode | null) => node?.kind === 'Pod' || node?.kind === 'Container';

export const useKubeDashboard = ({ cluster, sourcePicker }: KubeDashboardProps) => {
  const navigation = useKubeNavigation(cluster);
  const { namespace, root, view } = navigation;
  const { kinds, loading: kindsLoading } = useKubeKinds(cluster);
  const topology = useKubeTopology(cluster, view);

  // A kind selected in the sidebar is listed; drilling from it switches to the topology
  const listedKind = root.type === 'kind' && !view
    ? kinds.find((k) => k.group === root.group && k.kind === root.kind) || null
    : null;
  const objects = useKubeObjects(cluster, listedKind, namespace);
  const namespaceKind = kinds.find((k) => k.group === '' && k.kind === 'Namespace') || null;
  const { items: namespaces } = useKubeObjects(cluster, namespaceKind, ALL_NAMESPACES);

  const [selected, setSelected] = useState<KubeGraphNode | null>(null);
  const [panelRequest, setPanelRequest] = useState<PanelRequest | null>(null);
  const [category, setCategory] = useState<Category>('all');
  const [display, setDisplay] = useState<Display>('map');
  const [prompt, setPrompt] = useState<Prompt | null>(null);
  const [command, setCommand] = useState('');
  const [commandError, setCommandError] = useState<string | null>(null);
  const [filter, setFilter] = useState('');
  const panel = useResizablePanel();

  const listing = root.type === 'kind' && !view;
  const graph = useMemo(() => {
    if (!listing) return topology.graph;
    if (!listedKind || objects.loading) return null;
    return kindGraph(objects.items, !namespace && listedKind.namespaced);
  }, [listing, topology.graph, listedKind, objects.items, objects.loading, namespace]);

  const kindMissing = listing && !kindsLoading && !listedKind;
  const error = listing ? (kindMissing ? `${root.type === 'kind' ? root.kind : ''} is not served by this cluster` : objects.error) : topology.error;
  const loading = listing ? kindsLoading || objects.loading : topology.loading;

  useEffect(() => {
    setSelected(null);
    setFilter('');
    panel.setPanelExpanded(false);
  }, [navigation.viewKey]);

  // Keep the side panel in sync when polling updates the selected node
  useEffect(() => {
    if (!selected || !graph) return;
    const fresh = graph.nodes.find((n) => n.id === selected.id);
    if (fresh) setSelected(fresh);
  }, [graph]);

  const visibleGraph = useMemo(() => {
    if (!graph) return null;
    const hidden = graph.level === 'namespace' ? HIDDEN_KINDS[category] : new Set<string>();
    const query = filter.trim().toLowerCase();
    return withoutNodes(graph, (n) =>
      !hidden.has(n.kind) && (!query || n.name.toLowerCase().includes(query) || n.kind.toLowerCase().includes(query)));
  }, [graph, category, filter]);

  const ordered = useMemo(() => [...(visibleGraph?.nodes || [])].sort(byKindAndName), [visibleGraph]);

  const handleSelect = useCallback((node: KubeGraphNode | null) => {
    setSelected(node);
    setPanelRequest(null);
    if (!node) panel.setPanelExpanded(false);
  }, [panel.setPanelExpanded]);

  const handleClose = useCallback(() => handleSelect(null), [handleSelect]);

  const requestPanel = (action: PanelRequest['action']) => setPanelRequest((prev) => ({ action, seq: (prev?.seq || 0) + 1 }));

  /** Enter / double-click: go one level deeper; a container opens its logs. */
  const open = useCallback((node: KubeGraphNode) => {
    if (node.kind === 'Container') {
      handleSelect(node);
      requestPanel('logs');
    } else if (node.drillable) {
      navigation.drill(node);
    } else {
      handleSelect(node);
    }
  }, [handleSelect, navigation.drill]);

  const submitCommand = () => {
    const result = resolveKubeCommand(command, kinds);
    if (!result) {
      setCommandError(`Unknown resource "${command.trim()}"`);
      return;
    }
    setPrompt(null);
    setCommandError(null);
    navigation.setRoot(result.root, result.namespace ?? namespace);
  };

  useKubeHotkeys({
    command: () => {
      setCommand('');
      setCommandError(null);
      setPrompt('command');
    },
    filter: () => setPrompt('filter'),
    open: () => selected && open(selected),
    escape: () => {
      if (selected) handleClose();
      else if (navigation.canGoBack) navigation.back();
    },
    back: () => navigation.canGoBack && navigation.back(),
    move: (delta) => {
      if (ordered.length === 0) return;
      const index = ordered.findIndex((n) => n.id === selected?.id);
      const next = index === -1 ? (delta === 1 ? 0 : ordered.length - 1) : (index + delta + ordered.length) % ordered.length;
      handleSelect(ordered[next]);
    },
    panel: (action) => {
      if (!selected || ((action === 'logs' || action === 'shell') && !hasShell(selected))) return;
      requestPanel(action);
    },
  });

  // A pod, or a container inside a pod's view, opens the pod panel
  const podSelection = selected?.kind === 'Pod'
    ? { name: selected.name, container: undefined }
    : selected?.kind === 'Container' && view?.kind === 'Pod' && view.name
      ? { name: view.name, container: selected.name }
      : null;

  const hints = [
    { keys: ':', label: 'resources' },
    { keys: '/', label: 'filter' },
    { keys: '↑↓', label: 'select' },
    ...(selected?.drillable ? [{ keys: 'enter', label: selected.kind === 'Container' ? 'logs' : 'open' }] : []),
    ...(hasShell(selected) ? [{ keys: 'l', label: 'logs' }, { keys: 's', label: 'shell' }] : []),
    ...(selected ? [{ keys: 'y', label: 'yaml' }, { keys: 'd', label: 'describe' }, { keys: 'esc', label: 'close' }] : []),
    ...(!selected && navigation.canGoBack ? [{ keys: 'esc', label: 'back' }] : []),
  ];

  const placeholder = error || (loading ? 'Loading...' : visibleGraph?.nodes.length === 0 ? (filter ? `Nothing matches "${filter}"` : 'Nothing here') : null);

  return {
    sourcePicker,
    cluster,
    kinds,
    namespace,
    root,
    namespaceOptions: [
      { value: ALL_NAMESPACES, label: 'All namespaces' },
      ...namespaces.map((n) => ({ value: n.name, label: n.name })),
    ],
    setNamespace: navigation.setNamespace,
    setRoot: (next: typeof root) => navigation.setRoot(next),
    crumbs: navigation.crumbs,
    goTo: navigation.goTo,
    viewKey: navigation.viewKey,
    showCategories: graph?.level === 'namespace',
    category,
    setCategory: (value: string) => setCategory(value as Category),
    showDisplay: listing,
    display: listing ? display : 'map',
    setDisplay: (value: string) => setDisplay(value as Display),
    prompt,
    command,
    setCommand: (value: string) => {
      setCommand(value);
      setCommandError(null);
    },
    commandError,
    submitCommand,
    closePrompt: () => setPrompt(null),
    filter,
    setFilter,
    clearFilter: () => {
      setFilter('');
      setPrompt(null);
    },
    graph: visibleGraph,
    listNodes: ordered,
    objectCount: `${visibleGraph?.nodes.length ?? 0}${listing && objects.hasMore ? '+' : ''} objects`,
    hasMore: listing && objects.hasMore,
    loadMore: objects.loadMore,
    loadingMore: objects.loadingMore,
    placeholder,
    hints,
    selected,
    podSelection,
    panelRequest,
    handleSelect,
    handleClose,
    open,
    drill: navigation.drill,
    ...panel,
  };
};
