import { useState, useCallback } from 'react';
import { ContainerInspect } from '../../../types/docker';

export interface InspectViewProps {
  inspect: ContainerInspect;
  host: string;
  containerId: string;
  onContainerIdChange?: (newId: string) => void;
  className?: string;
}

const SECRET_PATTERNS = /password|secret|key|token|api_key|auth/i;

export interface EnvVar {
  key: string;
  value: string;
  isSensitive: boolean;
}

export const useInspectView = ({ inspect, host, containerId, onContainerIdChange }: InspectViewProps) => {
  const [showSecrets, setShowSecrets] = useState(false);
  const [editing, setEditing] = useState(false);
  const [editedEnv, setEditedEnv] = useState<EnvVar[]>([]);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const envVars: EnvVar[] = (inspect.env || []).map((env) => {
    const eqIndex = env.indexOf('=');
    const key = env.slice(0, eqIndex);
    const value = env.slice(eqIndex + 1);
    const isSensitive = SECRET_PATTERNS.test(key);
    return { key, value, isSensitive };
  });

  const toggleSecrets = () => setShowSecrets((s) => !s);

  const startEditing = () => {
    setEditedEnv(envVars.map((e) => ({ ...e })));
    setEditing(true);
    setError(null);
  };

  const cancelEditing = () => {
    setEditing(false);
    setEditedEnv([]);
    setError(null);
  };

  const updateEnvValue = (index: number, value: string) => {
    setEditedEnv((prev) => prev.map((e, i) => i === index ? { ...e, value } : e));
  };

  const updateEnvKey = (index: number, key: string) => {
    setEditedEnv((prev) => prev.map((e, i) => i === index ? { ...e, key } : e));
  };

  const addEnvVar = () => {
    setEditedEnv((prev) => [...prev, { key: '', value: '', isSensitive: false }]);
  };

  const removeEnvVar = (index: number) => {
    setEditedEnv((prev) => prev.filter((_, i) => i !== index));
  };

  const saveEnv = useCallback(async () => {
    setSaving(true);
    setError(null);
    try {
      const env = editedEnv
        .filter((e) => e.key.trim() !== '')
        .map((e) => `${e.key}=${e.value}`);

      const hostParam = host ? `?host=${encodeURIComponent(host)}` : '';
      const res = await fetch(`/api/containers/${containerId}/env${hostParam}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ env }),
      });

      const data = await res.json();
      if (!res.ok) {
        throw new Error(data.message || 'Failed to update environment');
      }

      setEditing(false);
      setEditedEnv([]);

      if (data.id && onContainerIdChange) {
        onContainerIdChange(data.id);
      }
    } catch (err: any) {
      setError(err.message);
    } finally {
      setSaving(false);
    }
  }, [editedEnv, containerId]);

  return {
    inspect,
    envVars: editing ? editedEnv : envVars,
    showSecrets,
    toggleSecrets,
    editing,
    startEditing,
    cancelEditing,
    updateEnvValue,
    updateEnvKey,
    addEnvVar,
    removeEnvVar,
    saveEnv,
    saving,
    error,
  };
};
