import { useKubeDashboard, KubeDashboardProps, CATEGORY_OPTIONS, DISPLAY_OPTIONS } from './KubeDashboard.vm';
import { MainLayout } from '../../templates/main_layout/MainLayout';
import { GraphCanvas } from '../../organisms/graph_canvas/GraphCanvas';
import { KubeObjectList } from '../../organisms/kube_object_list/KubeObjectList';
import { KubeSidebar } from '../../organisms/kube_sidebar/KubeSidebar';
import { PodPanel } from '../../organisms/pod_panel/PodPanel';
import { ResourcePanel } from '../../organisms/resource_panel/ResourcePanel';
import { PromptInput } from '../../molecules/prompt_input/PromptInput';
import { Breadcrumb } from '../../atoms/breadcrumb/Breadcrumb';
import { Button } from '../../atoms/button/Button';
import { ErrorBoundary } from '../../atoms/error_boundary/ErrorBoundary';
import { KeyHint } from '../../atoms/key_hint/KeyHint';
import { Panel } from '../../atoms/panel/Panel';
import { ResizeHandle } from '../../atoms/resize_handle/ResizeHandle';
import { Select } from '../../atoms/select/Select';
import { Stack } from '../../atoms/stack/Stack';
import { Text } from '../../atoms/text/Text';
import { Toggle } from '../../atoms/toggle/Toggle';
import { Toolbar } from '../../atoms/toolbar/Toolbar';

export const KubeDashboard = (props: KubeDashboardProps) => {
  const {
    sourcePicker, cluster, kinds, namespace, root, namespaceOptions, setNamespace, setRoot,
    crumbs, goTo, viewKey,
    showCategories, category, setCategory, showDisplay, display, setDisplay,
    prompt, command, setCommand, commandError, submitCommand, closePrompt, filter, setFilter, clearFilter,
    graph, listNodes, objectCount, hasMore, loadMore, loadingMore, placeholder, hints,
    selected, podSelection, panelRequest, handleSelect, handleClose, open, drill,
    panelExpanded, panelWidth, toggleExpand, handlePanelResize, handlePanelResizeEnd,
  } = useKubeDashboard(props);

  return (
    <MainLayout>
      <Stack direction="row" gap="none" fullHeight>
        {!panelExpanded && (
          <Stack flex="0 0 220px" direction="column" gap="none" overflow="hidden">
            <Panel variant="bordered" padding="none" fullHeight overflow="hidden">
              <KubeSidebar cluster={cluster} kinds={kinds} namespace={namespace} root={root} onSelect={setRoot} />
            </Panel>
          </Stack>
        )}
        {!panelExpanded && (
          <Stack flex="1" direction="column" gap="none" overflow="hidden">
            <Toolbar justify="between">
              <Stack direction="row" gap="sm" align="center" overflow="hidden">
                {sourcePicker}
                <Select options={namespaceOptions} value={namespace} onChange={setNamespace} />
                <Breadcrumb items={crumbs} onNavigate={goTo} />
                {graph && <Text variant="secondary" size="xs">{objectCount}</Text>}
              </Stack>
              <Stack direction="row" gap="sm" align="center">
                {showCategories && <Toggle options={CATEGORY_OPTIONS} value={category} onChange={setCategory} />}
                {showDisplay && <Toggle options={DISPLAY_OPTIONS} value={display} onChange={setDisplay} />}
              </Stack>
            </Toolbar>
            {prompt === 'command' && (
              <Toolbar>
                <PromptInput
                  symbol=":"
                  value={command}
                  onChange={setCommand}
                  onSubmit={submitCommand}
                  onCancel={closePrompt}
                  placeholder="resource, e.g. po, deploy, svc, ns, nodes — add a namespace or 'all': deploy kube-system"
                  error={commandError}
                />
              </Toolbar>
            )}
            {(prompt === 'filter' || filter) && (
              <Toolbar>
                {prompt === 'filter' ? (
                  <PromptInput symbol="/" value={filter} onChange={setFilter} onSubmit={closePrompt} onCancel={clearFilter} placeholder="filter by name or kind" />
                ) : (
                  <Stack direction="row" gap="sm" align="center">
                    <Text variant="mono" size="sm">/{filter}</Text>
                    <Button variant="ghost" size="sm" onClick={clearFilter}>Clear filter</Button>
                  </Stack>
                )}
              </Toolbar>
            )}
            <ErrorBoundary>
              {placeholder || !graph ? (
                <Stack align="center" justify="center" flex="1">
                  <Text variant="secondary" size="sm">{placeholder}</Text>
                </Stack>
              ) : display === 'list' ? (
                <KubeObjectList
                  nodes={listNodes}
                  selectedId={selected?.id || null}
                  onSelect={handleSelect}
                  hasMore={hasMore}
                  loadMore={loadMore}
                  loadingMore={loadingMore}
                />
              ) : (
                <Stack flex="1" overflow="hidden">
                  <GraphCanvas
                    graph={graph}
                    viewKey={viewKey}
                    selectedId={selected?.id || null}
                    onSelect={handleSelect}
                    onDrill={open}
                  />
                </Stack>
              )}
            </ErrorBoundary>
            <Toolbar>
              <Stack direction="row" gap="md" align="center" overflow="hidden">
                {hints.map((hint) => <KeyHint key={hint.keys} keys={hint.keys} label={hint.label} />)}
              </Stack>
            </Toolbar>
          </Stack>
        )}
        {selected && !panelExpanded && (
          <ResizeHandle onResize={handlePanelResize} onResizeEnd={handlePanelResizeEnd} />
        )}
        {selected && (
          <Stack flex={panelExpanded ? '1' : `0 0 ${panelWidth}px`} direction="column" gap="none" fullHeight overflow="hidden">
            <ErrorBoundary>
              {podSelection && selected.namespace ? (
                <PodPanel
                  key={selected.id}
                  cluster={cluster}
                  namespace={selected.namespace}
                  name={podSelection.name}
                  container={podSelection.container}
                  request={panelRequest}
                  onOpen={selected.drillable && !podSelection.container ? () => drill(selected) : undefined}
                  onClose={handleClose}
                  expanded={panelExpanded}
                  onToggleExpand={toggleExpand}
                />
              ) : (
                <ResourcePanel
                  key={selected.id}
                  cluster={cluster}
                  node={selected}
                  request={panelRequest}
                  onClose={handleClose}
                  onDrill={drill}
                  expanded={panelExpanded}
                  onToggleExpand={toggleExpand}
                />
              )}
            </ErrorBoundary>
          </Stack>
        )}
      </Stack>
    </MainLayout>
  );
};
