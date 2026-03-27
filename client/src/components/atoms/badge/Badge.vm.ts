export interface BadgeProps {
  label: string;
  variant?: 'default' | 'success' | 'warning' | 'error' | 'info';
  className?: string;
}

export const useBadge = ({ label, variant = 'default', className }: BadgeProps) => {
  return {
    label,
    className: `badge--${variant} ${className || ''}`.trim(),
  };
};
