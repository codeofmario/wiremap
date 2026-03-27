import { ContainerInspect, ContainerNetwork } from '../../../types/docker';

export interface NetworkViewProps {
  inspect: ContainerInspect;
  className?: string;
}

export interface NetworkRow {
  name: string;
  ip: string;
  gateway: string;
  mac: string;
  aliases: string[];
}

export const useNetworkView = ({ inspect }: NetworkViewProps) => {
  const networks: NetworkRow[] = Object.entries(inspect.networks || {}).map(
    ([name, net]: [string, ContainerNetwork]) => ({
      name,
      ip: net.ipAddress || '-',
      gateway: net.gateway || '-',
      mac: net.macAddress || '-',
      aliases: net.aliases || [],
    }),
  );

  return { networks };
};
