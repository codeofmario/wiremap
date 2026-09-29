import { useState, useEffect, useCallback } from 'react';
import { KubeGraph, KubeView } from '../types/kubernetes';
import { kubeApi } from '../services/api';
import { getKubeSocket } from '../services/socket';

// Changes arrive over the socket; polling only covers a dropped connection
const REFRESH_INTERVAL = 30000;

/** Changes only when something visible on the canvas changes, so polling doesn't re-layout. */
const graphKey = (g: KubeGraph) =>
  g.nodes.map((n) => `${n.id}:${n.status}:${n.summary}`).join(',') + '|' + g.edges.length;

/** The graph of one topology level; a null view fetches nothing. */
export const useKubeTopology = (cluster: string, view: KubeView | null) => {
  const [graph, setGraph] = useState<KubeGraph | null>(null);
  const [error, setError] = useState<string | null>(null);
  const viewKey = JSON.stringify(view);

  const refresh = useCallback(async () => {
    if (!view) return;
    try {
      const data = await kubeApi.getTopology(cluster, view);
      setGraph((prev) => (prev && graphKey(prev) === graphKey(data) ? prev : data));
      setError(null);
    } catch (err: any) {
      setError(err.message);
    }
  }, [cluster, viewKey]);

  useEffect(() => {
    setGraph(null);
    setError(null);
    refresh();
    const interval = setInterval(refresh, REFRESH_INTERVAL);
    return () => clearInterval(interval);
  }, [refresh]);

  // Namespace and object levels watch their namespace; cluster lenses and nodes watch the cluster
  const watchNamespace = view?.level === 'namespace' || view?.level === 'object' ? view.namespace || '' : '';
  useEffect(() => {
    if (!view) return;
    const socket = getKubeSocket();
    const scope = { cluster, namespace: watchNamespace };
    const key = `${cluster}/${watchNamespace}`;
    const onChanged = (data: { key: string }) => {
      if (data.key === key) refresh();
    };

    socket.emit('topology:watch', scope);
    socket.on('topology:changed', onChanged);
    return () => {
      socket.emit('topology:unwatch', scope);
      socket.off('topology:changed', onChanged);
    };
  }, [cluster, watchNamespace, refresh]);

  return { graph, loading: !!view && graph === null && error === null, error };
};
