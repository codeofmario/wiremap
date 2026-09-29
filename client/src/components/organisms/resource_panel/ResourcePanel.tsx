import { useResourcePanel, ResourcePanelProps } from './ResourcePanel.vm';
import { Badge } from '../../atoms/badge/Badge';
import { Button } from '../../atoms/button/Button';
import { KeyValue } from '../../atoms/key_value/KeyValue';
import { Panel } from '../../atoms/panel/Panel';
import { ScrollArea } from '../../atoms/scroll_area/ScrollArea';
import { Stack } from '../../atoms/stack/Stack';
import { Tabs } from '../../atoms/tabs/Tabs';
import { Text } from '../../atoms/text/Text';
import { Toolbar } from '../../atoms/toolbar/Toolbar';
import { KubeEventsView } from '../kube_events_view/KubeEventsView';
import { ManifestView } from '../manifest_view/ManifestView';
import { ResourceActions } from '../resource_actions/ResourceActions';

export const ResourcePanel = (props: ResourcePanelProps) => {
  const {
    tabs, activeTab, setActiveTab, statusBadge, overview, resource, error, refresh, target, saveYaml,
  } = useResourcePanel(props);
  const { node } = props;

  return (
    <Panel variant="elevated" fullHeight>
      <Stack fullHeight>
        <Toolbar justify="between">
          <Stack direction="row" gap="sm" align="center" overflow="hidden">
            <Badge label={node.kind} variant="info" />
            <Text variant="heading" size="md" truncate>{node.name}</Text>
            <Badge label={node.status} variant={statusBadge} />
          </Stack>
          <Stack direction="row" gap="xs">
            {node.drillable && (
              <Button variant="primary" size="sm" onClick={() => props.onDrill(node)}>Open</Button>
            )}
            {props.onToggleExpand && (
              <Button variant="ghost" size="sm" onClick={props.onToggleExpand}>
                {props.expanded ? 'Collapse' : 'Expand'}
              </Button>
            )}
            <Button variant="ghost" size="sm" onClick={props.onClose}>Close</Button>
          </Stack>
        </Toolbar>
        {resource && (
          <ResourceActions target={target} actions={resource.actions} onChanged={refresh} onDeleted={props.onClose} />
        )}
        <Tabs tabs={tabs} activeTab={activeTab} onTabChange={setActiveTab} />
        <Stack flex="1" overflow="hidden">
          {activeTab === 'overview' && (
            <ScrollArea flex>
              <Stack padding="md">
                <KeyValue items={overview} />
              </Stack>
            </ScrollArea>
          )}
          {activeTab === 'yaml' && <ManifestView yaml={resource?.yaml ?? null} error={error} onSave={saveYaml} />}
          {activeTab === 'events' && <KubeEventsView events={resource?.events ?? null} error={error} />}
        </Stack>
      </Stack>
    </Panel>
  );
};
