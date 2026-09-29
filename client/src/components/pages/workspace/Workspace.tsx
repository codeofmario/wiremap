import { useWorkspace } from './Workspace.vm';
import { Dashboard } from '../dashboard/Dashboard';
import { KubeDashboard } from '../kube_dashboard/KubeDashboard';
import { MainLayout } from '../../templates/main_layout/MainLayout';
import { SourceSelect } from '../../molecules/source_select/SourceSelect';
import { Stack } from '../../atoms/stack/Stack';
import { Text } from '../../atoms/text/Text';

export const Workspace = () => {
  const { sources, selectedSource, selectSource, loading } = useWorkspace();

  if (loading || !selectedSource) {
    return (
      <MainLayout>
        <Stack align="center" justify="center" fullHeight>
          <Text variant="secondary" size="sm">{loading ? 'Connecting...' : 'No Docker hosts or Kubernetes clusters available'}</Text>
        </Stack>
      </MainLayout>
    );
  }

  const sourcePicker = <SourceSelect sources={sources} value={selectedSource.id} onChange={selectSource} />;

  return selectedSource.kind === 'docker' ? (
    <Dashboard key={selectedSource.id} host={selectedSource.name} sourcePicker={sourcePicker} />
  ) : (
    <KubeDashboard key={selectedSource.id} cluster={selectedSource.name} sourcePicker={sourcePicker} />
  );
};
