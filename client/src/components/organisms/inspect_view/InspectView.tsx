import { useInspectView, InspectViewProps } from './InspectView.vm';
import { Badge } from '../../atoms/badge/Badge';
import { Button } from '../../atoms/button/Button';
import { Input } from '../../atoms/input/Input';
import { Stack } from '../../atoms/stack/Stack';
import { Text } from '../../atoms/text/Text';
import { KeyValue } from '../../atoms/key_value/KeyValue';
import { ScrollArea } from '../../atoms/scroll_area/ScrollArea';
import { Panel } from '../../atoms/panel/Panel';
import { Toolbar } from '../../atoms/toolbar/Toolbar';

export const InspectView = (props: InspectViewProps) => {
  const {
    inspect, envVars, showSecrets, toggleSecrets,
    editing, startEditing, cancelEditing,
    updateEnvValue, updateEnvKey, addEnvVar, removeEnvVar,
    saveEnv, saving, error,
  } = useInspectView(props);
  const state = inspect.state;
  const healthStatus = state.health;

  return (
    <ScrollArea flex>
      <Stack gap="lg" padding="md">
        <Panel variant="bordered" padding="md">
          <Stack gap="sm">
            <Text variant="heading" size="md">General</Text>
            <KeyValue items={[
              { label: 'Name', value: inspect.name },
              { label: 'Image', value: inspect.image },
              { label: 'Status', value: (
                <Badge
                  label={state.status}
                  variant={state.running ? 'success' : state.paused ? 'warning' : 'error'}
                />
              )},
              ...(healthStatus ? [{
                label: 'Health',
                value: (
                  <Badge
                    label={healthStatus}
                    variant={healthStatus === 'healthy' ? 'success' : healthStatus === 'unhealthy' ? 'error' : 'warning'}
                  />
                ),
              }] : []),
              { label: 'Created', value: new Date(inspect.created).toLocaleString() },
              { label: 'Started', value: new Date(state.startedAt).toLocaleString() },
              { label: 'Command', value: <Text variant="mono" size="sm">{inspect.command}</Text> },
              { label: 'Entrypoint', value: <Text variant="mono" size="sm">{(inspect.entrypoint || []).join(' ')}</Text> },
              { label: 'Restart Policy', value: inspect.restartPolicy },
            ]} />
          </Stack>
        </Panel>

        <Panel variant="bordered" padding="md">
          <Stack gap="sm">
            <Toolbar justify="between" bordered={false}>
              <Text variant="heading" size="md">Environment Variables</Text>
              <Stack direction="row" gap="xs">
                {!editing && (
                  <Button variant="ghost" size="sm" onClick={startEditing}>Edit</Button>
                )}
                {editing && (
                  <>
                    <Button variant="ghost" size="sm" onClick={addEnvVar}>Add</Button>
                    <Button variant="ghost" size="sm" onClick={cancelEditing}>Cancel</Button>
                    <Button variant="primary" size="sm" onClick={saveEnv} disabled={saving}>
                      {saving ? 'Saving...' : 'Apply (recreate)'}
                    </Button>
                  </>
                )}
                {!editing && (
                  <Button variant="ghost" size="sm" onClick={toggleSecrets}>
                    {showSecrets ? 'Hide' : 'Show'} Secrets
                  </Button>
                )}
              </Stack>
            </Toolbar>
            {error && (
              <Badge label={error} variant="error" />
            )}
            {editing && (
              <Text variant="secondary" size="xs">
                Applying will stop, remove, and recreate the container with the new environment.
              </Text>
            )}
            {editing ? (
              <Stack gap="xs">
                {envVars.map((env, i) => (
                  <Stack key={i} direction="row" gap="xs" align="center">
                    <Input
                      value={env.key}
                      onChange={(v) => updateEnvKey(i, v)}
                      variant="mono"
                      size="sm"
                      placeholder="KEY"
                    />
                    <Text variant="mono" size="sm">=</Text>
                    <Input
                      value={env.value}
                      onChange={(v) => updateEnvValue(i, v)}
                      variant="mono"
                      size="sm"
                      placeholder="value"
                    />
                    <Button variant="ghost" size="sm" onClick={() => removeEnvVar(i)}>x</Button>
                  </Stack>
                ))}
              </Stack>
            ) : (
              <KeyValue items={envVars.map(({ key, value, isSensitive }) => ({
                label: key,
                value: isSensitive && !showSecrets ? '--------' : value,
              }))} />
            )}
          </Stack>
        </Panel>

        <Panel variant="bordered" padding="md">
          <Stack gap="sm">
            <Text variant="heading" size="md">Labels</Text>
            <KeyValue items={Object.entries(inspect.labels || {}).map(([key, value]) => ({
              label: key,
              value,
            }))} />
          </Stack>
        </Panel>
      </Stack>
    </ScrollArea>
  );
};
