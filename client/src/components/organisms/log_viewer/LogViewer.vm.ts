import { useRef, useCallback, useState } from 'react';
import { LogEntry } from '../../../types/docker';

export interface LogViewerProps {
  logs: LogEntry[];
  onClear: () => void;
  className?: string;
}

export const useLogViewer = ({ logs, onClear }: LogViewerProps) => {
  const bottomRef = useRef<HTMLDivElement>(null);
  const [autoScroll, setAutoScroll] = useState(true);
  const [wordWrap, setWordWrap] = useState(true);

  const scrollToBottom = useCallback(() => {
    if (autoScroll) {
      bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
    }
  }, [autoScroll]);

  const toggleAutoScroll = () => setAutoScroll((s) => !s);
  const toggleWordWrap = () => setWordWrap((s) => !s);

  return { logs, onClear, bottomRef, scrollToBottom, autoScroll, toggleAutoScroll, wordWrap, toggleWordWrap };
};
