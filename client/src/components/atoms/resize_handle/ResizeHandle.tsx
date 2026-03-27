import cn from 'classnames';
import { useResizeHandle, ResizeHandleProps } from './ResizeHandle.vm';
import './ResizeHandle.scss';

export const ResizeHandle = (props: ResizeHandleProps) => {
  const { onMouseDown } = useResizeHandle(props);

  return (
    <div className={cn('resize-handle', props.className)} onMouseDown={onMouseDown}>
      <div className="resize-handle__line" />
    </div>
  );
};
