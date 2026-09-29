import { usePodPanel, PodPanelProps } from './PodPanel.vm';
import { Badge } from '../../atoms/badge/Badge';
import { Button } from '../../atoms/button/Button';
import { Panel } from '../../atoms/panel/Panel';
import { Select } from '../../atoms/select/Select';
import { Stack } from '../../atoms/stack/Stack';
import { Tabs } from '../../atoms/tabs/Tabs';
import { Text } from '../../atoms/text/Text';
import { Toolbar } from '../../atoms/toolbar/Toolbar';
import { ConsoleView } from '../console_view/ConsoleView';
import { KubeEventsView } from '../kube_events_view/KubeEventsView';
import { LogViewer } from '../log_viewer/LogViewer';
import { ManifestView } from '../manifest_view/ManifestView';
import { ResourceActions } from '../resource_actions/ResourceActions';
import { StatsChart } from '../stats_chart/StatsChart';

export const PodPanel = (props: PodPanelProps) => {
  const {
    tabs, activeTab, setActiveTab,
    pod, podError, statusBadge,
    containerOptions, container, setContainer,
    logs, clearLogs,
    currentStats, statsHistory, statsError,
    execPath,
    resource, resourceError, refreshResource, resourceTarget, saveYaml,
  } = usePodPanel(props);

  return (
    <Panel variant="elevated" fullHeight>
      <Stack fullHeight>
        <Toolbar justify="between">
          <Stack direction="row" gap="sm" align="center" overflow="hidden">
            <Badge label={props.container ? 'Container' : 'Pod'} variant="info" />
            <Text variant="heading" size="md" truncate>{props.container || props.name}</Text>
            {pod && <Badge label={pod.phase} variant={statusBadge} />}
          </Stack>
          <Stack direction="row" gap="xs">
            {props.onOpen && (
              <Button variant="primary" size="sm" onClick={props.onOpen}>Open</Button>
            )}
            {props.onToggleExpand && (
              <Button variant="ghost" size="sm" onClick={props.onToggleExpand}>
                {props.expanded ? 'Collapse' : 'Expand'}
              </Button>
            )}
            <Button variant="ghost" size="sm" onClick={props.onClose}>Close</Button>
          </Stack>
        </Toolbar>
        {pod && (
          <Toolbar justify="between">
            <Text variant="secondary" size="xs">{props.container && `pod ${props.name} · `}{props.namespace} · {pod.node || 'unscheduled'} · {pod.podIp || 'no IP'}</Text>
            {containerOptions.length > 1 && (
              <Select options={containerOptions} value={container} onChange={setContainer} />
            )}
          </Toolbar>
        )}
        {podError && (
          <Stack padding="md">
            <Text variant="secondary" size="sm">{podError}</Text>
          </Stack>
        )}
        {resource && (
          <ResourceActions target={resourceTarget} actions={resource.actions} onChanged={refreshResource} onDeleted={props.onClose} />
        )}
        <Tabs tabs={tabs} activeTab={activeTab} onTabChange={setActiveTab} />
        <Stack flex="1" overflow="hidden">
          {activeTab === 'logs' && <LogViewer logs={logs} onClear={clearLogs} />}
          {activeTab === 'metrics' && (statsError ? (
            <Stack padding="md">
              <Text variant="secondary" size="sm">{statsError}</Text>
            </Stack>
          ) : (
            <StatsChart current={currentStats} history={statsHistory} showNetwork={false} />
          ))}
          {activeTab === 'console' && execPath && <ConsoleView key={execPath} execPath={execPath} />}
          {activeTab === 'yaml' && <ManifestView yaml={resource?.yaml ?? null} error={resourceError} onSave={saveYaml} />}
          {activeTab === 'events' && <KubeEventsView events={resource?.events ?? null} error={resourceError} />}
        </Stack>
      </Stack>
    </Panel>
  );
};
