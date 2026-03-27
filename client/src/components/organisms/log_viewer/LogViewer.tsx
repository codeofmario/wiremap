import { useEffect } from 'react';
import { useLogViewer, LogViewerProps } from './LogViewer.vm';
import { LogLine } from '../../molecules/log_line/LogLine';
import { Button } from '../../atoms/button/Button';
import { Stack } from '../../atoms/stack/Stack';
import { Toolbar } from '../../atoms/toolbar/Toolbar';
import { ScrollArea } from '../../atoms/scroll_area/ScrollArea';
import { Text } from '../../atoms/text/Text';

export const LogViewer = (props: LogViewerProps) => {
  const { logs, onClear, bottomRef, scrollToBottom, autoScroll, toggleAutoScroll, wordWrap, toggleWordWrap } = useLogViewer(props);

  useEffect(() => {
    scrollToBottom();
  }, [logs.length, scrollToBottom]);

  return (
    <Stack fullHeight overflow="hidden">
      <Toolbar justify="between">
        <Text variant="secondary" size="sm">{logs.length} lines</Text>
        <Stack direction="row" gap="xs">
          <Button variant={wordWrap ? 'primary' : 'ghost'} size="sm" onClick={toggleWordWrap}>Wrap</Button>
          <Button variant={autoScroll ? 'primary' : 'ghost'} size="sm" onClick={toggleAutoScroll}>Auto-scroll</Button>
          <Button variant="ghost" size="sm" onClick={onClear}>Clear</Button>
        </Stack>
      </Toolbar>
      <ScrollArea flex>
        {logs.length === 0 && (
          <Text variant="secondary" size="sm">Waiting for logs...</Text>
        )}
        {logs.map((log, i) => (
          <LogLine key={i} text={log.text} type={log.type} />
        ))}
        <div ref={bottomRef} />
      </ScrollArea>
    </Stack>
  );
};
