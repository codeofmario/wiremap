import { useKubeEventsView, KubeEventsViewProps } from './KubeEventsView.vm';
import { Badge } from '../../atoms/badge/Badge';
import { DataTable } from '../../atoms/data_table/DataTable';
import { ScrollArea } from '../../atoms/scroll_area/ScrollArea';
import { Stack } from '../../atoms/stack/Stack';
import { Text } from '../../atoms/text/Text';

export const KubeEventsView = (props: KubeEventsViewProps) => {
  const { columns, events, loading, error } = useKubeEventsView(props);

  if (error || loading || events.length === 0) {
    return (
      <Stack padding="md">
        <Text variant="secondary" size="sm">{error || (loading ? 'Loading...' : 'No recent events')}</Text>
      </Stack>
    );
  }

  return (
    <ScrollArea flex>
      <Stack padding="md">
        <DataTable
          columns={columns}
          rows={events.map((e) => ({
            type: <Badge label={e.type} variant={e.type === 'Warning' ? 'warning' : 'default'} />,
            reason: <Text size="sm">{e.reason}</Text>,
            message: <Text variant="mono" size="xs">{e.message}</Text>,
            count: <Text size="sm">{e.count || 1}</Text>,
            lastSeen: <Text variant="secondary" size="xs">{e.age}</Text>,
          }))}
        />
      </Stack>
    </ScrollArea>
  );
};
