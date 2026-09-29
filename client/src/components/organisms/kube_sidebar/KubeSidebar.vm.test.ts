// @vitest-environment jsdom
import { act, renderHook, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { useKubeSidebar, KubeSidebarProps } from './KubeSidebar.vm';
import { kubeApi } from '../../../services/api';
import { KubeKind, KubeRoot } from '../../../types/kubernetes';

vi.mock('../../../services/api', () => ({ kubeApi: { getCounts: vi.fn() } }));

const kind = (group: string, kindName: string, namespaced = true): KubeKind => ({
  group, version: 'v1', resource: `${kindName.toLowerCase()}s`, kind: kindName, namespaced,
});

const KINDS = [
  kind('', 'Pod'), kind('', 'Node', false), kind('', 'Namespace', false), kind('', 'Service'),
  kind('apps', 'Deployment'), kind('argoproj.io', 'Rollout'), kind('', 'Event'),
];

const setup = (overrides: Partial<KubeSidebarProps> = {}) => {
  const props: KubeSidebarProps = {
    cluster: 'prod', kinds: KINDS, namespace: 'shop', root: { type: 'overview' }, onSelect: vi.fn(), ...overrides,
  };
  return { props, ...renderHook(() => useKubeSidebar(props)) };
};

const labels = (sections: { label: string; items: { label: string }[] }[]) =>
  Object.fromEntries(sections.map((s) => [s.label, s.items.map((i) => i.label)]));

describe('useKubeSidebar', () => {
  beforeEach(() => {
    vi.mocked(kubeApi.getCounts).mockResolvedValue({ '/Pod': 7, '/Node': 3, 'apps/Deployment': 2 });
  });

  it('lists only the pinned kinds the cluster serves, plus the lens maps', async () => {
    const { result } = setup();
    await waitFor(() => expect(kubeApi.getCounts).toHaveBeenCalled());

    expect(labels(result.current.sections)).toEqual({
      Cluster: ['Nodes', 'Namespaces', 'Storage map', 'Access map'],
      Workloads: ['Pods', 'Deployments'],
      Network: ['Services'],
    });
  });

  it('counts the served pinned kinds in the selected namespace', async () => {
    const { result } = setup();
    await waitFor(() => expect(result.current.sections[1].items[0].count).toBe(7));

    const [cluster, namespace, kinds] = vi.mocked(kubeApi.getCounts).mock.calls[0];
    expect(cluster).toBe('prod');
    expect(namespace).toBe('shop');
    expect([...kinds].sort()).toEqual(['/Namespace', '/Node', '/Pod', '/Service', 'apps/Deployment']);

    const items = result.current.sections.flatMap((s) => s.items);
    expect(items.find((i) => i.label === 'Nodes')?.count).toBe(3);
    expect(items.find((i) => i.label === 'Deployments')?.count).toBe(2);
    expect(items.find((i) => i.label === 'Storage map')?.count).toBeUndefined();
  });

  it('puts every other served kind, CRDs included, under others', async () => {
    const { result } = setup();
    await waitFor(() => expect(kubeApi.getCounts).toHaveBeenCalled());

    expect(result.current.others.map((o) => [o.label, o.group])).toEqual([['Rollouts', 'argoproj.io'], ['Events', '']]);
  });

  it.each<[string, KubeRoot, string]>([
    ['overview', { type: 'overview' }, 'Namespace shop'],
    ['lens', { type: 'lens', lens: 'storage' }, 'Storage map'],
    ['kind', { type: 'kind', group: 'apps', kind: 'Deployment' }, 'Deployments'],
    ['other kind', { type: 'kind', group: 'argoproj.io', kind: 'Rollout' }, 'Rollouts'],
  ])('marks the selected %s as active', async (_, root, activeLabel) => {
    const { result } = setup({ root });
    await waitFor(() => expect(kubeApi.getCounts).toHaveBeenCalled());

    const all = [result.current.overview, ...result.current.sections.flatMap((s) => s.items), ...result.current.others];
    expect(all.filter((i) => i.active).map((i) => i.label)).toEqual([activeLabel]);
  });

  it('names the overview after the scope and selects roots', async () => {
    const { result, props } = setup({ namespace: '' });
    await waitFor(() => expect(kubeApi.getCounts).toHaveBeenCalled());
    expect(result.current.overview.label).toBe('Cluster overview');

    act(() => result.current.sections[0].items[0].select());
    expect(props.onSelect).toHaveBeenCalledWith({ type: 'lens', lens: 'nodes' });
    act(() => result.current.others[0].select());
    expect(props.onSelect).toHaveBeenCalledWith({ type: 'kind', group: 'argoproj.io', kind: 'Rollout' });
  });

  it('shows no counts when counting fails', async () => {
    vi.mocked(kubeApi.getCounts).mockRejectedValue(new Error('forbidden'));
    const { result } = setup();
    await waitFor(() => expect(kubeApi.getCounts).toHaveBeenCalled());
    expect(result.current.sections.flatMap((s) => s.items).every((i) => i.count === undefined)).toBe(true);
  });
});
