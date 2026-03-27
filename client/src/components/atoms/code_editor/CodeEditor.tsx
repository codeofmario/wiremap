import cn from 'classnames';
import { useCodeEditor, CodeEditorProps } from './CodeEditor.vm';
import './CodeEditor.scss';

export const CodeEditor = (props: CodeEditorProps) => {
  const { editorRef, className } = useCodeEditor(props);

  return (
    <div className={cn('code-editor', className)} ref={editorRef} />
  );
};
