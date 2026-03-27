import { useState, useCallback } from 'react';
import { ContainerInspect, MountInfo } from '../../../types/docker';

export interface VolumeBrowserProps {
  inspect: ContainerInspect;
  host: string;
  containerId: string;
  className?: string;
}

export interface FileEntry {
  name: string;
  path: string;
  isDir: boolean;
  size: number;
  permissions: string;
}

export interface FileContent {
  path: string;
  content: string;
  binary: boolean;
  size: number;
}

export const useVolumeBrowser = ({ inspect, host, containerId }: VolumeBrowserProps) => {
  const [currentPath, setCurrentPath] = useState<string | null>(null);
  const [entries, setEntries] = useState<FileEntry[]>([]);
  const [loading, setLoading] = useState(false);
  const [fileContent, setFileContent] = useState<FileContent | null>(null);
  const [editedContent, setEditedContent] = useState<string | null>(null);
  const [fileLoading, setFileLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [saveStatus, setSaveStatus] = useState<string | null>(null);

  const mounts: MountInfo[] = inspect.mounts || [];

  const navigateTo = useCallback(async (path: string) => {
    setLoading(true);
    setFileContent(null);
    setEditedContent(null);
    setSaveStatus(null);
    try {
      const res = await fetch(`/api/containers/${containerId}/fs?path=${encodeURIComponent(path)}${host ? `&host=${encodeURIComponent(host)}` : ''}`);
      if (!res.ok) throw new Error('Failed to list directory');
      const data = await res.json();
      setEntries(data);
      setCurrentPath(path);
    } catch {
      setEntries([]);
    } finally {
      setLoading(false);
    }
  }, [containerId]);

  const openFile = useCallback(async (path: string) => {
    setFileLoading(true);
    setSaveStatus(null);
    try {
      const res = await fetch(`/api/containers/${containerId}/fs/read?path=${encodeURIComponent(path)}${host ? `&host=${encodeURIComponent(host)}` : ''}`);
      if (!res.ok) throw new Error('Failed to read file');
      const data = await res.json();
      setFileContent(data);
      setEditedContent(data.content);
    } catch {
      setFileContent(null);
      setEditedContent(null);
    } finally {
      setFileLoading(false);
    }
  }, [containerId]);

  const saveFile = useCallback(async () => {
    if (!fileContent || editedContent === null) return;
    setSaving(true);
    setSaveStatus(null);
    try {
      const res = await fetch(`/api/containers/${containerId}/fs/write${host ? `?host=${encodeURIComponent(host)}` : ''}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ path: fileContent.path, content: editedContent }),
      });
      if (!res.ok) throw new Error('Failed to save file');
      setFileContent({ ...fileContent, content: editedContent });
      setSaveStatus('Saved');
      setTimeout(() => setSaveStatus(null), 2000);
    } catch {
      setSaveStatus('Failed to save');
    } finally {
      setSaving(false);
    }
  }, [containerId, fileContent, editedContent]);

  const goUp = useCallback(() => {
    if (!currentPath || currentPath === '/') return;
    const parent = currentPath.split('/').slice(0, -1).join('/') || '/';
    navigateTo(parent);
  }, [currentPath, navigateTo]);

  const breadcrumbs = currentPath
    ? currentPath.split('/').filter(Boolean).reduce<Array<{ name: string; path: string }>>(
        (acc, part) => {
          const prev = acc.length > 0 ? acc[acc.length - 1].path : '';
          acc.push({ name: part, path: `${prev}/${part}` });
          return acc;
        },
        [],
      )
    : [];

  const isDirty = editedContent !== null && fileContent !== null && editedContent !== fileContent.content;

  return {
    mounts,
    currentPath,
    entries,
    loading,
    fileContent,
    editedContent,
    setEditedContent,
    fileLoading,
    saving,
    saveStatus,
    isDirty,
    navigateTo,
    openFile,
    saveFile,
    goUp,
    breadcrumbs,
    closeFile: () => { setFileContent(null); setEditedContent(null); setSaveStatus(null); },
    downloadFile: () => {
      if (!fileContent) return;
      const content = editedContent ?? fileContent.content;
      const blob = new Blob([content], { type: 'application/octet-stream' });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = fileContent.path.split('/').pop() || 'file';
      a.click();
      URL.revokeObjectURL(url);
    },
  };
};

export const formatSize = (bytes: number): string => {
  if (bytes === 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / Math.pow(1024, i)).toFixed(i === 0 ? 0 : 1)} ${units[i]}`;
};
