import { ReactNode } from 'react';

export interface ScrollAreaProps {
  children: ReactNode;
  flex?: boolean;
  className?: string;
}

export const useScrollArea = (props: ScrollAreaProps) => props;
