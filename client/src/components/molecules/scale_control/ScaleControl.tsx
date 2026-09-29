import { useScaleControl, ScaleControlProps } from './ScaleControl.vm';
import { Button } from '../../atoms/button/Button';
import { Input } from '../../atoms/input/Input';
import { Stack } from '../../atoms/stack/Stack';
import { Text } from '../../atoms/text/Text';

export const ScaleControl = (props: ScaleControlProps) => {
  const { value, setValue, canApply, apply } = useScaleControl(props);

  return (
    <Stack direction="row" gap="xs" align="center">
      <Text variant="secondary" size="xs">Replicas</Text>
      <Stack flex="0 0 64px">
        <Input value={value} onChange={setValue} size="sm" variant="mono" />
      </Stack>
      <Button variant="secondary" size="sm" onClick={apply} disabled={!canApply || props.disabled}>Scale</Button>
    </Stack>
  );
};
