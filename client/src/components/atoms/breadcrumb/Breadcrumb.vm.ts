export interface BreadcrumbItem {
  id: string;
  label: string;
}

export interface BreadcrumbProps {
  items: BreadcrumbItem[];
  onNavigate: (index: number) => void;
  className?: string;
}

export const useBreadcrumb = ({ items, onNavigate, className }: BreadcrumbProps) => {
  return {
    items: items.map((item, index) => ({ ...item, index, current: index === items.length - 1 })),
    onNavigate,
    className,
  };
};
