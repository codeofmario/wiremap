import { useState } from 'react';

export interface ConfirmButtonProps {
  label: string;
  /** Question shown in place of the button until the user confirms or cancels */
  question: string;
  onConfirm: () => void;
  disabled?: boolean;
  variant?: 'primary' | 'secondary' | 'ghost';
}

export const useConfirmButton = ({ onConfirm }: ConfirmButtonProps) => {
  const [asking, setAsking] = useState(false);

  return {
    asking,
    ask: () => setAsking(true),
    cancel: () => setAsking(false),
    confirm: () => {
      setAsking(false);
      onConfirm();
    },
  };
};
