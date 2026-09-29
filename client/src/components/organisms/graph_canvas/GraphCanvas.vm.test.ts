import { describe, expect, it } from 'vitest';
import { computeLayout, kindStyle, KIND_STYLES, nodeRadius } from './GraphCanvas.vm';
import { KubeGraph, KubeGraphEdge, KubeGraphNode, KubeLevel } from '../../../types/kubernetes';

const node = (kind: string, name: string, extra: Partial<KubeGraphNode> = {}): KubeGraphNode => ({
  id: `${kind}/${name}`, kind, name, status: 'healthy', summary: '', drillable: false, weight: 1, ...extra,
});

const graph = (level: KubeLevel, nodes: KubeGraphNode[], edges: KubeGraphEdge[] = [], groups: KubeGraph['groups'] = []): KubeGraph =>
  ({ level, nodes, edges, groups });

describe('kindStyle', () => {
  it('uses the known style of a kind', () => {
    expect(kindStyle('Deployment')).toBe(KIND_STYLES.Deployment);
    expect(kindStyle('Container').abbr).toBe('CTR');
  });

  it('abbreviates unknown kinds', () => {
    expect(kindStyle('Rollout')).toEqual({ abbr: 'ROL', color: '#8b8fa7' });
  });
});

describe('nodeRadius', () => {
  it('grows with pod count at cluster level, up to a cap', () => {
    const small = nodeRadius(node('Namespace', 'a', { weight: 1 }), 'cluster');
    const big = nodeRadius(node('Namespace', 'b', { weight: 100 }), 'cluster');
    expect(big).toBeGreaterThan(small);
    expect(nodeRadius(node('Namespace', 'c', { weight: 1_000_000 }), 'cluster')).toBe(64);
  });

  it('is fixed below cluster level', () => {
    for (const level of ['namespace', 'object', 'node', 'kind'] as KubeLevel[]) {
      expect(nodeRadius(node('Pod', 'p', { weight: 50 }), level)).toBe(22);
    }
  });
});

describe('computeLayout', () => {
  it('lays the namespace out left to right, skipping empty columns', () => {
    const g = graph('namespace', [
      node('Ingress', 'shop'), node('Service', 'web'), node('Deployment', 'web'), node('ConfigMap', 'cfg'),
    ]);
    const { targets } = computeLayout(g, 1200, 800);
    const x = (id: string) => targets.get(id)!.x;
    expect(x('Ingress/shop')).toBeLessThan(x('Service/web'));
    expect(x('Service/web')).toBeLessThan(x('Deployment/web'));
    expect(x('Deployment/web')).toBeLessThan(x('ConfigMap/cfg'));
    // Four used columns, evenly spaced from the left edge
    expect(x('Ingress/shop')).toBe(100);
    expect(x('Service/web') - x('Ingress/shop')).toBeCloseTo(x('ConfigMap/cfg') - x('Deployment/web'));
  });

  it('stacks a column vertically sorted by name', () => {
    const g = graph('namespace', [node('Service', 'b'), node('Service', 'a'), node('Service', 'c')]);
    const { targets } = computeLayout(g, 1200, 800);
    const y = (name: string) => targets.get(`Service/${name}`)!.y;
    expect(y('a')).toBeLessThan(y('b'));
    expect(y('b')).toBeLessThan(y('c'));
    expect(targets.get('Service/a')!.x).toBe(600);
  });

  it('orders object levels by ownership depth', () => {
    const g = graph('object', [node('Deployment', 'web'), node('Pod', 'web-1'), node('Container', 'nginx')], [
      { source: 'Deployment/web', target: 'Pod/web-1', kind: 'owns' },
      { source: 'Pod/web-1', target: 'Container/nginx', kind: 'owns' },
    ]);
    const { targets } = computeLayout(g, 1200, 800);
    expect(targets.get('Deployment/web')!.x).toBeLessThan(targets.get('Pod/web-1')!.x);
    expect(targets.get('Pod/web-1')!.x).toBeLessThan(targets.get('Container/nginx')!.x);
  });

  it('leaves kind lists free, even for kinds that have lens columns', () => {
    const g = graph('kind', [node('PersistentVolumeClaim', 'a'), node('PersistentVolumeClaim', 'b')]);
    expect(computeLayout(g, 1200, 800).targets.size).toBe(0);
  });

  it('uses the storage and access columns for cluster lenses', () => {
    const storage = computeLayout(graph('cluster', [node('PersistentVolumeClaim', 'c'), node('PersistentVolume', 'v'), node('StorageClass', 's')]), 1200, 800);
    expect(storage.targets.get('PersistentVolumeClaim/c')!.x).toBeLessThan(storage.targets.get('PersistentVolume/v')!.x);
    expect(storage.targets.get('PersistentVolume/v')!.x).toBeLessThan(storage.targets.get('StorageClass/s')!.x);

    const access = computeLayout(graph('cluster', [node('ServiceAccount', 'sa'), node('RoleBinding', 'rb'), node('Role', 'r')]), 1200, 800);
    expect(access.targets.get('ServiceAccount/sa')!.x).toBeLessThan(access.targets.get('RoleBinding/rb')!.x);
    expect(access.targets.get('RoleBinding/rb')!.x).toBeLessThan(access.targets.get('Role/r')!.x);
  });

  it('does not anchor namespace bubbles at cluster level', () => {
    expect(computeLayout(graph('cluster', [node('Namespace', 'a'), node('Namespace', 'b')]), 1200, 800).targets.size).toBe(0);
  });

  it('places groups on a grid covering the canvas', () => {
    const groups = ['a', 'b', 'c', 'd'].map((n) => ({ id: `Node//${n}`, name: n, kind: 'Node' }));
    const g = graph('node', groups.map((grp) => node('Pod', grp.name, { group: grp.id })), [], groups);
    const { targets, groupCenters } = computeLayout(g, 1200, 800);

    expect(targets.size).toBe(0);
    expect([...groupCenters.values()]).toEqual([
      { x: 300, y: 200 }, { x: 900, y: 200 },
      { x: 300, y: 600 }, { x: 900, y: 600 },
    ]);
  });

  it('puts groups to the right of the columns', () => {
    const groups = [{ id: 'Node//a', name: 'a', kind: 'Node' }];
    const g = graph('object', [node('Deployment', 'web'), node('Pod', 'web-1', { group: 'Node//a' })],
      [{ source: 'Deployment/web', target: 'Pod/web-1', kind: 'owns' }], groups);
    const { targets, groupCenters } = computeLayout(g, 1200, 800);

    expect(targets.has('Pod/web-1')).toBe(false);
    expect(groupCenters.get('Node//a')!.x).toBeGreaterThan(targets.get('Deployment/web')!.x);
  });
});
