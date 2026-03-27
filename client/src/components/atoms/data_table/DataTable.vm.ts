import { ReactNode } from 'react';

export interface DataTableColumn {
  key: string;
  label: string;
}

export interface DataTableProps {
  columns: Array<DataTableColumn>;
  rows: Array<Record<string, ReactNode>>;
  className?: string;
}

export const useDataTable = ({ className, ...rest }: DataTableProps) => {
  return {
    className: className || '',
    ...rest,
  };
};
