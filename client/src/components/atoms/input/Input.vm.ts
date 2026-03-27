export interface InputProps {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  variant?: 'default' | 'mono';
  size?: 'sm' | 'md';
  className?: string;
}

export const useInput = (props: InputProps) => props;
