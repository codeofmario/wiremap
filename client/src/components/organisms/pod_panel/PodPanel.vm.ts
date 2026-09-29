import { useState, useEffect, useMemo } from 'react';
import { kubeActions, KubeTarget } from '../../../services/api';
import { useKubePod } from '../../../hooks/useKubePod';
import { useKubeResource } from '../../../hooks/useKubeResource';
import { usePodLogs } from '../../../hooks/usePodLogs';
import { usePodStats } from '../../../hooks/usePodStats';
import { PanelRequest, PodTarget } from '../../../types/kubernetes';
import { Tab } from '../../atoms/tabs/Tabs.vm';

export interface PodPanelProps {
  cluster: string;
  namespace: string;
  name: string;
  /** Container to open with; defaults to the first app container */
  container?: string;
  /** Opens the pod's containers level; absent when already inside the pod */
  onOpen?: () => void;
  /** Hotkey asking for a tab */
  request?: PanelRequest | null;
  onClose: () => void;
  expanded?: boolean;
  onToggleExpand?: () => void;
}

const TABS: Tab[] = [
  { id: 'logs', label: 'Logs' },
  { id: 'metrics', label: 'Metrics' },
  { id: 'console', label: 'Console' },
  { id: 'yaml', label: 'YAML' },
  { id: 'events', label: 'Events' },
];

const REQUEST_TABS: Record<PanelRequest['action'], string> = { logs: 'logs', shell: 'console', yaml: 'yaml', describe: 'events' };

const STATUS_BADGES = { healthy: 'success', warning: 'warning', error: 'error', idle: 'default' } as const;

export const usePodPanel = ({ cluster, namespace, name, container: initialContainer, request }: PodPanelProps) => {
  const [activeTab, setActiveTab] = useState('logs');

  useEffect(() => {
    if (request) setActiveTab(REQUEST_TABS[request.action]);
  }, [request?.seq]);
  const [container, setContainer] = useState('');
  const { pod, error: podError } = useKubePod(cluster, namespace, name);
  const { resource, error: resourceError, refresh: refreshResource } = useKubeResource(cluster, 'Pod', '', name, namespace);
  const resourceTarget: KubeTarget = useMemo(() => ({ cluster, kind: 'Pod', group: '', name, namespace }), [cluster, name, namespace]);

  const saveYaml = async (yaml: string) => {
    await kubeActions.update(resourceTarget, yaml);
    refreshResource();
  };

  // Open the requested container, else the first app container, once the pod is loaded
  useEffect(() => {
    const first = pod?.containers.find((c) => c.name === initialContainer)
      || pod?.containers.find((c) => !c.init)
      || pod?.containers[0];
    setContainer(first?.name || '');
  }, [pod, initialContainer]);

  const target: PodTarget | null = container ? { cluster, namespace, pod: name, container } : null;
  const { logs, clear: clearLogs } = usePodLogs(target);
  const { current: currentStats, history: statsHistory, error: statsError } = usePodStats(target);

  const execPath = target
    ? `/ws/k8s/exec/${[cluster, namespace, name].map(encodeURIComponent).join('/')}?container=${encodeURIComponent(container)}`
    : null;

  return {
    tabs: TABS,
    activeTab,
    setActiveTab,
    pod,
    podError,
    statusBadge: pod ? STATUS_BADGES[pod.status] : 'default',
    containerOptions: (pod?.containers || []).map((c) => ({
      value: c.name,
      label: `${c.init ? 'init: ' : ''}${c.name} (${c.reason || c.state})`,
    })),
    container,
    setContainer,
    logs,
    clearLogs,
    currentStats,
    statsHistory,
    statsError,
    execPath,
    resource,
    resourceError,
    refreshResource,
    resourceTarget,
    saveYaml: resource?.actions.edit ? saveYaml : undefined,
  };
};
