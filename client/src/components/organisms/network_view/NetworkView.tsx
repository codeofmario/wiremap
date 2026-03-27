import { useNetworkView, NetworkViewProps } from './NetworkView.vm';
import { Badge } from '../../atoms/badge/Badge';
import { Stack } from '../../atoms/stack/Stack';
import { Text } from '../../atoms/text/Text';
import { KeyValue } from '../../atoms/key_value/KeyValue';
import { ScrollArea } from '../../atoms/scroll_area/ScrollArea';
import { Panel } from '../../atoms/panel/Panel';

export const NetworkView = (props: NetworkViewProps) => {
  const { networks } = useNetworkView(props);

  return (
    <ScrollArea flex>
      <Stack gap="lg" padding="md">
        <Text variant="heading" size="md">Container Networks</Text>
        {networks.length === 0 ? (
          <Text variant="secondary" size="sm">Not connected to any network</Text>
        ) : (
          <Stack gap="md">
            {networks.map((net) => (
              <Panel key={net.name} variant="bordered" padding="md">
                <Stack gap="sm">
                  <Badge label={net.name} variant="info" />
                  <KeyValue items={[
                    { label: 'IP Address', value: net.ip },
                    { label: 'Gateway', value: net.gateway },
                    { label: 'MAC', value: net.mac },
                    ...(net.aliases.length > 0 ? [{ label: 'DNS Aliases', value: net.aliases.join(', ') }] : []),
                  ]} />
                </Stack>
              </Panel>
            ))}
          </Stack>
        )}
      </Stack>
    </ScrollArea>
  );
};
