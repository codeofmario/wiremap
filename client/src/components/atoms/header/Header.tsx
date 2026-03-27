import cn from 'classnames';
import { useHeader, HeaderProps } from './Header.vm';
import './Header.scss';

export const Header = (props: HeaderProps) => {
  const { className, title, subtitle } = useHeader(props);

  return (
    <header className={cn('header', className)}>
      <h1 className="header__title">{title}</h1>
      {subtitle && <span className="header__subtitle">{subtitle}</span>}
    </header>
  );
};
