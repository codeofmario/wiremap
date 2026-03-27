import { forwardRef } from 'react';
import cn from 'classnames';
import { useStack, StackProps } from './Stack.vm';
import './Stack.scss';

export const Stack = forwardRef<HTMLDivElement, StackProps>((props, ref) => {
  const { children, gap = 'md', direction = 'column', align, justify, flex, padding = 'none', fullHeight, overflow, className, onClick } = useStack(props);

  return (
    <div ref={ref} className={cn(
      'stack',
      `stack--gap-${gap}`,
      `stack--dir-${direction}`,
      `stack--pad-${padding}`,
      { 'stack--full-height': fullHeight, 'stack--clickable': !!onClick },
      overflow && `stack--overflow-${overflow}`,
      className,
    )} style={{ alignItems: align, justifyContent: justify, flex, minHeight: flex ? 0 : undefined }} onClick={onClick}>
      {children}
    </div>
  );
});
