import { useState, useCallback } from 'react';
import { useHosts } from '../../../hooks/useHosts';
import { useContainers } from '../../../hooks/useContainers';
import { useNetworks } from '../../../hooks/useNetworks';

export type ViewMode = 'canvas' | 'list';

const PANEL_WIDTH_KEY = 'wiremap:panelWidth';
const DEFAULT_PANEL_WIDTH = 480;
const MIN_PANEL_WIDTH = 320;
const MAX_PANEL_WIDTH = 1200;

const loadPanelWidth = (): number => {
  const stored = localStorage.getItem(PANEL_WIDTH_KEY);
  if (stored) {
    const n = parseInt(stored, 10);
    if (!isNaN(n) && n >= MIN_PANEL_WIDTH && n <= MAX_PANEL_WIDTH) return n;
  }
  return DEFAULT_PANEL_WIDTH;
};

export const useDashboard = () => {
  const [selectedContainerId, setSelectedContainerId] = useState<string | null>(null);
  const [panelExpanded, setPanelExpanded] = useState(false);
  const [panelWidth, setPanelWidth] = useState(loadPanelWidth);
  const [viewMode, setViewMode] = useState<ViewMode>('canvas');
  const { hosts, selectedHost, setSelectedHost, loading: hostsLoading } = useHosts();
  const { containers, loading: containersLoading, refresh: refreshContainers } = useContainers(selectedHost);
  const { networks, loading: networksLoading, refresh: refreshNetworks } = useNetworks(selectedHost);

  const handleSelectContainer = useCallback((id: string | null) => {
    setSelectedContainerId(id);
    if (!id) setPanelExpanded(false);
  }, []);

  const handleContainerIdChange = useCallback((newId: string) => {
    setSelectedContainerId(newId);
    refreshContainers();
    refreshNetworks();
  }, [refreshContainers, refreshNetworks]);

  const handleClosePanel = useCallback(() => {
    setSelectedContainerId(null);
    setPanelExpanded(false);
  }, []);

  const handleHostChange = useCallback((host: string) => {
    setSelectedHost(host);
    setSelectedContainerId(null);
    setPanelExpanded(false);
  }, [setSelectedHost]);

  const toggleExpand = () => setPanelExpanded((s) => !s);

  const handlePanelResize = useCallback((delta: number) => {
    setPanelWidth((w) => Math.max(MIN_PANEL_WIDTH, Math.min(MAX_PANEL_WIDTH, w + delta)));
  }, []);

  const handlePanelResizeEnd = useCallback(() => {
    setPanelWidth((w) => {
      localStorage.setItem(PANEL_WIDTH_KEY, String(w));
      return w;
    });
  }, []);

  return {
    hosts,
    selectedHost,
    handleHostChange,
    containers,
    networks,
    loading: hostsLoading || containersLoading || networksLoading,
    selectedContainerId,
    panelExpanded,
    panelWidth,
    viewMode,
    setViewMode,
    handleSelectContainer,
    handleContainerIdChange,
    handleClosePanel,
    toggleExpand,
    handlePanelResize,
    handlePanelResizeEnd,
  };
};
