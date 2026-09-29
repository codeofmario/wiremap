import { KeyboardEvent } from 'react';

export interface InputProps {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  variant?: 'default' | 'mono';
  size?: 'sm' | 'md';
  autoFocus?: boolean;
  /** Called on Enter */
  onSubmit?: () => void;
  /** Called on Escape */
  onCancel?: () => void;
  className?: string;
}

export const useInput = ({ onSubmit, onCancel, ...rest }: InputProps) => ({
  ...rest,
  onKeyDown: (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'Enter' && onSubmit) {
      event.preventDefault();
      onSubmit();
    } else if (event.key === 'Escape' && onCancel) {
      event.preventDefault();
      onCancel();
    }
  },
});
