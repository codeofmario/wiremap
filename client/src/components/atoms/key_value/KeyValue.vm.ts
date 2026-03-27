import { ReactNode } from 'react';

export interface KeyValueItem {
  label: string;
  value: ReactNode;
}

export interface KeyValueProps {
  items: Array<KeyValueItem>;
  className?: string;
}

export const useKeyValue = ({ className, ...rest }: KeyValueProps) => {
  return {
    className: className || '',
    ...rest,
  };
};
