import { Source } from '../../../hooks/useSources';

export interface SourceSelectProps {
  sources: Source[];
  value: string;
  onChange: (id: string) => void;
}

const KIND_LABELS = { docker: 'Docker', kubernetes: 'Kubernetes' };

export const useSourceSelect = ({ sources, value, onChange }: SourceSelectProps) => {
  return {
    visible: sources.length > 1,
    options: sources.map((s) => ({
      value: s.id,
      label: `${KIND_LABELS[s.kind]} · ${s.name}${s.connected ? '' : ' (offline)'}`,
    })),
    value,
    onChange,
  };
};
