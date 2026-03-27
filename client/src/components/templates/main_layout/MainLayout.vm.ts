import { ReactNode } from 'react';

export interface MainLayoutProps {
  children: ReactNode;
  className?: string;
}

export const useMainLayout = (props: MainLayoutProps) => {
  return props;
};
