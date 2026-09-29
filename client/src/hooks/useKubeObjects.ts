import { useState, useEffect, useCallback, useRef } from 'react';
import { KubeKind, KubeObject } from '../types/kubernetes';
import { kubeApi } from '../services/api';

const REFRESH_INTERVAL = 10000;

/**
 * Objects of one kind, a page at a time. Auto-refresh re-reads the first page,
 * so it pauses once more pages have been loaded to keep them on screen.
 */
export const useKubeObjects = (cluster: string, kind: KubeKind | null, namespace: string) => {
  const [items, setItems] = useState<KubeObject[] | null>(null);
  const [continueToken, setContinueToken] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [loadingMore, setLoadingMore] = useState(false);
  const extraPages = useRef(0);
  const kindKey = kind ? `${kind.group}/${kind.version}/${kind.resource}` : '';
  const scope = kind?.namespaced ? namespace : '';

  const refresh = useCallback(async () => {
    if (!kind || extraPages.current > 0) return;
    try {
      const page = await kubeApi.getObjects(cluster, kind, scope);
      setItems(page.items);
      setContinueToken(page.continue || '');
      setError(null);
    } catch (err: any) {
      setError(err.message);
    }
  }, [cluster, kindKey, scope]);

  const loadMore = useCallback(async () => {
    if (!kind || !continueToken) return;
    setLoadingMore(true);
    try {
      const page = await kubeApi.getObjects(cluster, kind, scope, continueToken);
      extraPages.current++;
      setItems((prev) => [...(prev || []), ...page.items]);
      setContinueToken(page.continue || '');
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoadingMore(false);
    }
  }, [cluster, kindKey, scope, continueToken]);

  useEffect(() => {
    extraPages.current = 0;
    setItems(null);
    setContinueToken('');
    setError(null);
    refresh();
    const interval = setInterval(refresh, REFRESH_INTERVAL);
    return () => clearInterval(interval);
  }, [refresh]);

  return {
    items: items || [],
    hasMore: continueToken !== '',
    loadMore,
    loadingMore,
    loading: !!kind && items === null && error === null,
    error,
  };
};
