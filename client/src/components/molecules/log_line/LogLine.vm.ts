import { useState, useMemo } from 'react';

export interface LogLineProps {
  text: string;
  type: 'stdout' | 'stderr';
  className?: string;
}

const LEVEL_PATTERNS: Array<{ pattern: RegExp; level: string }> = [
  { pattern: /\b(ERROR|ERR|FATAL|PANIC|CRIT)\b/i, level: 'error' },
  { pattern: /\b(WARN|WARNING)\b/i, level: 'warn' },
  { pattern: /\b(INFO)\b/i, level: 'info' },
  { pattern: /\b(DEBUG|TRACE)\b/i, level: 'debug' },
];

const TIMESTAMP_PATTERN = /^(\d{4}-\d{2}-\d{2}T[\d:.]+Z?\s*)/;

export const useLogLine = ({ text, type, className }: LogLineProps) => {
  const [collapsed, setCollapsed] = useState(true);

  const { timestamp, message, level, isMultiLine } = useMemo(() => {
    const tsMatch = text.match(TIMESTAMP_PATTERN);
    const timestamp = tsMatch ? tsMatch[1].trim() : '';
    const message = tsMatch ? text.slice(tsMatch[1].length) : text;

    let level = type === 'stderr' ? 'error' : 'default';
    for (const { pattern, level: lvl } of LEVEL_PATTERNS) {
      if (pattern.test(message)) {
        level = lvl;
        break;
      }
    }

    const isMultiLine = message.includes('\n') && message.trim().split('\n').length > 1;

    return { timestamp, message, level, isMultiLine };
  }, [text, type]);

  const toggle = () => setCollapsed((s) => !s);

  const displayTimestamp = timestamp
    ? new Date(timestamp).toLocaleTimeString('en-US', { hour12: false } as Intl.DateTimeFormatOptions)
    : '';

  const displayMessage = collapsed && isMultiLine ? message.split('\n')[0] : message;

  return {
    timestamp: displayTimestamp,
    message: displayMessage,
    level,
    isMultiLine,
    collapsed,
    truncate: collapsed,
    toggle,
  };
};
