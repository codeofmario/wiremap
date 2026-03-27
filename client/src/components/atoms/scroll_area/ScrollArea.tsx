import cn from 'classnames';
import { useScrollArea, ScrollAreaProps } from './ScrollArea.vm';
import './ScrollArea.scss';

export const ScrollArea = (props: ScrollAreaProps) => {
  const { children, flex, className } = useScrollArea(props);

  return (
    <div className={cn('scroll-area', { 'scroll-area--flex': flex }, className)}>
      {children}
    </div>
  );
};
