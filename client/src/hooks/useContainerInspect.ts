import { useState, useEffect } from 'react';
import { ContainerInspect } from '../types/docker';
import { api } from '../services/api';

export const useContainerInspect = (host: string, containerId: string | null) => {
  const [inspect, setInspect] = useState<ContainerInspect | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!containerId) {
      setInspect(null);
      return;
    }

    setLoading(true);
    api.inspectContainer(host, containerId)
      .then(setInspect)
      .catch(() => setInspect(null))
      .finally(() => setLoading(false));
  }, [host, containerId]);

  return { inspect, loading };
};
