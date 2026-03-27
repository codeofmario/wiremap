import { useDashboard } from './Dashboard.vm';
import { MainLayout } from '../../templates/main_layout/MainLayout';
import { TopologyCanvas } from '../../organisms/topology_canvas/TopologyCanvas';
import { ContainerList } from '../../organisms/container_list/ContainerList';
import { ContainerPanel } from '../../organisms/container_panel/ContainerPanel';
import { ErrorBoundary } from '../../atoms/error_boundary/ErrorBoundary';
import { ResizeHandle } from '../../atoms/resize_handle/ResizeHandle';
import { Select } from '../../atoms/select/Select';
import { Stack } from '../../atoms/stack/Stack';
import { Text } from '../../atoms/text/Text';
import { Toggle } from '../../atoms/toggle/Toggle';
import { Toolbar } from '../../atoms/toolbar/Toolbar';

const VIEW_OPTIONS = [
  { id: 'canvas', label: 'Canvas' },
  { id: 'list', label: 'List' },
];

export const Dashboard = () => {
  const {
    hosts, selectedHost, handleHostChange,
    containers, networks, loading,
    selectedContainerId, panelExpanded, panelWidth, viewMode, setViewMode,
    handleSelectContainer, handleContainerIdChange, handleClosePanel, toggleExpand,
    handlePanelResize, handlePanelResizeEnd,
  } = useDashboard();

  return (
    <MainLayout>
      {loading ? (
        <Stack align="center" justify="center" fullHeight>
          <Text variant="secondary" size="sm">Connecting to Docker...</Text>
        </Stack>
      ) : (
        <Stack direction="row" gap="none" fullHeight>
          {!panelExpanded && (
            <Stack flex="1" direction="column" gap="none" overflow="hidden">
              <Toolbar justify="between">
                <Stack direction="row" gap="sm" align="center">
                  {hosts.length > 1 && (
                    <Select
                      options={hosts.map((h) => ({ value: h.name, label: h.name }))}
                      value={selectedHost}
                      onChange={handleHostChange}
                    />
                  )}
                  <Text variant="secondary" size="xs">{containers.length} containers</Text>
                </Stack>
                <Toggle options={VIEW_OPTIONS} value={viewMode} onChange={(v) => setViewMode(v as 'canvas' | 'list')} />
              </Toolbar>
              <ErrorBoundary>
                {viewMode === 'canvas' ? (
                  <TopologyCanvas
                    containers={containers}
                    networks={networks}
                    selectedContainerId={selectedContainerId}
                    onSelectContainer={handleSelectContainer}
                  />
                ) : (
                  <ContainerList
                    containers={containers}
                    networks={networks}
                    selectedContainerId={selectedContainerId}
                    onSelectContainer={handleSelectContainer}
                  />
                )}
              </ErrorBoundary>
            </Stack>
          )}
          {selectedContainerId && !panelExpanded && (
            <ResizeHandle onResize={handlePanelResize} onResizeEnd={handlePanelResizeEnd} />
          )}
          {selectedContainerId && (
            <Stack flex={panelExpanded ? '1' : `0 0 ${panelWidth}px`} direction="column" gap="none" fullHeight overflow="hidden">
              <ErrorBoundary>
                <ContainerPanel
                  host={selectedHost}
                  containerId={selectedContainerId}
                  onClose={handleClosePanel}
                  onContainerIdChange={handleContainerIdChange}
                  expanded={panelExpanded}
                  onToggleExpand={toggleExpand}
                />
              </ErrorBoundary>
            </Stack>
          )}
        </Stack>
      )}
    </MainLayout>
  );
};
