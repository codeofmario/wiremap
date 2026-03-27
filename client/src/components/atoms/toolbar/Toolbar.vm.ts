import { ReactNode } from 'react';

export interface ToolbarProps {
  children: ReactNode;
  justify?: 'start' | 'end' | 'between' | 'center';
  bordered?: boolean;
  className?: string;
}

export const useToolbar = (props: ToolbarProps) => props;
