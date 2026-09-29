// @vitest-environment jsdom
import { act, renderHook } from '@testing-library/react';
import { beforeEach, describe, expect, it } from 'vitest';
import { useKubeNavigation } from './useKubeNavigation';
import { KubeGraphNode } from '../types/kubernetes';

const node = (overrides: Partial<KubeGraphNode>): KubeGraphNode => ({
  id: 'x', kind: 'Pod', name: 'web-1', namespace: 'shop', status: 'healthy', summary: '', drillable: true, weight: 1, ...overrides,
});

/** Navigation writes location.hash; jsdom fires hashchange asynchronously. */
const settle = () => act(() => new Promise((resolve) => setTimeout(resolve, 0)));

const render = async (hash: string) => {
  history.replaceState(null, '', `/${hash}`);
  const hook = renderHook(() => useKubeNavigation('prod'));
  await settle();
  return hook;
};

describe('useKubeNavigation', () => {
  beforeEach(() => {
    history.replaceState(null, '', '/');
  });

  it('starts at the cluster overview when the hash is for another cluster', async () => {
    const { result } = await render('#/k8s/staging/shop/Pod');
    expect(location.hash).toBe('#/k8s/prod/*/overview');
    expect(result.current.root).toEqual({ type: 'overview' });
    expect(result.current.view).toEqual({ level: 'cluster', lens: 'namespaces' });
    expect(result.current.crumbs.map((c) => c.label)).toEqual(['Cluster']);
    expect(result.current.canGoBack).toBe(false);
  });

  it('restores the view from the hash', async () => {
    const { result } = await render('#/k8s/prod/shop/Deployment.apps/Deployment.apps/shop/web');
    expect(result.current.namespace).toBe('shop');
    expect(result.current.root).toEqual({ type: 'kind', kind: 'Deployment', group: 'apps' });
    expect(result.current.view).toEqual({ level: 'object', namespace: 'shop', kind: 'Deployment', group: 'apps', name: 'web' });
    expect(result.current.crumbs.map((c) => c.label)).toEqual(['Deployments', 'Deployment web']);
    expect(result.current.canGoBack).toBe(true);
  });

  it('derives the topology view from the root', async () => {
    const { result } = await render('#/k8s/prod/shop/overview');
    expect(result.current.view).toEqual({ level: 'namespace', namespace: 'shop' });
    expect(result.current.crumbs[0].label).toBe('shop');

    act(() => result.current.setRoot({ type: 'lens', lens: 'storage' }));
    await settle();
    expect(result.current.view).toEqual({ level: 'cluster', lens: 'storage' });
    expect(result.current.crumbs[0].label).toBe('Storage');

    act(() => result.current.setRoot({ type: 'kind', group: '', kind: 'Pod' }));
    await settle();
    expect(result.current.view).toBeNull();
  });

  it('opens a namespace as the new scope', async () => {
    const { result } = await render('#/k8s/prod/*/Namespace');
    act(() => result.current.drill(node({ kind: 'Namespace', name: 'shop', namespace: undefined })));
    await settle();
    expect(location.hash).toBe('#/k8s/prod/shop/overview');
    expect(result.current.canGoBack).toBe(false);
  });

  it('drills into nodes and objects, then goes back', async () => {
    const { result } = await render('#/k8s/prod/*/nodes');
    act(() => result.current.drill(node({ kind: 'Node', name: 'worker', namespace: undefined })));
    await settle();
    expect(result.current.view).toEqual({ level: 'node', name: 'worker' });

    act(() => result.current.drill(node({ kind: 'Pod', name: 'web-1', namespace: 'shop' })));
    await settle();
    expect(result.current.view).toEqual({ level: 'object', namespace: 'shop', kind: 'Pod', group: '', name: 'web-1' });
    expect(result.current.crumbs.map((c) => c.label)).toEqual(['Nodes', 'Node worker', 'Pod web-1']);

    act(() => result.current.back());
    await settle();
    expect(result.current.view).toEqual({ level: 'node', name: 'worker' });

    act(() => result.current.goTo(0));
    await settle();
    expect(result.current.view).toEqual({ level: 'cluster', lens: 'nodes' });
  });

  it('ignores nodes that cannot be opened', async () => {
    const { result } = await render('#/k8s/prod/shop/overview');
    const before = location.hash;
    act(() => result.current.drill(node({ kind: 'ConfigMap', drillable: false })));
    await settle();
    expect(location.hash).toBe(before);
  });

  it('keeps the root but closes drills when switching namespace', async () => {
    const { result } = await render('#/k8s/prod/shop/Pod/Pod/shop/web-1');
    act(() => result.current.setNamespace('kube-system'));
    await settle();
    expect(location.hash).toBe('#/k8s/prod/kube-system/Pod');

    act(() => result.current.setRoot({ type: 'kind', group: 'apps', kind: 'Deployment' }, ''));
    await settle();
    expect(location.hash).toBe('#/k8s/prod/*/Deployment.apps');
  });

  it('follows browser navigation and clears the hash on unmount', async () => {
    const { result, unmount } = await render('#/k8s/prod/*/overview');
    act(() => {
      location.hash = '#/k8s/prod/shop/Service';
    });
    await settle();
    expect(result.current.root).toEqual({ type: 'kind', kind: 'Service', group: '' });

    unmount();
    expect(location.hash).toBe('');
  });
});
