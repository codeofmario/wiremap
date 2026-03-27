import { ContainerInspect } from '../../../types/docker';

export interface PortsVolumesViewProps {
  inspect: ContainerInspect;
  className?: string;
}

export interface PortRow {
  container: string;
  host: string;
  protocol: string;
}

export interface VolumeRow {
  type: string;
  name: string;
  source: string;
  destination: string;
  readOnly: boolean;
}

export const usePortsVolumesView = ({ inspect }: PortsVolumesViewProps) => {
  const ports: PortRow[] = Object.entries(inspect.portBindings || {}).flatMap(
    ([containerPort, bindings]) =>
      (bindings || []).map((b) => ({
        container: containerPort.replace('/tcp', '').replace('/udp', ''),
        host: `${b.hostIp || '0.0.0.0'}:${b.hostPort}`,
        protocol: containerPort.includes('/') ? containerPort.split('/')[1] : 'tcp',
      })),
  );

  const volumes: VolumeRow[] = (inspect.mounts || []).map((m) => ({
    type: m.type,
    name: m.name || '-',
    source: m.source,
    destination: m.destination,
    readOnly: !m.rw,
  }));

  return { ports, volumes };
};
