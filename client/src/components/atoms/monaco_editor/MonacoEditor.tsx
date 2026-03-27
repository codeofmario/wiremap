import cn from 'classnames';
import Editor from '@monaco-editor/react';
import { useMonacoEditor, MonacoEditorProps } from './MonacoEditor.vm';
import './MonacoEditor.scss';

export const MonacoEditor = (props: MonacoEditorProps) => {
  const { value, onChange, language = 'plaintext', readOnly = false, className } = useMonacoEditor(props);

  return (
    <div className={cn('monaco-editor-wrapper', className)}>
      <Editor
        value={value}
        language={language}
        theme="vs-dark"
        onChange={(val) => onChange?.(val || '')}
        options={{
          readOnly,
          minimap: { enabled: false },
          fontSize: 13,
          fontFamily: "'JetBrains Mono', 'Fira Code', 'Consolas', monospace",
          lineNumbers: 'on',
          scrollBeyondLastLine: false,
          wordWrap: 'on',
          automaticLayout: true,
          padding: { top: 8, bottom: 8 },
        }}
      />
    </div>
  );
};
