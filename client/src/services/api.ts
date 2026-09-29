import { ContainerInfo, ContainerInspect, NetworkInfo, HostInfo } from '../types/docker';
import { KubeCluster, KubeGraph, KubeKind, KubeObjectList, KubePod, KubeResource, KubeView } from '../types/kubernetes';

const BASE_URL = '/api';

const hostParam = (host: string) => host ? `?host=${encodeURIComponent(host)}` : '';
const hostParamAppend = (host: string) => host ? `&host=${encodeURIComponent(host)}` : '';

async function fetchJson<T>(url: string): Promise<T> {
  const response = await fetch(`${BASE_URL}${url}`);
  if (!response.ok) {
    throw new Error(`API error: ${response.status} ${response.statusText}`);
  }
  return response.json();
}

async function sendJson<T>(method: string, url: string, body?: unknown): Promise<T | null> {
  const response = await fetch(`${BASE_URL}${url}`, {
    method,
    headers: body === undefined ? undefined : { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  });
  const data = response.status === 204 ? null : await response.json().catch(() => null);
  if (!response.ok) {
    throw new Error(data?.message || `API error: ${response.status} ${response.statusText}`);
  }
  return data;
}

export const api = {
  getHosts: () => fetchJson<HostInfo[]>('/hosts'),
  getContainers: (host: string) => fetchJson<ContainerInfo[]>(`/containers${hostParam(host)}`),
  inspectContainer: (host: string, id: string) => fetchJson<ContainerInspect>(`/containers/${id}${hostParam(host)}`),
  getNetworks: (host: string) => fetchJson<NetworkInfo[]>(`/networks${hostParam(host)}`),
  inspectNetwork: (host: string, id: string) => fetchJson<NetworkInfo>(`/networks/${id}${hostParam(host)}`),
  listDir: (host: string, id: string, path: string) =>
    fetchJson<any[]>(`/containers/${id}/fs?path=${encodeURIComponent(path)}${hostParamAppend(host)}`),
  readFile: (host: string, id: string, path: string) =>
    fetchJson<any>(`/containers/${id}/fs/read?path=${encodeURIComponent(path)}${hostParamAppend(host)}`),
};

const k8sPath = (cluster: string) => `/k8s/clusters/${encodeURIComponent(cluster)}`;

const topologyQuery = (view: KubeView) => {
  const params = new URLSearchParams({ level: view.level });
  if (view.lens) params.set('lens', view.lens);
  if (view.namespace) params.set('namespace', view.namespace);
  if (view.kind) params.set('kind', view.kind);
  if (view.group) params.set('group', view.group);
  if (view.name) params.set('name', view.name);
  return params.toString();
};

export const kubeApi = {
  getClusters: () => fetchJson<KubeCluster[]>('/k8s/clusters'),
  getTopology: (cluster: string, view: KubeView) =>
    fetchJson<KubeGraph>(`${k8sPath(cluster)}/topology?${topologyQuery(view)}`),
  getResource: (cluster: string, kind: string, group: string, name: string, namespace?: string) =>
    fetchJson<KubeResource>(`${k8sPath(cluster)}/resources/${kind}/${encodeURIComponent(name)}?${new URLSearchParams({ group, namespace: namespace || '' })}`),
  getKinds: (cluster: string) => fetchJson<KubeKind[]>(`${k8sPath(cluster)}/kinds`),
  getObjects: (cluster: string, kind: KubeKind, namespace: string, continueToken = '') =>
    fetchJson<KubeObjectList>(`${k8sPath(cluster)}/objects?${new URLSearchParams({
      group: kind.group, version: kind.version, resource: kind.resource, namespace, continue: continueToken,
    })}`),
  /** Object counts keyed by "group/Kind"; kinds the cluster doesn't serve are left out */
  getCounts: (cluster: string, namespace: string, kinds: string[]) =>
    fetchJson<Record<string, number>>(`${k8sPath(cluster)}/counts?${new URLSearchParams({ namespace, kinds: kinds.join(',') })}`),
  getPod: (cluster: string, namespace: string, name: string) =>
    fetchJson<KubePod>(`${k8sPath(cluster)}/namespaces/${encodeURIComponent(namespace)}/pods/${encodeURIComponent(name)}`),
};

/** Identifies the object an action applies to. */
export interface KubeTarget {
  cluster: string;
  kind: string;
  group: string;
  name: string;
  namespace?: string;
}

const resourcePath = (t: KubeTarget, action = '') =>
  `${k8sPath(t.cluster)}/resources/${t.kind}/${encodeURIComponent(t.name)}${action}?${new URLSearchParams({
    group: t.group, namespace: t.namespace || '',
  })}`;

const nodePath = (cluster: string, node: string, action: string) =>
  `${k8sPath(cluster)}/nodes/${encodeURIComponent(node)}/${action}`;

export const kubeActions = {
  update: (t: KubeTarget, yaml: string) => sendJson('PUT', resourcePath(t), { yaml }),
  remove: (t: KubeTarget) => sendJson('DELETE', resourcePath(t)),
  scale: (t: KubeTarget, replicas: number) => sendJson('POST', resourcePath(t, '/scale'), { replicas }),
  restart: (t: KubeTarget) => sendJson('POST', resourcePath(t, '/restart')),
  trigger: (t: KubeTarget) => sendJson<{ job: string }>('POST', resourcePath(t, '/trigger')),
  suspend: (t: KubeTarget, value: boolean) => sendJson('POST', resourcePath(t, '/suspend'), { value }),
  cordon: (cluster: string, node: string, value: boolean) => sendJson('POST', nodePath(cluster, node, 'cordon'), { value }),
  drain: (cluster: string, node: string) => sendJson<{ refused: string[] | null }>('POST', nodePath(cluster, node, 'drain')),
};
