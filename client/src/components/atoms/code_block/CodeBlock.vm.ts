export interface CodeBlockProps {
  children: string;
  className?: string;
}

export const useCodeBlock = ({ className, ...rest }: CodeBlockProps) => {
  return {
    className: className || '',
    ...rest,
  };
};
