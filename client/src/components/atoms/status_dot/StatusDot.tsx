import cn from 'classnames';
import { useStatusDot, StatusDotProps } from './StatusDot.vm';
import './StatusDot.scss';

export const StatusDot = (props: StatusDotProps) => {
  const { className } = useStatusDot(props);

  return <span className={cn('status-dot', className)} />;
};
