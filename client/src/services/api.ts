import { ContainerInfo, ContainerInspect, NetworkInfo, HostInfo } from '../types/docker';

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
