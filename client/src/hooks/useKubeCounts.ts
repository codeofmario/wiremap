import { useState, useEffect } from 'react';
import { kubeApi } from '../services/api';

const REFRESH_INTERVAL = 15000;

/** Object counts per "group/Kind" in a namespace ("" for all), refreshed periodically. */
export const useKubeCounts = (cluster: string, namespace: string, kinds: string[]) => {
  const [counts, setCounts] = useState<Record<string, number>>({});
  const kindsKey = kinds.join(',');

  useEffect(() => {
    if (kinds.length === 0) return;
    let cancelled = false;
    const refresh = () => kubeApi.getCounts(cluster, namespace, kinds)
      .then((data) => { if (!cancelled) setCounts(data); })
      .catch(() => { if (!cancelled) setCounts({}); });

    refresh();
    const interval = setInterval(refresh, REFRESH_INTERVAL);
    return () => {
      cancelled = true;
      clearInterval(interval);
    };
  }, [cluster, namespace, kindsKey]);

  return counts;
};
