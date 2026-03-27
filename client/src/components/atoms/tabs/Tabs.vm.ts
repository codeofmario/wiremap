export interface Tab {
  id: string;
  label: string;
}

export interface TabsProps {
  tabs: Tab[];
  activeTab: string;
  onTabChange: (tabId: string) => void;
}

export const useTabs = ({ tabs, activeTab, onTabChange }: TabsProps) => {
  return {
    tabs,
    activeTab,
    onTabClick: onTabChange,
  };
};
