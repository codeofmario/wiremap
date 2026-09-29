import { useConfirmButton, ConfirmButtonProps } from './ConfirmButton.vm';
import { Button } from '../../atoms/button/Button';
import { Stack } from '../../atoms/stack/Stack';
import { Text } from '../../atoms/text/Text';

export const ConfirmButton = (props: ConfirmButtonProps) => {
  const { asking, ask, cancel, confirm } = useConfirmButton(props);

  if (!asking) {
    return (
      <Button variant={props.variant || 'secondary'} size="sm" onClick={ask} disabled={props.disabled}>
        {props.label}
      </Button>
    );
  }

  return (
    <Stack direction="row" gap="xs" align="center">
      <Text size="xs">{props.question}</Text>
      <Button variant="primary" size="sm" onClick={confirm}>Yes</Button>
      <Button variant="ghost" size="sm" onClick={cancel}>No</Button>
    </Stack>
  );
};
