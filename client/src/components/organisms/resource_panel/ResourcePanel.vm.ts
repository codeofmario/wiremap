import { useState, useEffect, useMemo } from 'react';
import { useKubeResource } from '../../../hooks/useKubeResource';
import { kubeActions, KubeTarget } from '../../../services/api';
import { KubeGraphNode, PanelRequest } from '../../../types/kubernetes';
import { Tab } from '../../atoms/tabs/Tabs.vm';

export interface ResourcePanelProps {
  cluster: string;
  node: KubeGraphNode;
  onClose: () => void;
  onDrill: (node: KubeGraphNode) => void;
  /** Hotkey asking for a tab; logs and shell don't apply here */
  request?: PanelRequest | null;
  expanded?: boolean;
  onToggleExpand?: () => void;
}

const TABS: Tab[] = [
  { id: 'overview', label: 'Overview' },
  { id: 'yaml', label: 'YAML' },
  { id: 'events', label: 'Events' },
];

const STATUS_BADGES = { healthy: 'success', warning: 'warning', error: 'error', idle: 'default' } as const;

const REQUEST_TABS: Partial<Record<PanelRequest['action'], string>> = { yaml: 'yaml', describe: 'overview' };

export const useResourcePanel = ({ cluster, node, request }: ResourcePanelProps) => {
  const [activeTab, setActiveTab] = useState('overview');

  useEffect(() => {
    const tab = request && REQUEST_TABS[request.action];
    if (tab) setActiveTab(tab);
  }, [request?.seq]);
  const { resource, error, refresh } = useKubeResource(cluster, node.kind, node.apiGroup || '', node.name, node.namespace);
  const target: KubeTarget = useMemo(
    () => ({ cluster, kind: node.kind, group: node.apiGroup || '', name: node.name, namespace: node.namespace }),
    [cluster, node.kind, node.apiGroup, node.name, node.namespace],
  );

  const saveYaml = async (yaml: string) => {
    await kubeActions.update(target, yaml);
    refresh();
  };

  return {
    tabs: TABS,
    activeTab,
    setActiveTab,
    statusBadge: STATUS_BADGES[node.status],
    overview: [
      { label: 'Kind', value: node.apiGroup ? `${node.kind} (${node.apiGroup})` : node.kind },
      ...(node.namespace ? [{ label: 'Namespace', value: node.namespace }] : []),
      { label: 'Status', value: node.status },
      ...(node.summary ? [{ label: 'Summary', value: node.summary }] : []),
      ...Object.entries(node.details || {}).filter(([, v]) => v).map(([label, value]) => ({ label, value })),
    ],
    resource,
    error,
    refresh,
    target,
    saveYaml: resource?.actions.edit ? saveYaml : undefined,
  };
};
