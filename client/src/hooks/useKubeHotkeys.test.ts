// @vitest-environment jsdom
import { renderHook } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { KubeHotkeyHandlers, useKubeHotkeys } from './useKubeHotkeys';

const setup = () => {
  const handlers: KubeHotkeyHandlers = {
    command: vi.fn(), filter: vi.fn(), open: vi.fn(), escape: vi.fn(), back: vi.fn(), move: vi.fn(), panel: vi.fn(),
  };
  const hook = renderHook(() => useKubeHotkeys(handlers));
  return { handlers, ...hook };
};

const press = (key: string, target: EventTarget = document.body, init: KeyboardEventInit = {}) => {
  const event = new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true, ...init });
  target.dispatchEvent(event);
  return event;
};

describe('useKubeHotkeys', () => {
  afterEach(() => {
    document.body.innerHTML = '';
  });

  it('maps keys to handlers and swallows them', () => {
    const { handlers } = setup();

    expect(press(':').defaultPrevented).toBe(true);
    press('/');
    press('Enter');
    press('Escape');
    press('Backspace');
    press('ArrowDown');
    press('j');
    press('ArrowUp');
    press('k');

    expect(handlers.command).toHaveBeenCalledTimes(1);
    expect(handlers.filter).toHaveBeenCalledTimes(1);
    expect(handlers.open).toHaveBeenCalledTimes(1);
    expect(handlers.escape).toHaveBeenCalledTimes(1);
    expect(handlers.back).toHaveBeenCalledTimes(1);
    expect(handlers.move).toHaveBeenNthCalledWith(1, 1);
    expect(handlers.move).toHaveBeenNthCalledWith(2, 1);
    expect(handlers.move).toHaveBeenNthCalledWith(3, -1);
    expect(handlers.move).toHaveBeenNthCalledWith(4, -1);
  });

  it('asks the side panel for logs, shell, YAML and describe', () => {
    const { handlers } = setup();
    for (const key of ['l', 's', 'y', 'd']) press(key);
    expect(vi.mocked(handlers.panel).mock.calls.map(([action]) => action)).toEqual(['logs', 'shell', 'yaml', 'describe']);
  });

  it('leaves other keys alone', () => {
    const { handlers } = setup();
    const event = press('x');
    expect(event.defaultPrevented).toBe(false);
    expect(Object.values(handlers).every((h) => vi.mocked(h).mock.calls.length === 0)).toBe(true);
  });

  it('ignores keys while typing or with modifiers', () => {
    const { handlers } = setup();
    const input = document.body.appendChild(document.createElement('input'));
    const textarea = document.body.appendChild(document.createElement('textarea'));
    const editor = document.body.appendChild(document.createElement('div'));
    editor.contentEditable = 'true';
    // jsdom doesn't implement isContentEditable
    Object.defineProperty(editor, 'isContentEditable', { value: true });

    press('l', input);
    press('/', textarea);
    press('Escape', editor);
    press('l', document.body, { ctrlKey: true });
    press('y', document.body, { metaKey: true });
    press('d', document.body, { altKey: true });

    expect(handlers.panel).not.toHaveBeenCalled();
    expect(handlers.filter).not.toHaveBeenCalled();
    expect(handlers.escape).not.toHaveBeenCalled();
  });

  it('lets a focused button handle Enter but still takes other keys', () => {
    const { handlers } = setup();
    const button = document.body.appendChild(document.createElement('button'));

    expect(press('Enter', button).defaultPrevented).toBe(false);
    press('l', button);

    expect(handlers.open).not.toHaveBeenCalled();
    expect(handlers.panel).toHaveBeenCalledWith('logs');
  });

  it('stops listening on unmount', () => {
    const { handlers, unmount } = setup();
    unmount();
    press(':');
    expect(handlers.command).not.toHaveBeenCalled();
  });
});
