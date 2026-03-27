export interface StatusDotProps {
  state: string;
  className?: string;
}

export const useStatusDot = ({ state, className }: StatusDotProps) => {
  const variant = state === 'running' ? 'running' : state === 'exited' ? 'stopped' : 'other';
  return {
    className: `status-dot--${variant} ${className || ''}`.trim(),
  };
};
