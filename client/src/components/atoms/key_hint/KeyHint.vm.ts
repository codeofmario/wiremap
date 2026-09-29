export interface KeyHintProps {
  /** The key to press, e.g. "l" or "enter" */
  keys: string;
  label: string;
  className?: string;
}

export const useKeyHint = (props: KeyHintProps) => props;
