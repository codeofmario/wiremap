export interface HeaderProps {
  title: string;
  subtitle?: string;
  className?: string;
}

export const useHeader = ({ className, ...rest }: HeaderProps) => {
  return {
    className: className || '',
    ...rest,
  };
};
