import cn from 'classnames';
import { useInput, InputProps } from './Input.vm';
import './Input.scss';

export const Input = (props: InputProps) => {
  const { value, onChange, placeholder, variant = 'default', size = 'md', autoFocus, onKeyDown, className } = useInput(props);

  return (
    <input
      className={cn('input', `input--${variant}`, `input--${size}`, className)}
      type="text"
      value={value}
      onChange={(e) => onChange(e.target.value)}
      onKeyDown={onKeyDown}
      placeholder={placeholder}
      autoFocus={autoFocus}
      spellCheck={false}
    />
  );
};
