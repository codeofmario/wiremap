import { useLogLine, LogLineProps } from './LogLine.vm';
import { Stack } from '../../atoms/stack/Stack';
import { Text } from '../../atoms/text/Text';
import { Badge } from '../../atoms/badge/Badge';

const LEVEL_TO_BADGE_VARIANT: Record<string, 'default' | 'success' | 'warning' | 'error' | 'info'> = {
  error: 'error',
  warn: 'warning',
  info: 'info',
  debug: 'default',
};

export const LogLine = (props: LogLineProps) => {
  const { timestamp, message, level, collapsed, toggle } = useLogLine(props);

  return (
    <Stack gap="none" overflow="hidden" onClick={toggle} padding="xs">
      <Stack direction="row" gap="sm" align="center" overflow="hidden">
        <Text variant="secondary" size="xs">{collapsed ? '\u25B8' : '\u25BE'}</Text>
        {timestamp && <Text variant="secondary" size="xs">{timestamp}</Text>}
        {level !== 'default' && (
          <Badge label={level} variant={LEVEL_TO_BADGE_VARIANT[level] || 'default'} />
        )}
        <Text variant="mono" size="sm" truncate>{message.split('\n')[0]}</Text>
      </Stack>
      {!collapsed && (
        <Stack padding="xs" overflow="auto">
          <Text variant="mono" size="xs">{message}</Text>
        </Stack>
      )}
    </Stack>
  );
};
