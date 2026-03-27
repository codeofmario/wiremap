import { useStatCard, StatCardProps } from './StatCard.vm';
import { Stack } from '../../atoms/stack/Stack';
import { Text } from '../../atoms/text/Text';
import { Panel } from '../../atoms/panel/Panel';

export const StatCard = (props: StatCardProps) => {
  const { label, value, unit, className } = useStatCard(props);

  return (
    <Panel variant="bordered" padding="md" className={className}>
      <Stack gap="xs">
        <Text variant="secondary" size="sm">{label}</Text>
        <Stack direction="row" gap="xs" align="baseline">
          <Text variant="heading" size="lg">{value}</Text>
          {unit && <Text variant="secondary" size="xs">{unit}</Text>}
        </Stack>
      </Stack>
    </Panel>
  );
};
