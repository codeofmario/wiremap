import { useState, useEffect } from 'react';
import { KubeKind } from '../types/kubernetes';
import { kubeApi } from '../services/api';

export const useKubeKinds = (cluster: string) => {
  const [kinds, setKinds] = useState<KubeKind[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    kubeApi.getKinds(cluster)
      .then((data) => { if (!cancelled) { setKinds(data); setError(null); } })
      .catch((err) => { if (!cancelled) setError(err.message); })
      .finally(() => { if (!cancelled) setLoading(false); });
    return () => { cancelled = true; };
  }, [cluster]);

  return { kinds, loading, error };
};
