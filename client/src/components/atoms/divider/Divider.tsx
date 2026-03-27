import cn from 'classnames';
import { useDivider, DividerProps } from './Divider.vm';
import './Divider.scss';

export const Divider = (props: DividerProps) => {
  const { className } = useDivider(props);
  return <hr className={cn('divider', className)} />;
};
