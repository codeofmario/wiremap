import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { kindGraph } from './KubeDashboard.vm';
import { KubeObject } from '../../../types/kubernetes';

const object = (name: string, namespace?: string, extra: Partial<KubeObject> = {}): KubeObject => ({
  name, namespace, kind: 'Deployment', group: 'apps', status: 'healthy', summary: '2/2 ready',
  created: '2026-01-10T11:00:00Z', drillable: true, ...extra,
});

describe('kindGraph', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-01-10T12:00:00Z'));
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it('turns each object into an unlinked node', () => {
    const g = kindGraph([object('web', 'shop'), object('api', 'shop', { status: 'error', drillable: false })], false);

    expect(g.level).toBe('kind');
    expect(g.edges).toEqual([]);
    expect(g.groups).toEqual([]);
    expect(g.nodes).toEqual([
      {
        id: 'Deployment/shop/web', kind: 'Deployment', apiGroup: 'apps', name: 'web', namespace: 'shop',
        status: 'healthy', summary: '2/2 ready', drillable: true, group: undefined, weight: 1, details: { Age: '1h' },
      },
      expect.objectContaining({ id: 'Deployment/shop/api', status: 'error', drillable: false }),
    ]);
  });

  it('groups objects by namespace when showing all namespaces', () => {
    const g = kindGraph([object('web', 'shop'), object('dns', 'kube-system'), object('api', 'shop')], true);

    expect(g.groups).toEqual([
      { id: 'Namespace//shop', name: 'shop', kind: 'Namespace' },
      { id: 'Namespace//kube-system', name: 'kube-system', kind: 'Namespace' },
    ]);
    expect(g.nodes.map((n) => n.group)).toEqual(['Namespace//shop', 'Namespace//kube-system', 'Namespace//shop']);
  });

  it('leaves cluster-scoped objects ungrouped', () => {
    const g = kindGraph([object('node-a', undefined, { kind: 'Node', group: '' })], true);
    expect(g.groups).toEqual([]);
    expect(g.nodes[0]).toMatchObject({ id: 'Node//node-a', group: undefined });
  });
});
