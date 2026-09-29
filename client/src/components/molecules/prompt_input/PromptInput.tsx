import { usePromptInput, PromptInputProps } from './PromptInput.vm';
import { Input } from '../../atoms/input/Input';
import { Stack } from '../../atoms/stack/Stack';
import { Text } from '../../atoms/text/Text';

export const PromptInput = (props: PromptInputProps) => {
  const { symbol, value, onChange, onSubmit, onCancel, placeholder, error } = usePromptInput(props);

  return (
    <Stack direction="row" gap="sm" align="center" flex="1">
      <Text variant="mono" size="md">{symbol}</Text>
      <Stack flex="1">
        <Input value={value} onChange={onChange} onSubmit={onSubmit} onCancel={onCancel} placeholder={placeholder} variant="mono" autoFocus />
      </Stack>
      {error && <Text variant="secondary" size="xs">{error}</Text>}
    </Stack>
  );
};
