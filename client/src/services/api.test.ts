import { afterEach, describe, expect, it, vi } from 'vitest';
import { kubeApi } from './api';

const mockFetch = (body: unknown, ok = true) => {
  const fetchMock = vi.fn().mockResolvedValue({ ok, status: ok ? 200 : 500, statusText: ok ? 'OK' : 'Server Error', json: () => Promise.resolve(body) });
  vi.stubGlobal('fetch', fetchMock);
  return fetchMock;
};

describe('kubeApi.getCounts', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('requests the counts of the given kinds in a namespace', async () => {
    const fetchMock = mockFetch({ '/Pod': 3 });

    const counts = await kubeApi.getCounts('arn:aws/prod', 'shop', ['/Pod', 'apps/Deployment']);

    expect(counts).toEqual({ '/Pod': 3 });
    const url = new URL(fetchMock.mock.calls[0][0], 'http://localhost');
    expect(url.pathname).toBe('/api/k8s/clusters/arn%3Aaws%2Fprod/counts');
    expect(url.searchParams.get('namespace')).toBe('shop');
    expect(url.searchParams.get('kinds')).toBe('/Pod,apps/Deployment');
  });

  it('sends an empty namespace for all namespaces', async () => {
    const fetchMock = mockFetch({});
    await kubeApi.getCounts('prod', '', ['/Pod']);
    expect(new URL(fetchMock.mock.calls[0][0], 'http://localhost').searchParams.get('namespace')).toBe('');
  });

  it('rejects on HTTP errors', async () => {
    mockFetch({}, false);
    await expect(kubeApi.getCounts('prod', '', ['/Pod'])).rejects.toThrow('API error: 500');
  });
});
