import { useState, useEffect, useCallback } from 'react';
import { LogEntry } from '../types/docker';
import { PodTarget, podTargetKey } from '../types/kubernetes';
import { getKubeSocket } from '../services/socket';

export const usePodLogs = (target: PodTarget | null) => {
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const key = target ? podTargetKey(target) : null;

  const clear = useCallback(() => setLogs([]), []);

  useEffect(() => {
    if (!target || !key) return;

    setLogs([]);
    const socket = getKubeSocket();
    socket.emit('pod-logs:subscribe', target);

    const handler = (data: LogEntry) => {
      if (data.containerId === key) {
        setLogs((prev) => [...prev.slice(-999), data]);
      }
    };

    // Surface stream failures (e.g. container not started yet) inline with the logs
    const onError = (data: { key: string; message: string }) => {
      if (data.key === key) handler({ containerId: key, type: 'stderr', text: data.message });
    };

    socket.on('pod-logs:data', handler);
    socket.on('pod-logs:error', onError);

    return () => {
      socket.emit('pod-logs:unsubscribe', target);
      socket.off('pod-logs:data', handler);
      socket.off('pod-logs:error', onError);
    };
  }, [key]);

  return { logs, clear };
};
