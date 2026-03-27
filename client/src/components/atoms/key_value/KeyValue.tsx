import cn from 'classnames';
import { useKeyValue, KeyValueProps } from './KeyValue.vm';
import './KeyValue.scss';

export const KeyValue = (props: KeyValueProps) => {
  const { className, items } = useKeyValue(props);

  return (
    <dl className={cn('key-value', className)}>
      {items.map((item, i) => (
        <div key={i} className="key-value__item">
          <dt className="key-value__label">{item.label}</dt>
          <dd className="key-value__value">{item.value}</dd>
        </div>
      ))}
    </dl>
  );
};
