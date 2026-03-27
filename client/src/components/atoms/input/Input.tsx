import cn from 'classnames';
import { useInput, InputProps } from './Input.vm';
import './Input.scss';

export const Input = (props: InputProps) => {
  const { value, onChange, placeholder, variant = 'default', size = 'md', className } = useInput(props);

  return (
    <input
      className={cn('input', `input--${variant}`, `input--${size}`, className)}
      type="text"
      value={value}
      onChange={(e) => onChange(e.target.value)}
      placeholder={placeholder}
    />
  );
};
