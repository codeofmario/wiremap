import cn from 'classnames';
import { useText, TextProps } from './Text.vm';
import './Text.scss';

export const Text = (props: TextProps) => {
  const { children, variant = 'body', size = 'md', as: Tag = 'span', truncate, onClick, className } = useText(props);

  return (
    <Tag className={cn('text', `text--${variant}`, `text--${size}`, { 'text--truncate': truncate }, className)} onClick={onClick}>
      {children}
    </Tag>
  );
};
