import { useManifestView, ManifestViewProps } from './ManifestView.vm';
import { Button } from '../../atoms/button/Button';
import { CodeEditor } from '../../atoms/code_editor/CodeEditor';
import { Stack } from '../../atoms/stack/Stack';
import { Text } from '../../atoms/text/Text';
import { Toolbar } from '../../atoms/toolbar/Toolbar';

export const ManifestView = (props: ManifestViewProps) => {
  const {
    yaml, error, editable, editing, draft, setDraft, saving, saveError, startEditing, cancel, save,
  } = useManifestView(props);

  if (error || yaml === null) {
    return (
      <Stack padding="md">
        <Text variant="secondary" size="sm">{error || 'Loading...'}</Text>
      </Stack>
    );
  }

  return (
    <Stack fullHeight gap="none">
      {editable && (
        <Toolbar justify="between">
          <Text variant="secondary" size="xs">
            {saveError || (editing ? 'Editing — Save applies the manifest to the cluster' : '')}
          </Text>
          {editing ? (
            <Stack direction="row" gap="xs">
              <Button variant="ghost" size="sm" onClick={cancel} disabled={saving}>Cancel</Button>
              <Button variant="primary" size="sm" onClick={save} disabled={saving}>{saving ? 'Saving...' : 'Save'}</Button>
            </Stack>
          ) : (
            <Button variant="secondary" size="sm" onClick={startEditing}>Edit</Button>
          )}
        </Toolbar>
      )}
      {editing ? (
        <Stack flex="1" overflow="hidden">
          <CodeEditor value={draft} onChange={setDraft} language="yaml" />
        </Stack>
      ) : (
        <Stack flex="1" overflow="hidden">
          <CodeEditor value={yaml} language="yaml" readOnly />
        </Stack>
      )}
    </Stack>
  );
};
