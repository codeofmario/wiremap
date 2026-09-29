import { useKubeSidebar, KubeSidebarProps } from './KubeSidebar.vm';
import { Collapsible } from '../../atoms/collapsible/Collapsible';
import { ListItem } from '../../atoms/list_item/ListItem';
import { ScrollArea } from '../../atoms/scroll_area/ScrollArea';
import { Stack } from '../../atoms/stack/Stack';
import { Text } from '../../atoms/text/Text';

export const KubeSidebar = (props: KubeSidebarProps) => {
  const { overview, sections, others } = useKubeSidebar(props);

  return (
    <ScrollArea flex>
      <Stack gap="sm" padding="sm">
        <ListItem active={overview.active} onClick={overview.select}>
          <Text size="sm" truncate>{overview.label}</Text>
        </ListItem>
        {sections.map((section) => (
          <Stack key={section.label} gap="none">
            <Stack padding="xs">
              <Text variant="label" size="xs">{section.label}</Text>
            </Stack>
            {section.items.map((item) => (
              <ListItem key={item.id} active={item.active} onClick={item.select}>
                <Stack direction="row" gap="sm" align="center" justify="space-between">
                  <Text size="sm" truncate>{item.label}</Text>
                  {item.count !== undefined && <Text variant="secondary" size="xs">{item.count}</Text>}
                </Stack>
              </ListItem>
            ))}
          </Stack>
        ))}
        {others.length > 0 && (
          <Collapsible title={`Other kinds (${others.length})`} defaultOpen={others.some((o) => o.active)}>
            {others.map((item) => (
              <ListItem key={item.id} active={item.active} onClick={item.select}>
                <Stack gap="none">
                  <Text size="sm" truncate>{item.label}</Text>
                  {item.group && <Text variant="secondary" size="xs" truncate>{item.group}</Text>}
                </Stack>
              </ListItem>
            ))}
          </Collapsible>
        )}
      </Stack>
    </ScrollArea>
  );
};
