import cn from 'classnames';
import { useCanvas, CanvasProps } from './Canvas.vm';
import './Canvas.scss';

export const Canvas = (props: CanvasProps) => {
  const { className, svgRef, children } = useCanvas(props);

  return (
    <div className={cn('canvas', className)}>
      <svg ref={svgRef} className="canvas__svg" />
      {children}
    </div>
  );
};
