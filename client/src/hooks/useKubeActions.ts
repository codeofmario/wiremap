import { useState, useCallback } from 'react';
import { KubeTarget } from '../services/api';

/**
 * Runs cluster actions one at a time, tracking the outcome to show next to the
 * controls. onDone runs after each success so views can re-fetch; run resolves
 * to whether the action succeeded.
 */
export const useKubeActions = (target: KubeTarget, onDone: () => void) => {
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const run = useCallback(async (action: (t: KubeTarget) => Promise<unknown>, success: (result: any) => string): Promise<boolean> => {
    setBusy(true);
    setError(null);
    setMessage(null);
    try {
      const result = await action(target);
      setMessage(success(result));
      onDone();
      return true;
    } catch (err: any) {
      setError(err.message);
      return false;
    } finally {
      setBusy(false);
    }
  }, [target.cluster, target.kind, target.group, target.name, target.namespace, onDone]);

  return { run, busy, message, error };
};
