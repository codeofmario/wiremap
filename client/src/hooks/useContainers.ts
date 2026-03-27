import { useState, useEffect, useCallback, useRef } from 'react';
import { ContainerInfo } from '../types/docker';
import { api } from '../services/api';

export const useContainers = (host: string) => {
  const [containers, setContainers] = useState<ContainerInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const initialLoad = useRef(true);

  const refresh = useCallback(async () => {
    if (!host) return;
    try {
      if (initialLoad.current) setLoading(true);
      const data = await api.getContainers(host);
      setContainers((prev) => {
        const prevKey = prev.map((c) => `${c.id}:${c.state}`).join(',');
        const nextKey = data.map((c: ContainerInfo) => `${c.id}:${c.state}`).join(',');
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
    setContainers([]);
    refresh();
    const interval = setInterval(refresh, 10000);
    return () => clearInterval(interval);
  }, [refresh]);

  return { containers, loading, error, refresh };
};
