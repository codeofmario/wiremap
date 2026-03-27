import cn from 'classnames';
import { useToggle, ToggleProps } from './Toggle.vm';
import './Toggle.scss';

export const Toggle = (props: ToggleProps) => {
  const { options, value, onChange, className } = useToggle(props);

  return (
    <div className={cn('toggle', className)}>
      {options.map((opt) => (
        <button
          key={opt.id}
          className={cn('toggle__option', { 'toggle__option--active': opt.id === value })}
          onClick={() => onChange(opt.id)}
        >
          {opt.label}
        </button>
      ))}
    </div>
  );
};
