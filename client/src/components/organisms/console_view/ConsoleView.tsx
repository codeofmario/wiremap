import { useConsoleView, ConsoleViewProps } from './ConsoleView.vm';
import { Stack } from '../../atoms/stack/Stack';
import '@xterm/xterm/css/xterm.css';

export const ConsoleView = (props: ConsoleViewProps) => {
  const { termRef } = useConsoleView(props);

  return (
    <Stack fullHeight padding="sm">
      <Stack ref={termRef} flex="1" />
    </Stack>
  );
};
