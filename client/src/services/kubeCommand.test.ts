import { describe, expect, it } from 'vitest';
import { resolveKubeCommand } from './kubeCommand';
import { KubeKind } from '../types/kubernetes';

const kind = (group: string, kindName: string, resource: string, shortNames?: string[]): KubeKind => ({
  group, version: 'v1', resource, kind: kindName, namespaced: true, shortNames,
});

// Discovery order: core group first
const KINDS: KubeKind[] = [
  kind('', 'Pod', 'pods', ['po']),
  kind('', 'Service', 'services', ['svc']),
  kind('apps', 'Deployment', 'deployments', ['deploy']),
  kind('example.com', 'PodAlias', 'podaliases', ['po']),
];

describe('resolveKubeCommand', () => {
  it('resolves short names, plural resources and kind names', () => {
    const deployment = { root: { type: 'kind', group: 'apps', kind: 'Deployment' } };
    expect(resolveKubeCommand('deploy', KINDS)).toEqual(deployment);
    expect(resolveKubeCommand('deployments', KINDS)).toEqual(deployment);
    expect(resolveKubeCommand('Deployment', KINDS)).toEqual(deployment);
    expect(resolveKubeCommand('  SVC ', KINDS)).toEqual({ root: { type: 'kind', group: '', kind: 'Service' } });
  });

  it('prefers the core kind when a CRD reuses its alias', () => {
    expect(resolveKubeCommand('po', KINDS)?.root).toEqual({ type: 'kind', group: '', kind: 'Pod' });
  });

  it('resolves the overview and lens words', () => {
    expect(resolveKubeCommand('overview', KINDS)?.root).toEqual({ type: 'overview' });
    expect(resolveKubeCommand('map', KINDS)?.root).toEqual({ type: 'overview' });
    for (const word of ['nodes', 'node', 'no']) {
      expect(resolveKubeCommand(word, KINDS)?.root).toEqual({ type: 'lens', lens: 'nodes' });
    }
    expect(resolveKubeCommand('storage', KINDS)?.root).toEqual({ type: 'lens', lens: 'storage' });
    expect(resolveKubeCommand('access', KINDS)?.root).toEqual({ type: 'lens', lens: 'access' });
  });

  it('switches namespace when one is given', () => {
    expect(resolveKubeCommand('po kube-system', KINDS)?.namespace).toBe('kube-system');
    expect(resolveKubeCommand('po all', KINDS)?.namespace).toBe('');
    expect(resolveKubeCommand('po *', KINDS)?.namespace).toBe('');
    expect(resolveKubeCommand('po', KINDS)).not.toHaveProperty('namespace');
  });

  it('returns null for empty or unknown commands', () => {
    expect(resolveKubeCommand('', KINDS)).toBeNull();
    expect(resolveKubeCommand('   ', KINDS)).toBeNull();
    expect(resolveKubeCommand('nope', KINDS)).toBeNull();
  });
});
