import { useState, useEffect, useCallback } from 'react';
import { KubeResource } from '../types/kubernetes';
import { kubeApi } from '../services/api';

export const useKubeResource = (cluster: string, kind: string, group: string, name: string, namespace?: string) => {
  const [resource, setResource] = useState<KubeResource | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [version, setVersion] = useState(0);

  // Re-fetch after an action changed the object
  const refresh = useCallback(() => setVersion((v) => v + 1), []);

  useEffect(() => {
    let cancelled = false;
    setError(null);

    kubeApi.getResource(cluster, kind, group, name, namespace)
      .then((data) => { if (!cancelled) setResource(data); })
      .catch((err) => { if (!cancelled) setError(err.message); });

    return () => { cancelled = true; };
  }, [cluster, kind, group, name, namespace, version]);

  return { resource, loading: resource === null && error === null, error, refresh };
};
