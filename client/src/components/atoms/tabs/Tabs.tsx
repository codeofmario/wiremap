import cn from 'classnames';
import { useTabs, TabsProps } from './Tabs.vm';
import './Tabs.scss';

export const Tabs = (props: TabsProps) => {
  const { tabs, activeTab, onTabClick } = useTabs(props);

  return (
    <div className="tabs">
      {tabs.map((tab) => (
        <button
          key={tab.id}
          className={cn('tabs__item', { 'tabs__item--active': tab.id === activeTab })}
          onClick={() => onTabClick(tab.id)}
        >
          {tab.label}
        </button>
      ))}
    </div>
  );
};
