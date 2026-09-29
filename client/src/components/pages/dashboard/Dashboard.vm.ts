import { ReactNode, useState, useCallback } from 'react';
import { useContainers } from '../../../hooks/useContainers';
import { useNetworks } from '../../../hooks/useNetworks';
import { useResizablePanel } from '../../../hooks/useResizablePanel';

export type ViewMode = 'canvas' | 'list';

export interface DashboardProps {
  host: string;
  sourcePicker: ReactNode;
}

export const useDashboard = ({ host, sourcePicker }: DashboardProps) => {
  const [selectedContainerId, setSelectedContainerId] = useState<string | null>(null);
  const [viewMode, setViewMode] = useState<ViewMode>('canvas');
  const { panelExpanded, setPanelExpanded, panelWidth, toggleExpand, handlePanelResize, handlePanelResizeEnd } = useResizablePanel();
  const { containers, loading: containersLoading, refresh: refreshContainers } = useContainers(host);
  const { networks, loading: networksLoading, refresh: refreshNetworks } = useNetworks(host);

  const handleSelectContainer = useCallback((id: string | null) => {
    setSelectedContainerId(id);
    if (!id) setPanelExpanded(false);
  }, [setPanelExpanded]);

  const handleContainerIdChange = useCallback((newId: string) => {
    setSelectedContainerId(newId);
    refreshContainers();
    refreshNetworks();
  }, [refreshContainers, refreshNetworks]);

  const handleClosePanel = useCallback(() => {
    setSelectedContainerId(null);
    setPanelExpanded(false);
  }, [setPanelExpanded]);

  return {
    host,
    sourcePicker,
    containers,
    networks,
    loading: containersLoading || networksLoading,
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
