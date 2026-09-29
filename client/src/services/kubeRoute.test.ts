import { describe, expect, it } from 'vitest';
import { defaultRoute, KubeRoute, parseKubeHash, toKubeHash } from './kubeRoute';

const route = (overrides: Partial<KubeRoute>): KubeRoute => ({ ...defaultRoute('prod'), ...overrides });

describe('parseKubeHash', () => {
  it('ignores hashes that are not Kubernetes routes', () => {
    expect(parseKubeHash('')).toBeNull();
    expect(parseKubeHash('#/docker/local')).toBeNull();
    expect(parseKubeHash('#/k8s/')).toBeNull();
  });

  it('defaults to the all-namespace overview', () => {
    expect(parseKubeHash('#/k8s/prod')).toEqual(defaultRoute('prod'));
    expect(parseKubeHash('#/k8s/prod/*')).toEqual(defaultRoute('prod'));
  });

  it('reads the namespace and lens roots', () => {
    expect(parseKubeHash('#/k8s/prod/shop/overview')).toEqual(route({ namespace: 'shop' }));
    for (const lens of ['nodes', 'storage', 'access'] as const) {
      expect(parseKubeHash(`#/k8s/prod/*/${lens}`)?.root).toEqual({ type: 'lens', lens });
    }
  });

  it('reads kind roots with and without an API group', () => {
    expect(parseKubeHash('#/k8s/prod/*/Pod')?.root).toEqual({ type: 'kind', kind: 'Pod', group: '' });
    expect(parseKubeHash('#/k8s/prod/*/Rollout.argoproj.io')?.root).toEqual({ type: 'kind', kind: 'Rollout', group: 'argoproj.io' });
  });

  it('reads drills as kind/namespace/name triples', () => {
    const parsed = parseKubeHash('#/k8s/prod/shop/Deployment.apps/Deployment.apps/shop/web/Pod/shop/web-1/Node/-/worker');
    expect(parsed?.drills).toEqual([
      { level: 'object', namespace: 'shop', kind: 'Deployment', group: 'apps', name: 'web' },
      { level: 'object', namespace: 'shop', kind: 'Pod', group: '', name: 'web-1' },
      { level: 'node', name: 'worker' },
    ]);
  });

  it('drops an incomplete trailing drill', () => {
    expect(parseKubeHash('#/k8s/prod/shop/overview/Pod/shop')?.drills).toEqual([]);
  });

  it('reads cluster-scoped objects with the "-" namespace', () => {
    expect(parseKubeHash('#/k8s/prod/*/PersistentVolume/PersistentVolume/-/pv-1')?.drills[0])
      .toEqual({ level: 'object', namespace: '', kind: 'PersistentVolume', group: '', name: 'pv-1' });
  });
});

describe('toKubeHash', () => {
  it('writes every route shape back to the hash it was parsed from', () => {
    const hashes = [
      '#/k8s/prod/*/overview',
      '#/k8s/prod/shop/overview',
      '#/k8s/prod/*/nodes',
      '#/k8s/prod/*/nodes/Node/-/worker/Pod/kube-system/proxy-1',
      '#/k8s/prod/shop/storage',
      '#/k8s/prod/*/access',
      '#/k8s/prod/*/Pod',
      '#/k8s/prod/shop/Deployment.apps/Deployment.apps/shop/web/Pod/shop/web-1',
      '#/k8s/prod/*/Rollout.argoproj.io/Rollout.argoproj.io/shop/canary',
    ];
    for (const hash of hashes) {
      expect(toKubeHash(parseKubeHash(hash)!)).toBe(hash);
    }
  });

  it('encodes names that are not URL-safe and decodes them back', () => {
    const odd = route({
      cluster: 'arn:aws:eks/prod cluster',
      namespace: 'team a',
      drills: [{ level: 'object', namespace: 'team a', kind: 'Pod', group: '', name: 'web#1?x' }],
    });
    const hash = toKubeHash(odd);
    expect(hash).not.toContain(' ');
    expect(hash).not.toContain('#1');
    expect(parseKubeHash(hash)).toEqual(odd);
  });
});
