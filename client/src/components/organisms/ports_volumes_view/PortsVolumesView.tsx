import { usePortsVolumesView, PortsVolumesViewProps } from './PortsVolumesView.vm';
import { Badge } from '../../atoms/badge/Badge';
import { DataTable } from '../../atoms/data_table/DataTable';
import { ScrollArea } from '../../atoms/scroll_area/ScrollArea';
import { Stack } from '../../atoms/stack/Stack';
import { Text } from '../../atoms/text/Text';

export const PortsVolumesView = (props: PortsVolumesViewProps) => {
  const { ports, volumes } = usePortsVolumesView(props);

  return (
    <ScrollArea flex>
      <Stack gap="lg" padding="md">
        <Stack gap="sm">
          <Text variant="label" size="sm">Port Mappings</Text>
          {ports.length === 0 ? (
            <Text variant="secondary" size="sm">No port mappings</Text>
          ) : (
            <DataTable
              columns={[
                { key: 'host', label: 'Host' },
                { key: 'container', label: 'Container' },
                { key: 'protocol', label: 'Protocol' },
              ]}
              rows={ports.map((p) => ({
                host: p.host,
                container: p.container,
                protocol: <Badge label={p.protocol} />,
              }))}
            />
          )}
        </Stack>

        <Stack gap="sm">
          <Text variant="label" size="sm">Volumes &amp; Mounts</Text>
          {volumes.length === 0 ? (
            <Text variant="secondary" size="sm">No volumes</Text>
          ) : (
            <DataTable
              columns={[
                { key: 'type', label: 'Type' },
                { key: 'source', label: 'Source' },
                { key: 'destination', label: 'Destination' },
                { key: 'mode', label: 'Mode' },
              ]}
              rows={volumes.map((v) => ({
                type: <Badge label={v.type} variant="info" />,
                source: <Text variant="mono" size="xs">{v.source}</Text>,
                destination: <Text variant="mono" size="xs">{v.destination}</Text>,
                mode: (
                  <Badge
                    label={v.readOnly ? 'RO' : 'RW'}
                    variant={v.readOnly ? 'warning' : 'success'}
                  />
                ),
              }))}
            />
          )}
        </Stack>
      </Stack>
    </ScrollArea>
  );
};
