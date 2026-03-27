export interface SearchBarProps {
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
  className?: string;
}

export const useSearchBar = (props: SearchBarProps) => {
  return {
    ...props,
    placeholder: props.placeholder || 'Search...',
  };
};
