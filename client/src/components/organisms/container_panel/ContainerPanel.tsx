import { useContainerPanel, ContainerPanelProps } from './ContainerPanel.vm';
import { Tabs } from '../../atoms/tabs/Tabs';
import { Button } from '../../atoms/button/Button';
import { Stack } from '../../atoms/stack/Stack';
import { Panel } from '../../atoms/panel/Panel';
import { Toolbar } from '../../atoms/toolbar/Toolbar';
import { Text } from '../../atoms/text/Text';
import { LogViewer } from '../log_viewer/LogViewer';
import { StatsChart } from '../stats_chart/StatsChart';
import { InspectView } from '../inspect_view/InspectView';
import { PortsVolumesView } from '../ports_volumes_view/PortsVolumesView';
import { NetworkView } from '../network_view/NetworkView';
import { ConsoleView } from '../console_view/ConsoleView';
import { VolumeBrowser } from '../volume_browser/VolumeBrowser';

export const ContainerPanel = (props: ContainerPanelProps) => {
  const {
    tabs, activeTab, setActiveTab,
    logs, clearLogs,
    currentStats, statsHistory,
    inspect, inspectLoading,
    containerName,
  } = useContainerPanel(props);

  return (
    <Panel variant="elevated" fullHeight>
      <Stack fullHeight>
        <Toolbar justify="between">
          <Text variant="heading" size="md">{containerName}</Text>
          <Stack direction="row" gap="xs">
            {props.onToggleExpand && (
              <Button variant="ghost" size="sm" onClick={props.onToggleExpand}>
                {props.expanded ? 'Collapse' : 'Expand'}
              </Button>
            )}
            <Button variant="ghost" size="sm" onClick={props.onClose}>Close</Button>
          </Stack>
        </Toolbar>
        <Tabs tabs={tabs} activeTab={activeTab} onTabChange={setActiveTab} />
        <Stack flex="1" overflow="hidden">
          {activeTab === 'logs' && (
            <LogViewer logs={logs} onClear={clearLogs} />
          )}
          {activeTab === 'stats' && (
            <StatsChart current={currentStats} history={statsHistory} />
          )}
          {activeTab === 'console' && (
            <ConsoleView host={props.host} containerId={props.containerId} />
          )}
          {activeTab === 'inspect' && inspect && !inspectLoading && (
            <InspectView inspect={inspect} host={props.host} containerId={props.containerId} onContainerIdChange={props.onContainerIdChange} />
          )}
          {activeTab === 'volumes' && inspect && !inspectLoading && (
            <VolumeBrowser inspect={inspect} host={props.host} containerId={props.containerId} />
          )}
          {activeTab === 'ports' && inspect && !inspectLoading && (
            <PortsVolumesView inspect={inspect} />
          )}
          {activeTab === 'network' && inspect && !inspectLoading && (
            <NetworkView inspect={inspect} />
          )}
          {(activeTab !== 'logs' && activeTab !== 'stats' && activeTab !== 'console') && inspectLoading && (
            <Text variant="secondary" size="sm">Loading...</Text>
          )}
        </Stack>
      </Stack>
    </Panel>
  );
};
