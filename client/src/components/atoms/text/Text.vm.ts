import { ReactNode } from 'react';

export interface TextProps {
  children: ReactNode;
  variant?: 'body' | 'label' | 'heading' | 'mono' | 'secondary';
  size?: 'xs' | 'sm' | 'md' | 'lg';
  as?: 'span' | 'p' | 'h1' | 'h2' | 'h3' | 'div';
  truncate?: boolean;
  onClick?: () => void;
  className?: string;
}

export const useText = (props: TextProps) => props;
