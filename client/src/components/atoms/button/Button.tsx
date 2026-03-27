import cn from 'classnames';
import { useButton, ButtonProps } from './Button.vm';
import './Button.scss';

export const Button = (props: ButtonProps) => {
  const { className, children, ...rest } = useButton(props);

  return (
    <button className={cn('button', className)} {...rest}>
      {children}
    </button>
  );
};
