export interface ToggleOption {
  id: string;
  label: string;
}

export interface ToggleProps {
  options: ToggleOption[];
  value: string;
  onChange: (value: string) => void;
  className?: string;
}

export const useToggle = (props: ToggleProps) => props;
