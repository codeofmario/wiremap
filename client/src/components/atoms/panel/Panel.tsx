import cn from 'classnames';
import { usePanel, PanelProps } from './Panel.vm';
import './Panel.scss';

export const Panel = (props: PanelProps) => {
  const { children, variant = 'default', padding = 'none', fullHeight, overflow = 'hidden', className } = usePanel(props);

  return (
    <div className={cn(
      'panel',
      `panel--${variant}`,
      `panel--pad-${padding}`,
      `panel--overflow-${overflow}`,
      { 'panel--full-height': fullHeight },
      className,
    )}>
      {children}
    </div>
  );
};
