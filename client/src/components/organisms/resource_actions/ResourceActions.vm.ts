import { useKubeActions } from '../../../hooks/useKubeActions';
import { kubeActions, KubeTarget } from '../../../services/api';
import { KubeActions } from '../../../types/kubernetes';

export interface ResourceActionsProps {
  target: KubeTarget;
  actions: KubeActions;
  /** Called after an action succeeds, to re-fetch the object */
  onChanged: () => void;
  /** Called after the object was deleted */
  onDeleted: () => void;
}

export const useResourceActions = ({ target, actions, onChanged, onDeleted }: ResourceActionsProps) => {
  const { run, busy, message, error } = useKubeActions(target, onChanged);
  const isNode = target.kind === 'Node' && !target.group;

  return {
    busy,
    message,
    error,
    replicas: actions.replicas,
    canRestart: actions.restart,
    canTrigger: actions.trigger,
    suspended: actions.suspended,
    unschedulable: isNode ? actions.unschedulable : undefined,
    canDelete: actions.delete,
    kindLabel: target.kind.toLowerCase(),

    scale: (replicas: number) => run((t) => kubeActions.scale(t, replicas), () => `Scaled to ${replicas}`),
    restart: () => run(kubeActions.restart, () => 'Rollout restart started'),
    trigger: () => run(kubeActions.trigger, (r) => `Started job ${r?.job}`),
    toggleSuspend: () => run(
      (t) => kubeActions.suspend(t, !actions.suspended),
      () => (actions.suspended ? 'Resumed' : 'Suspended'),
    ),
    toggleCordon: () => run(
      (t) => kubeActions.cordon(t.cluster, t.name, !actions.unschedulable),
      () => (actions.unschedulable ? 'Uncordoned' : 'Cordoned'),
    ),
    drain: () => run(
      (t) => kubeActions.drain(t.cluster, t.name),
      (r) => (r?.refused?.length ? `Drained; not evicted: ${r.refused.join('; ')}` : 'Drained'),
    ),
    remove: () => run(kubeActions.remove, () => 'Deleted').then((ok) => ok && onDeleted()),
  };
};
