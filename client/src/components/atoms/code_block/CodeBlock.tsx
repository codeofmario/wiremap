import cn from 'classnames';
import { useCodeBlock, CodeBlockProps } from './CodeBlock.vm';
import './CodeBlock.scss';

export const CodeBlock = (props: CodeBlockProps) => {
  const { className, children } = useCodeBlock(props);

  return (
    <pre className={cn('code-block', className)}>
      <code className="code-block__code">{children}</code>
    </pre>
  );
};
