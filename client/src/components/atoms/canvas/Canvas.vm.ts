import { RefObject, ReactNode } from 'react';

export interface CanvasProps {
  svgRef: RefObject<SVGSVGElement | null>;
  className?: string;
  children?: ReactNode;
}

export const useCanvas = ({ className, ...rest }: CanvasProps) => {
  return {
    className: className || '',
    ...rest,
  };
};
