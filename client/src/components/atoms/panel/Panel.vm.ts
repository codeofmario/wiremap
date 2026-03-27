import { ReactNode } from 'react';

export interface PanelProps {
  children: ReactNode;
  variant?: 'default' | 'bordered' | 'elevated';
  padding?: 'none' | 'sm' | 'md' | 'lg';
  fullHeight?: boolean;
  overflow?: 'hidden' | 'auto' | 'visible';
  className?: string;
}

export const usePanel = (props: PanelProps) => props;
