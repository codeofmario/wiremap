import cn from 'classnames';
import { useListItem, ListItemProps } from './ListItem.vm';
import './ListItem.scss';

export const ListItem = (props: ListItemProps) => {
  const { className, children, onClick } = useListItem(props);

  return (
    <div className={cn('list-item', className)} onClick={onClick}>
      {children}
    </div>
  );
};
