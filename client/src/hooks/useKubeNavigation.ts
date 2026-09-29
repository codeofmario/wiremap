import { useState, useEffect, useCallback } from 'react';
import { KubeGraphNode, KubeRoot, KubeView } from '../types/kubernetes';
import { defaultRoute, KubeRoute, parseKubeHash, toKubeHash } from '../services/kubeRoute';
import { pluralKind } from '../services/format';

const routeFromHash = (cluster: string): KubeRoute => {
  const route = parseKubeHash(location.hash);
  return route?.cluster === cluster ? route : defaultRoute(cluster);
};

const rootLabel = ({ root, namespace }: KubeRoute): string => {
  switch (root.type) {
    case 'overview': return namespace || 'Cluster';
    case 'lens': return root.lens[0].toUpperCase() + root.lens.slice(1);
    case 'kind': return pluralKind(root.kind);
  }
};

const drillLabel = (view: KubeView) => (view.level === 'node' ? `Node ${view.name}` : `${view.kind} ${view.name}`);

/** The topology level the root shows; null when the root is a kind, which is listed instead. */
const rootView = ({ root, namespace }: KubeRoute): KubeView | null => {
  switch (root.type) {
    case 'overview': return namespace ? { level: 'namespace', namespace } : { level: 'cluster', lens: 'namespaces' };
    case 'lens': return { level: 'cluster', lens: root.lens };
    case 'kind': return null;
  }
};

/**
 * Where the user is in a cluster: namespace, what the sidebar selected, and the
 * objects opened from it. The URL hash is the source of truth, so browser
 * back/forward, reloads and shared links all land on the same view.
 */
export const useKubeNavigation = (cluster: string) => {
  const [route, setRoute] = useState<KubeRoute>(() => routeFromHash(cluster));

  const navigate = useCallback((next: Omit<KubeRoute, 'cluster'>) => {
    location.hash = toKubeHash({ cluster, ...next });
  }, [cluster]);

  useEffect(() => {
    const sync = () => setRoute(routeFromHash(cluster));
    if (parseKubeHash(location.hash)?.cluster !== cluster) {
      history.replaceState(null, '', toKubeHash(defaultRoute(cluster)));
    }
    sync();
    window.addEventListener('hashchange', sync);
    return () => {
      window.removeEventListener('hashchange', sync);
      history.replaceState(null, '', location.pathname + location.search);
    };
  }, [cluster]);

  const { namespace, root, drills } = route;

  /** Opens a drillable node: a namespace becomes the scope, anything else goes one level deeper. */
  const drill = useCallback((node: KubeGraphNode) => {
    if (!node.drillable) return;
    if (node.kind === 'Namespace') {
      navigate({ namespace: node.name, root: { type: 'overview' }, drills: [] });
    } else if (node.kind === 'Node') {
      navigate({ namespace, root, drills: [...drills, { level: 'node', name: node.name }] });
    } else {
      const view: KubeView = { level: 'object', namespace: node.namespace, kind: node.kind, group: node.apiGroup || '', name: node.name };
      navigate({ namespace, root, drills: [...drills, view] });
    }
  }, [navigate, namespace, root, drills]);

  return {
    namespace,
    root,
    /** The topology level to draw, or null when listing the root kind */
    view: drills.length > 0 ? drills[drills.length - 1] : rootView(route),
    viewKey: toKubeHash(route),
    crumbs: [rootLabel(route), ...drills.map(drillLabel)].map((label, i) => ({ id: String(i), label })),
    canGoBack: drills.length > 0,
    drill,
    back: useCallback(() => navigate({ namespace, root, drills: drills.slice(0, -1) }), [navigate, namespace, root, drills]),
    goTo: useCallback((index: number) => navigate({ namespace, root, drills: drills.slice(0, index) }), [navigate, namespace, root, drills]),
    setNamespace: useCallback((next: string) => navigate({ namespace: next, root, drills: [] }), [navigate, root]),
    setRoot: useCallback((next: KubeRoot, nextNamespace = namespace) => navigate({ namespace: nextNamespace, root: next, drills: [] }), [navigate, namespace]),
  };
};
