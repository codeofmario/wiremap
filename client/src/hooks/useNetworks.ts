import { useState, useEffect, useCallback, useRef } from 'react';
import { NetworkInfo } from '../types/docker';
import { api } from '../services/api';

export const useNetworks = (host: string) => {
  const [networks, setNetworks] = useState<NetworkInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const initialLoad = useRef(true);

  const refresh = useCallback(async () => {
    if (!host) return;
    try {
      if (initialLoad.current) setLoading(true);
      const data = await api.getNetworks(host);
      setNetworks((prev) => {
        const prevKey = prev.map((n) => n.id).join(',');
        const nextKey = data.map((n: NetworkInfo) => n.id).join(',');
        return prevKey === nextKey ? prev : data;
      });
      setError(null);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
      initialLoad.current = false;
    }
  }, [host]);

  useEffect(() => {
    initialLoad.current = true;
    setNetworks([]);
    refresh();
    const interval = setInterval(refresh, 10000);
    return () => clearInterval(interval);
  }, [refresh]);

  return { networks, loading, error, refresh };
};
