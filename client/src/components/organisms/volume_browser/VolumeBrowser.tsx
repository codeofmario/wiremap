import { useVolumeBrowser, VolumeBrowserProps, formatSize } from './VolumeBrowser.vm';
import { detectLanguage } from '../../atoms/monaco_editor/MonacoEditor.vm';
import { MonacoEditor } from '../../atoms/monaco_editor/MonacoEditor';
import { Badge } from '../../atoms/badge/Badge';
import { Button } from '../../atoms/button/Button';
import { ListItem } from '../../atoms/list_item/ListItem';
import { Panel } from '../../atoms/panel/Panel';
import { ScrollArea } from '../../atoms/scroll_area/ScrollArea';
import { Stack } from '../../atoms/stack/Stack';
import { Text } from '../../atoms/text/Text';
import { Toolbar } from '../../atoms/toolbar/Toolbar';

export const VolumeBrowser = (props: VolumeBrowserProps) => {
  const {
    mounts, currentPath, entries, loading, fileContent, editedContent, setEditedContent,
    fileLoading, saving, saveStatus, isDirty,
    navigateTo, openFile, saveFile, downloadFile, goUp, breadcrumbs, closeFile,
  } = useVolumeBrowser(props);

  if (fileContent) {
    return (
      <Stack fullHeight>
        <Toolbar justify="between" bordered>
          <Stack direction="row" gap="sm" align="center">
            <Button variant="ghost" size="sm" onClick={closeFile}>Back</Button>
            <Text variant="mono" size="xs" truncate>{fileContent.path}</Text>
            <Badge label={formatSize(fileContent.size)} />
          </Stack>
          <Stack direction="row" gap="xs" align="center">
            {saveStatus && <Text variant="secondary" size="xs">{saveStatus}</Text>}
            <Button variant="ghost" size="sm" onClick={downloadFile}>Download</Button>
            {isDirty && (
              <Button variant="primary" size="sm" onClick={saveFile} disabled={saving}>
                {saving ? 'Saving...' : 'Save'}
              </Button>
            )}
          </Stack>
        </Toolbar>
        {fileContent.binary ? (
          <Stack flex="1" align="center" justify="center">
            <Text variant="secondary" size="sm">Binary file ({formatSize(fileContent.size)})</Text>
          </Stack>
        ) : (
          <MonacoEditor
            value={editedContent || ''}
            onChange={setEditedContent}
            language={detectLanguage(fileContent.path)}
          />
        )}
      </Stack>
    );
  }

  if (!currentPath) {
    return (
      <Stack fullHeight>
        <Stack padding="md" gap="sm">
          <Text variant="label" size="sm">Volumes &amp; Mounts</Text>
        </Stack>
        {mounts.length === 0 ? (
          <Stack flex="1" align="center" justify="center">
            <Text variant="secondary" size="sm">No volumes or mounts</Text>
          </Stack>
        ) : (
          <ScrollArea flex>
            <Stack gap="sm" padding="md">
              {mounts.map((m, i) => (
                <Panel key={i} variant="bordered" padding="md">
                  <ListItem onClick={() => navigateTo(m.destination)}>
                    <Stack gap="xs">
                      <Stack direction="row" gap="xs">
                        <Badge label={m.type} variant="info" />
                        <Badge label={m.rw ? 'RW' : 'RO'} variant={m.rw ? 'success' : 'warning'} />
                      </Stack>
                      <Text variant="mono" size="sm">{m.destination}</Text>
                      <Text variant="mono" size="xs">{m.source}</Text>
                    </Stack>
                  </ListItem>
                </Panel>
              ))}
            </Stack>
          </ScrollArea>
        )}
        <Toolbar justify="start" bordered>
          <Button variant="secondary" size="sm" onClick={() => navigateTo('/')}>
            Browse root filesystem
          </Button>
        </Toolbar>
      </Stack>
    );
  }

  return (
    <Stack fullHeight>
      <Toolbar justify="start" bordered>
        <Button variant="ghost" size="sm" onClick={goUp}>Up</Button>
        <Stack direction="row" gap="xs">
          <Text variant="mono" size="xs" onClick={() => navigateTo('/')}>/</Text>
          {breadcrumbs.map((b) => (
            <Text key={b.path} variant="mono" size="xs" onClick={() => navigateTo(b.path)}>
              {b.name}/
            </Text>
          ))}
        </Stack>
      </Toolbar>
      <ScrollArea flex>
        {loading ? (
          <Stack flex="1" align="center" justify="center">
            <Text variant="secondary" size="sm">Loading...</Text>
          </Stack>
        ) : entries.length === 0 ? (
          <Stack flex="1" align="center" justify="center">
            <Text variant="secondary" size="sm">Empty directory</Text>
          </Stack>
        ) : (
          entries.map((entry) => (
            <ListItem
              key={entry.path}
              onClick={() => entry.isDir ? navigateTo(entry.path) : openFile(entry.path)}
            >
              <Stack direction="row" gap="sm" align="center">
                <Badge label={entry.isDir ? 'D' : 'F'} />
                <Text variant="mono" size="xs" truncate>{entry.name}</Text>
                <Text variant="mono" size="xs">{entry.permissions}</Text>
                {!entry.isDir && (
                  <Text variant="mono" size="xs">{formatSize(entry.size)}</Text>
                )}
              </Stack>
            </ListItem>
          ))
        )}
      </ScrollArea>
      {fileLoading && (
        <Stack align="center" justify="center" padding="md">
          <Text variant="secondary" size="sm">Loading file...</Text>
        </Stack>
      )}
    </Stack>
  );
};
