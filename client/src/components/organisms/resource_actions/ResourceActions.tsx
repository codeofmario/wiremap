import { useResourceActions, ResourceActionsProps } from './ResourceActions.vm';
import { ConfirmButton } from '../../molecules/confirm_button/ConfirmButton';
import { ScaleControl } from '../../molecules/scale_control/ScaleControl';
import { Stack } from '../../atoms/stack/Stack';
import { Text } from '../../atoms/text/Text';
import { Toolbar } from '../../atoms/toolbar/Toolbar';

export const ResourceActions = (props: ResourceActionsProps) => {
  const {
    busy, message, error, replicas, canRestart, canTrigger, suspended, unschedulable, canDelete, kindLabel,
    scale, restart, trigger, toggleSuspend, toggleCordon, drain, remove,
  } = useResourceActions(props);

  return (
    <Toolbar justify="between">
      <Stack direction="row" gap="sm" align="center">
        {replicas !== undefined && <ScaleControl replicas={replicas} onScale={scale} disabled={busy} />}
        {canRestart && <ConfirmButton label="Restart" question="Restart all pods?" onConfirm={restart} disabled={busy} />}
        {canTrigger && <ConfirmButton label="Run now" question="Start a job now?" onConfirm={trigger} disabled={busy} />}
        {suspended !== undefined && (
          <ConfirmButton label={suspended ? 'Resume' : 'Suspend'} question={suspended ? 'Resume schedule?' : 'Stop scheduling jobs?'} onConfirm={toggleSuspend} disabled={busy} />
        )}
        {unschedulable !== undefined && (
          <ConfirmButton label={unschedulable ? 'Uncordon' : 'Cordon'} question={unschedulable ? 'Allow new pods?' : 'Stop scheduling new pods?'} onConfirm={toggleCordon} disabled={busy} />
        )}
        {unschedulable !== undefined && (
          <ConfirmButton label="Drain" question="Cordon and evict all pods?" onConfirm={drain} disabled={busy} />
        )}
        {(message || error) && <Text variant="secondary" size="xs">{error || message}</Text>}
      </Stack>
      {canDelete && (
        <ConfirmButton label="Delete" question={`Delete this ${kindLabel}?`} onConfirm={remove} disabled={busy} variant="ghost" />
      )}
    </Toolbar>
  );
};
