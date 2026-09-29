import { useState, useEffect } from 'react';

export interface ScaleControlProps {
  replicas: number;
  onScale: (replicas: number) => void;
  disabled?: boolean;
}

export const useScaleControl = ({ replicas, onScale }: ScaleControlProps) => {
  const [value, setValue] = useState(String(replicas));

  useEffect(() => setValue(String(replicas)), [replicas]);

  const parsed = Number(value);
  const valid = value.trim() !== '' && Number.isInteger(parsed) && parsed >= 0;

  return {
    value,
    setValue,
    canApply: valid && parsed !== replicas,
    apply: () => valid && onScale(parsed),
  };
};
