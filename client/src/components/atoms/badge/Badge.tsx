import cn from 'classnames';
import { useBadge, BadgeProps } from './Badge.vm';
import './Badge.scss';

export const Badge = (props: BadgeProps) => {
  const { className, label } = useBadge(props);

  return <span className={cn('badge', className)}>{label}</span>;
};
