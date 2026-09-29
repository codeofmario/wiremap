import { useState, useEffect } from 'react';
import { api, kubeApi } from '../services/api';
import { parseKubeHash } from '../services/kubeRoute';

export type SourceKind = 'docker' | 'kubernetes';

export interface Source {
  id: string;
  kind: SourceKind;
  name: string;
  connected: boolean;
}

const sourceId = (kind: SourceKind, name: string) => `${kind}:${name}`;

/** Picks the cluster from a #/k8s deep link, otherwise the first connected source. */
const initialSource = (sources: Source[]): string => {
  const route = parseKubeHash(location.hash);
  const linked = route && sources.find((s) => s.id === sourceId('kubernetes', route.cluster) && s.connected);
  return (linked || sources.find((s) => s.connected) || sources[0])?.id || '';
};

export const useSources = () => {
  const [sources, setSources] = useState<Source[]>([]);
  const [selectedId, setSelectedId] = useState('');
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;

    const fetchSources = (retries = 5) => {
      Promise.all([api.getHosts(), kubeApi.getClusters()])
        .then(([hosts, clusters]) => {
          if (cancelled) return;
          const all: Source[] = [
            ...hosts.map((h) => ({ id: sourceId('docker', h.name), kind: 'docker' as const, name: h.name, connected: h.connected })),
            ...clusters.map((c) => ({ id: sourceId('kubernetes', c.name), kind: 'kubernetes' as const, name: c.name, connected: c.connected })),
          ];
          setSources(all);
          setSelectedId(initialSource(all));
          setLoading(false);
        })
        .catch(() => {
          if (cancelled) return;
          if (retries > 0) {
            setTimeout(() => fetchSources(retries - 1), 1000);
          } else {
            setLoading(false);
          }
        });
    };

    fetchSources();

    return () => { cancelled = true; };
  }, []);

  return {
    sources,
    selectedSource: sources.find((s) => s.id === selectedId) || null,
    selectSource: setSelectedId,
    loading,
  };
};
