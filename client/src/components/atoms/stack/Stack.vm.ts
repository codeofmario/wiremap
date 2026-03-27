import { ReactNode, CSSProperties, MouseEventHandler } from 'react';

export interface StackProps {
  children?: ReactNode;
  gap?: 'none' | 'xs' | 'sm' | 'md' | 'lg' | 'xl';
  direction?: 'column' | 'row';
  align?: CSSProperties['alignItems'];
  justify?: CSSProperties['justifyContent'];
  flex?: CSSProperties['flex'];
  padding?: 'none' | 'xs' | 'sm' | 'md' | 'lg';
  fullHeight?: boolean;
  overflow?: 'hidden' | 'auto';
  className?: string;
  onClick?: MouseEventHandler<HTMLDivElement>;
}

export const useStack = (props: StackProps) => props;
