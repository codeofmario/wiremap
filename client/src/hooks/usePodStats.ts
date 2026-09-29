import { useState, useEffect } from 'react';
import { ContainerStats } from '../types/docker';
import { PodTarget, podTargetKey } from '../types/kubernetes';
import { getKubeSocket } from '../services/socket';

export const usePodStats = (target: PodTarget | null) => {
  const [history, setHistory] = useState<ContainerStats[]>([]);
  const [current, setCurrent] = useState<ContainerStats | null>(null);
  const [error, setError] = useState<string | null>(null);
  const key = target ? podTargetKey(target) : null;

  useEffect(() => {
    if (!target || !key) return;

    setHistory([]);
    setCurrent(null);
    setError(null);
    const socket = getKubeSocket();
    socket.emit('pod-stats:subscribe', target);

    const onData = (data: { key: string; stats: ContainerStats }) => {
      if (data.key !== key) return;
      setCurrent(data.stats);
      setHistory((prev) => [...prev.slice(-59), data.stats]);
      setError(null);
    };
    const onError = (data: { key: string; message: string }) => {
      if (data.key === key) setError(data.message);
    };

    socket.on('pod-stats:data', onData);
    socket.on('pod-stats:error', onError);

    return () => {
      socket.emit('pod-stats:unsubscribe', target);
      socket.off('pod-stats:data', onData);
      socket.off('pod-stats:error', onError);
    };
  }, [key]);

  return { current, history, error };
};
