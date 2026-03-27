import { ReactNode, useState } from 'react';

export interface CollapsibleProps {
  title: ReactNode;
  children: ReactNode;
  defaultOpen?: boolean;
  badge?: ReactNode;
  accentColor?: string;
  className?: string;
}

export const useCollapsible = ({ defaultOpen = true }: CollapsibleProps) => {
  const [open, setOpen] = useState(defaultOpen);
  const toggle = () => setOpen((s) => !s);
  return { open, toggle };
};
