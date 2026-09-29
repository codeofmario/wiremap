import cn from 'classnames';
import { useKeyHint, KeyHintProps } from './KeyHint.vm';
import './KeyHint.scss';

export const KeyHint = (props: KeyHintProps) => {
  const { keys, label, className } = useKeyHint(props);

  return (
    <span className={cn('key-hint', className)}>
      <kbd className="key-hint__key">{keys}</kbd>
      <span className="key-hint__label">{label}</span>
    </span>
  );
};
