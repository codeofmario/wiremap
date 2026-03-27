export interface HostInfo {
  name: string;
}

export interface ContainerInfo {
  id: string;
  names: string[];
  image: string;
  state: string;
  status: string;
  created: number;
  ports: PortMapping[];
  labels: Record<string, string>;
  networks: Record<string, ContainerNetwork>;
}

export interface ContainerNetwork {
  ipAddress: string;
  gateway: string;
  macAddress: string;
  networkId: string;
  aliases: string[] | null;
}

export interface PortMapping {
  ip?: string;
  privatePort: number;
  publicPort?: number;
  type: string;
}

export interface ContainerInspect {
  id: string;
  name: string;
  state: ContainerState;
  image: string;
  command: string;
  entrypoint: string[] | null;
  env: string[] | null;
  labels: Record<string, string>;
  restartPolicy: string;
  mounts: MountInfo[];
  portBindings: Record<string, HostBinding[]> | null;
  networks: Record<string, ContainerNetwork>;
  created: string;
}

export interface ContainerState {
  status: string;
  running: boolean;
  paused: boolean;
  restarting: boolean;
  startedAt: string;
  finishedAt: string;
  health?: string;
}

export interface HostBinding {
  hostIp: string;
  hostPort: string;
}

export interface MountInfo {
  type: string;
  name?: string;
  source: string;
  destination: string;
  mode: string;
  rw: boolean;
}

export interface NetworkInfo {
  id: string;
  name: string;
  driver: string;
  scope: string;
  subnet?: string;
  gateway?: string;
  containers: Record<string, NetworkContainer>;
}

export interface NetworkContainer {
  name: string;
  ipv4Address: string;
  macAddress: string;
}

export interface ContainerStats {
  cpuPercent: number;
  memoryUsage: number;
  memoryLimit: number;
  memoryPercent: number;
  networkRx: number;
  networkTx: number;
  timestamp: string;
}

export interface LogEntry {
  containerId: string;
  type: 'stdout' | 'stderr';
  text: string;
}

export interface TopologyNode {
  id: string;
  name: string;
  image: string;
  state: string;
  networks: string[];
  x?: number;
  y?: number;
  vx?: number;
  vy?: number;
  fx?: number | null;
  fy?: number | null;
}

export interface TopologyLink {
  source: string | TopologyNode;
  target: string | TopologyNode;
  network: string;
}

export interface TopologyGroup {
  id: string;
  name: string;
  driver: string;
  scope: string;
  subnet: string;
  gateway: string;
  containerCount: number;
  nodes: string[];
}
