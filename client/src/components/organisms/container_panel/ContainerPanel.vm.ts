import { useState } from 'react';
import { useLogs } from '../../../hooks/useLogs';
import { useStats } from '../../../hooks/useStats';
import { useContainerInspect } from '../../../hooks/useContainerInspect';
import { Tab } from '../../atoms/tabs/Tabs.vm';

export interface ContainerPanelProps {
  host: string;
  containerId: string;
  onClose: () => void;
  onContainerIdChange?: (newId: string) => void;
  expanded?: boolean;
  onToggleExpand?: () => void;
  className?: string;
}

const TABS: Tab[] = [
  { id: 'logs', label: 'Logs' },
  { id: 'stats', label: 'Stats' },
  { id: 'console', label: 'Console' },
  { id: 'inspect', label: 'Inspect' },
  { id: 'volumes', label: 'Volumes' },
  { id: 'ports', label: 'Ports' },
  { id: 'network', label: 'Network' },
];

export const useContainerPanel = ({ host, containerId }: ContainerPanelProps) => {
  const [activeTab, setActiveTab] = useState('logs');
  const { logs, clear: clearLogs } = useLogs(host, containerId);
  const { current: currentStats, history: statsHistory } = useStats(host, containerId);
  const { inspect, loading: inspectLoading } = useContainerInspect(host, containerId);

  return {
    tabs: TABS,
    activeTab,
    setActiveTab,
    logs,
    clearLogs,
    currentStats,
    statsHistory,
    inspect,
    inspectLoading,
    containerName: inspect?.name?.replace(/^\//, '') || containerId.slice(0, 12),
  };
};
