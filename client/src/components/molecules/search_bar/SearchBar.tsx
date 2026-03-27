import { useSearchBar, SearchBarProps } from './SearchBar.vm';

export const SearchBar = (props: SearchBarProps) => {
  const { value, onChange, placeholder, className } = useSearchBar(props);

  return (
    <input
      className={className}
      type="text"
      value={value}
      onChange={(e) => onChange(e.target.value)}
      placeholder={placeholder}
    />
  );
};
