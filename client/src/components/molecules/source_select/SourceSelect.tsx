import { useSourceSelect, SourceSelectProps } from './SourceSelect.vm';
import { Select } from '../../atoms/select/Select';

export const SourceSelect = (props: SourceSelectProps) => {
  const { visible, options, value, onChange } = useSourceSelect(props);

  if (!visible) return null;
  return <Select options={options} value={value} onChange={onChange} />;
};
