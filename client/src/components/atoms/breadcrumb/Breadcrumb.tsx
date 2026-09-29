import { Fragment } from 'react';
import cn from 'classnames';
import { useBreadcrumb, BreadcrumbProps } from './Breadcrumb.vm';
import './Breadcrumb.scss';

export const Breadcrumb = (props: BreadcrumbProps) => {
  const { items, onNavigate, className } = useBreadcrumb(props);

  return (
    <nav className={cn('breadcrumb', className)} aria-label="Breadcrumb">
      {items.map((item) => (
        <Fragment key={item.id}>
          {item.index > 0 && <span className="breadcrumb__separator">›</span>}
          <button
            className={cn('breadcrumb__item', { 'breadcrumb__item--current': item.current })}
            onClick={() => onNavigate(item.index)}
            disabled={item.current}
            aria-current={item.current ? 'page' : undefined}
          >
            {item.label}
          </button>
        </Fragment>
      ))}
    </nav>
  );
};
