import { useKubeObjectList, KubeObjectListProps } from './KubeObjectList.vm';
import { Button } from '../../atoms/button/Button';
import { ListItem } from '../../atoms/list_item/ListItem';
import { ScrollArea } from '../../atoms/scroll_area/ScrollArea';
import { Stack } from '../../atoms/stack/Stack';
import { StatusDot } from '../../atoms/status_dot/StatusDot';
import { Text } from '../../atoms/text/Text';

export const KubeObjectList = (props: KubeObjectListProps) => {
  const { rows, hasMore, loadMore, loadingMore } = useKubeObjectList(props);

  return (
    <ScrollArea flex>
      <Stack gap="none" padding="sm">
        {rows.map((row) => (
          <ListItem key={row.node.id} active={row.active} onClick={row.toggle}>
            <Stack direction="row" gap="sm" align="center" overflow="hidden">
              <StatusDot state={row.dot} />
              <Stack gap="none" flex="1" overflow="hidden">
                <Text size="sm" truncate>{row.node.name}</Text>
                <Text variant="secondary" size="xs" truncate>{row.subtitle}</Text>
              </Stack>
            </Stack>
          </ListItem>
        ))}
        {hasMore && (
          <Stack padding="sm" align="center">
            <Button variant="secondary" size="sm" onClick={loadMore} disabled={loadingMore}>
              {loadingMore ? 'Loading...' : 'Load more'}
            </Button>
          </Stack>
        )}
      </Stack>
    </ScrollArea>
  );
};
