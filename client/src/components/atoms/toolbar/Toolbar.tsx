import cn from 'classnames';
import { useToolbar, ToolbarProps } from './Toolbar.vm';
import './Toolbar.scss';

export const Toolbar = (props: ToolbarProps) => {
  const { children, justify = 'end', bordered = true, className } = useToolbar(props);

  return (
    <div className={cn('toolbar', `toolbar--justify-${justify}`, { 'toolbar--bordered': bordered }, className)}>
      {children}
    </div>
  );
};
