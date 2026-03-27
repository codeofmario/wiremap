export interface MonacoEditorProps {
  value: string;
  onChange?: (value: string) => void;
  language?: string;
  readOnly?: boolean;
  className?: string;
}

const EXT_TO_LANG: Record<string, string> = {
  js: 'javascript',
  ts: 'typescript',
  jsx: 'javascript',
  tsx: 'typescript',
  json: 'json',
  yml: 'yaml',
  yaml: 'yaml',
  xml: 'xml',
  html: 'html',
  htm: 'html',
  css: 'css',
  scss: 'scss',
  py: 'python',
  rb: 'ruby',
  go: 'go',
  rs: 'rust',
  sh: 'shell',
  bash: 'shell',
  sql: 'sql',
  md: 'markdown',
  toml: 'ini',
  ini: 'ini',
  conf: 'ini',
  cfg: 'ini',
  env: 'ini',
  dockerfile: 'dockerfile',
  makefile: 'makefile',
};

export const detectLanguage = (path: string): string => {
  const filename = path.split('/').pop()?.toLowerCase() || '';

  if (filename === 'dockerfile') return 'dockerfile';
  if (filename === 'makefile') return 'makefile';

  const ext = filename.split('.').pop() || '';
  return EXT_TO_LANG[ext] || 'plaintext';
};

export const useMonacoEditor = (props: MonacoEditorProps) => props;
