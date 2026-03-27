import { useContainerList, ContainerListProps, containerDisplayName, STATE_FILTER_OPTIONS } from './ContainerList.vm';
import { Stack } from '../../atoms/stack/Stack';
import { ScrollArea } from '../../atoms/scroll_area/ScrollArea';
import { Collapsible } from '../../atoms/collapsible/Collapsible';
import { ListItem } from '../../atoms/list_item/ListItem';
import { Text } from '../../atoms/text/Text';
import { Badge } from '../../atoms/badge/Badge';
import { StatusDot } from '../../atoms/status_dot/StatusDot';
import { Input } from '../../atoms/input/Input';
import { Select } from '../../atoms/select/Select';
import { Toolbar } from '../../atoms/toolbar/Toolbar';

export const ContainerList = (props: ContainerListProps) => {
  const { selectedContainerId, onSelectContainer } = props;
  const { groups, ungrouped, search, setSearch, stateFilter, setStateFilter } = useContainerList(props);

  return (
    <Stack gap="none" flex="1" overflow="hidden">
      <Toolbar>
        <Stack direction="row" gap="sm" align="center" flex="1">
          <Input
            value={search}
            onChange={setSearch}
            placeholder="Search containers..."
            size="sm"
          />
          <Select
            options={STATE_FILTER_OPTIONS}
            value={stateFilter}
            onChange={(v) => setStateFilter(v as 'all' | 'running' | 'exited')}
          />
        </Stack>
      </Toolbar>
      <ScrollArea flex>
        <Stack gap="none">
          {groups.map((group) => (
            <Collapsible
              key={group.id}
              accentColor={group.color}
              title={group.name}
              badge={
                <Stack direction="row" gap="xs">
                  <Badge label={group.driver} />
                  {group.subnet && <Badge label={group.subnet} variant="info" />}
                  <Badge label={`${group.containerCount}`} variant="default" />
                </Stack>
              }
            >
              {group.containers.map((c) => (
                <ListItem
                  key={c.id}
                  active={c.id === selectedContainerId}
                  onClick={() => onSelectContainer(c.id === selectedContainerId ? null : c.id)}
                >
                  <Stack direction="row" gap="sm" align="center">
                    <StatusDot state={c.state} />
                    <Text variant="body" size="sm">{containerDisplayName(c)}</Text>
                    <Text variant="secondary" size="xs" truncate>{c.image}</Text>
                    <Badge
                      label={c.state}
                      variant={c.state === 'running' ? 'success' : c.state === 'exited' ? 'error' : 'warning'}
                    />
                  </Stack>
                </ListItem>
              ))}
            </Collapsible>
          ))}

          {ungrouped.length > 0 && (
            <Collapsible
              title="No network"
              badge={<Badge label={`${ungrouped.length}`} />}
            >
              {ungrouped.map((c) => (
                <ListItem
                  key={c.id}
                  active={c.id === selectedContainerId}
                  onClick={() => onSelectContainer(c.id === selectedContainerId ? null : c.id)}
                >
                  <Stack direction="row" gap="sm" align="center">
                    <StatusDot state={c.state} />
                    <Text variant="body" size="sm">{containerDisplayName(c)}</Text>
                    <Text variant="secondary" size="xs" truncate>{c.image}</Text>
                    <Badge
                      label={c.state}
                      variant={c.state === 'running' ? 'success' : c.state === 'exited' ? 'error' : 'warning'}
                    />
                  </Stack>
                </ListItem>
              ))}
            </Collapsible>
          )}

          {groups.length === 0 && ungrouped.length === 0 && (
            <Stack align="center" justify="center" fullHeight>
              <Text variant="secondary" size="sm">No containers match the current filters</Text>
            </Stack>
          )}
        </Stack>
      </ScrollArea>
    </Stack>
  );
};
