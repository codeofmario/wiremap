import { ReactNode } from 'react';

export interface ListItemProps {
  children: ReactNode;
  onClick?: () => void;
  active?: boolean;
  className?: string;
}

export const useListItem = ({ active = false, className, ...rest }: ListItemProps) => {
  return {
    className: `${active ? 'list-item--active' : ''} ${className || ''}`.trim(),
    ...rest,
  };
};
