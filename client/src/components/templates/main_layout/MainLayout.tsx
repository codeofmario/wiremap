import { useMainLayout, MainLayoutProps } from './MainLayout.vm';
import { Stack } from '../../atoms/stack/Stack';
import { Header } from '../../atoms/header/Header';

export const MainLayout = (props: MainLayoutProps) => {
  const { children, className } = useMainLayout(props);

  return (
    <Stack direction="column" fullHeight gap="none" className={className}>
      <Header title="Wiremap" subtitle="Docker Network Explorer" />
      <Stack flex="1" direction="column" gap="none" padding="none">
        {children}
      </Stack>
    </Stack>
  );
};
