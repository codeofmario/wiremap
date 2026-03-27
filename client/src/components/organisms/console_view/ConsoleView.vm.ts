import { useRef, useEffect } from 'react';
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';

export interface ConsoleViewProps {
  host: string;
  containerId: string;
  className?: string;
}

export const useConsoleView = ({ host, containerId }: ConsoleViewProps) => {
  const termRef = useRef<HTMLDivElement>(null);
  const terminalRef = useRef<Terminal | null>(null);
  const wsRef = useRef<WebSocket | null>(null);
  const fitRef = useRef<FitAddon | null>(null);

  useEffect(() => {
    const el = termRef.current;
    if (!el) return;

    const init = () => {
      if (el.clientWidth === 0 || el.clientHeight === 0) {
        requestAnimationFrame(init);
        return;
      }

      const terminal = new Terminal({
        cursorBlink: true,
        fontSize: 13,
        fontFamily: "'JetBrains Mono', 'Fira Code', 'Consolas', monospace",
        theme: {
          background: '#0f1117',
          foreground: '#e1e4ed',
          cursor: '#6366f1',
          selectionBackground: '#6366f133',
          black: '#0f1117',
          red: '#ef4444',
          green: '#22c55e',
          yellow: '#f59e0b',
          blue: '#3b82f6',
          magenta: '#ec4899',
          cyan: '#06b6d4',
          white: '#e1e4ed',
        },
      });

      const fit = new FitAddon();
      terminal.loadAddon(fit);
      terminal.open(el);
      fit.fit();

      terminalRef.current = terminal;
      fitRef.current = fit;

      const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
      const hostParam = host ? `?host=${encodeURIComponent(host)}` : '';
      const ws = new WebSocket(`${protocol}//${location.host}/ws/exec/${containerId}${hostParam}`);
      ws.binaryType = 'arraybuffer';
      wsRef.current = ws;

      ws.onopen = () => {
        terminal.writeln('\x1b[32mConnected to container shell\x1b[0m\r\n');
        fit.fit();
      };

      ws.onmessage = (event) => {
        const data = event.data instanceof ArrayBuffer
          ? new TextDecoder().decode(event.data)
          : event.data;
        terminal.write(data);
      };

      ws.onclose = () => {
        terminal.writeln('\r\n\x1b[31mDisconnected\x1b[0m');
      };

      ws.onerror = () => {
        terminal.writeln('\r\n\x1b[31mConnection error\x1b[0m');
      };

      terminal.onData((data) => {
        if (ws.readyState === WebSocket.OPEN) {
          ws.send(new TextEncoder().encode(data));
        }
      });

      terminal.onResize(({ cols, rows }) => {
        if (ws.readyState === WebSocket.OPEN) {
          ws.send(JSON.stringify({ event: 'resize', cols, rows }));
        }
      });
    };

    requestAnimationFrame(init);

    const handleResize = () => {
      fitRef.current?.fit();
    };
    window.addEventListener('resize', handleResize);

    return () => {
      window.removeEventListener('resize', handleResize);
      wsRef.current?.close();
      wsRef.current = null;
      terminalRef.current?.dispose();
      terminalRef.current = null;
    };
  }, [containerId]);

  return { termRef };
};
