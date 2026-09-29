import { useState, useCallback } from 'react';

const PANEL_WIDTH_KEY = 'wiremap:panelWidth';
const DEFAULT_PANEL_WIDTH = 480;
const MIN_PANEL_WIDTH = 320;
const MAX_PANEL_WIDTH = 1200;

const loadPanelWidth = (): number => {
  const stored = localStorage.getItem(PANEL_WIDTH_KEY);
  if (stored) {
    const n = parseInt(stored, 10);
    if (!isNaN(n) && n >= MIN_PANEL_WIDTH && n <= MAX_PANEL_WIDTH) return n;
  }
  return DEFAULT_PANEL_WIDTH;
};

/** Width and expand state of the right-hand detail panel, with the width persisted. */
export const useResizablePanel = () => {
  const [panelExpanded, setPanelExpanded] = useState(false);
  const [panelWidth, setPanelWidth] = useState(loadPanelWidth);

  const toggleExpand = useCallback(() => setPanelExpanded((s) => !s), []);

  const handlePanelResize = useCallback((delta: number) => {
    setPanelWidth((w) => Math.max(MIN_PANEL_WIDTH, Math.min(MAX_PANEL_WIDTH, w + delta)));
  }, []);

  const handlePanelResizeEnd = useCallback(() => {
    setPanelWidth((w) => {
      localStorage.setItem(PANEL_WIDTH_KEY, String(w));
      return w;
    });
  }, []);

  return { panelExpanded, setPanelExpanded, panelWidth, toggleExpand, handlePanelResize, handlePanelResizeEnd };
};
