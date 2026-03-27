import { useState, useEffect } from 'react';
import { HostInfo } from '../types/docker';
import { api } from '../services/api';

export const useHosts = () => {
  const [hosts, setHosts] = useState<HostInfo[]>([]);
  const [selectedHost, setSelectedHost] = useState('');
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;

    const fetchHosts = (retries = 5) => {
      api.getHosts()
        .then((data) => {
          if (cancelled) return;
          setHosts(data);
          if (data.length > 0 && !selectedHost) {
            setSelectedHost(data[0].name);
          }
          setLoading(false);
        })
        .catch(() => {
          if (cancelled) return;
          if (retries > 0) {
            setTimeout(() => fetchHosts(retries - 1), 1000);
          } else {
            setLoading(false);
          }
        });
    };

    fetchHosts();

    return () => { cancelled = true; };
  }, []);

  return { hosts, selectedHost, setSelectedHost, loading };
};
