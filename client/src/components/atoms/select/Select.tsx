import cn from 'classnames';
import { useSelect, SelectProps } from './Select.vm';
import './Select.scss';

export const Select = (props: SelectProps) => {
  const { options, value, onChange, className } = useSelect(props);

  return (
    <select
      className={cn('select', className)}
      value={value}
      onChange={(e) => onChange(e.target.value)}
    >
      {options.map((opt) => (
        <option key={opt.value} value={opt.value}>{opt.label}</option>
      ))}
    </select>
  );
};
