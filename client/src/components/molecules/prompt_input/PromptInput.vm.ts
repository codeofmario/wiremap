export interface PromptInputProps {
  /** Shown before the input, e.g. ":" or "/" */
  symbol: string;
  value: string;
  onChange: (value: string) => void;
  onSubmit: () => void;
  onCancel: () => void;
  placeholder?: string;
  error?: string | null;
}

export const usePromptInput = (props: PromptInputProps) => props;
