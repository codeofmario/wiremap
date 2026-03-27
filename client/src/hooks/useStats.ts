import { useState, useEffect } from 'react';
import { ContainerStats } from '../types/docker';
import { getSocket } from '../services/socket';

export const useStats = (host: string, containerId: string | null) => {
  const [history, setHistory] = useState<ContainerStats[]>([]);
  const [current, setCurrent] = useState<ContainerStats | null>(null);

  useEffect(() => {
    if (!containerId) return;

    setHistory([]);
    setCurrent(null);
    const socket = getSocket();
    socket.emit('stats:subscribe', { containerId, hostId: host });

    const handler = (data: { containerId: string; stats: ContainerStats }) => {
      if (data.containerId === containerId) {
        setCurrent(data.stats);
        setHistory((prev) => [...prev.slice(-59), data.stats]);
      }
    };

    socket.on('stats:data', handler);

    return () => {
      socket.emit('stats:unsubscribe', { containerId, hostId: host });
      socket.off('stats:data', handler);
    };
  }, [host, containerId]);

  return { current, history };
};
