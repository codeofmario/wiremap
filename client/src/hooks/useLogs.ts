import { useState, useEffect, useCallback } from 'react';
import { LogEntry } from '../types/docker';
import { getSocket } from '../services/socket';

export const useLogs = (host: string, containerId: string | null) => {
  const [logs, setLogs] = useState<LogEntry[]>([]);

  const clear = useCallback(() => setLogs([]), []);

  useEffect(() => {
    if (!containerId) return;

    setLogs([]);
    const socket = getSocket();
    socket.emit('logs:subscribe', { containerId, hostId: host });

    const handler = (data: LogEntry) => {
      if (data.containerId === containerId) {
        setLogs((prev) => [...prev.slice(-999), data]);
      }
    };

    socket.on('logs:data', handler);

    return () => {
      socket.emit('logs:unsubscribe', { containerId, hostId: host });
      socket.off('logs:data', handler);
    };
  }, [host, containerId]);

  return { logs, clear };
};
