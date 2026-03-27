import cn from 'classnames';
import { useCollapsible, CollapsibleProps } from './Collapsible.vm';
import './Collapsible.scss';

export const Collapsible = (props: CollapsibleProps) => {
  const { open, toggle } = useCollapsible(props);
  const { title, children, badge, accentColor, className } = props;

  return (
    <div className={cn('collapsible', className)}>
      <button
        className="collapsible__header"
        onClick={toggle}
        style={accentColor ? { borderLeftColor: accentColor } : undefined}
      >
        <span className="collapsible__arrow">{open ? '\u25BE' : '\u25B8'}</span>
        <span className="collapsible__title">{title}</span>
        {badge && <span className="collapsible__badge">{badge}</span>}
      </button>
      {open && (
        <div className="collapsible__body">{children}</div>
      )}
    </div>
  );
};
