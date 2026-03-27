import { ButtonHTMLAttributes } from 'react';

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'secondary' | 'ghost';
  size?: 'sm' | 'md';
}

export const useButton = ({ variant = 'secondary', size = 'md', className, ...rest }: ButtonProps) => {
  return {
    className: `button--${variant} button--${size} ${className || ''}`.trim(),
    ...rest,
  };
};
