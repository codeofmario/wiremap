import { useEffect, useRef } from 'react';
import { PanelRequest } from '../types/kubernetes';

export interface KubeHotkeyHandlers {
  command: () => void;
  filter: () => void;
  open: () => void;
  escape: () => void;
  back: () => void;
  move: (delta: 1 | -1) => void;
  panel: (action: PanelRequest['action']) => void;
}

const PANEL_KEYS: Record<string, PanelRequest['action']> = { l: 'logs', s: 'shell', y: 'yaml', d: 'describe' };

// Editors, the shell and form fields keep their keys
const isTyping = (target: EventTarget | null) =>
  target instanceof HTMLElement && (target.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName));

/** Keyboard shortcuts for the cluster view. */
export const useKubeHotkeys = (handlers: KubeHotkeyHandlers) => {
  const handlersRef = useRef(handlers);
  handlersRef.current = handlers;

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.ctrlKey || event.metaKey || event.altKey || isTyping(event.target)) return;
      // A focused button handles its own Enter
      if (event.key === 'Enter' && event.target instanceof HTMLButtonElement) return;

      const h = handlersRef.current;
      switch (event.key) {
        case ':': h.command(); break;
        case '/': h.filter(); break;
        case 'Enter': h.open(); break;
        case 'Escape': h.escape(); break;
        case 'Backspace': h.back(); break;
        case 'ArrowDown': case 'j': h.move(1); break;
        case 'ArrowUp': case 'k': h.move(-1); break;
        default:
          if (!(event.key in PANEL_KEYS)) return;
          h.panel(PANEL_KEYS[event.key]);
      }
      event.preventDefault();
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, []);
};
