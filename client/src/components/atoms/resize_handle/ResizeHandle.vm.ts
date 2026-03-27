import { useCallback, useRef } from 'react';

export interface ResizeHandleProps {
  onResize: (delta: number) => void;
  onResizeEnd?: () => void;
  className?: string;
}

export const useResizeHandle = ({ onResize, onResizeEnd }: ResizeHandleProps) => {
  const startXRef = useRef(0);

  const onMouseDown = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    startXRef.current = e.clientX;

    const onMouseMove = (ev: MouseEvent) => {
      const delta = startXRef.current - ev.clientX;
      startXRef.current = ev.clientX;
      onResize(delta);
    };

    const onMouseUp = () => {
      document.removeEventListener('mousemove', onMouseMove);
      document.removeEventListener('mouseup', onMouseUp);
      document.body.style.cursor = '';
      document.body.style.userSelect = '';
      onResizeEnd?.();
    };

    document.body.style.cursor = 'col-resize';
    document.body.style.userSelect = 'none';
    document.addEventListener('mousemove', onMouseMove);
    document.addEventListener('mouseup', onMouseUp);
  }, [onResize, onResizeEnd]);

  return { onMouseDown };
};
