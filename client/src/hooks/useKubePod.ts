import { useState, useEffect } from 'react';
import { KubePod } from '../types/kubernetes';
import { kubeApi } from '../services/api';

export const useKubePod = (cluster: string, namespace: string, name: string) => {
  const [pod, setPod] = useState<KubePod | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setPod(null);
    setError(null);

    kubeApi.getPod(cluster, namespace, name)
      .then((data) => { if (!cancelled) setPod(data); })
      .catch((err) => { if (!cancelled) setError(err.message); });

    return () => { cancelled = true; };
  }, [cluster, namespace, name]);

  return { pod, error };
};
