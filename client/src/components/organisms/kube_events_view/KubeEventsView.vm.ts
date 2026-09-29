import { KubeEvent } from '../../../types/kubernetes';
import { formatAge } from '../../../services/format';

export interface KubeEventsViewProps {
  events: KubeEvent[] | null;
  error: string | null;
}

const COLUMNS = [
  { key: 'type', label: 'Type' },
  { key: 'reason', label: 'Reason' },
  { key: 'message', label: 'Message' },
  { key: 'count', label: 'Count' },
  { key: 'lastSeen', label: 'Last seen' },
];

export const useKubeEventsView = ({ events, error }: KubeEventsViewProps) => {
  return {
    columns: COLUMNS,
    events: (events || []).map((e) => ({ ...e, age: `${formatAge(e.lastSeen)} ago` })),
    loading: events === null && error === null,
    error,
  };
};
