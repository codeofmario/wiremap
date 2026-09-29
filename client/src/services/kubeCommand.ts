import { KubeKind, KubeRoot } from '../types/kubernetes';

export interface KubeCommand {
  root: KubeRoot;
  /** Namespace to switch to; "" for all, absent to keep the current one */
  namespace?: string;
}

const ROOT_WORDS: Record<string, KubeRoot> = {
  overview: { type: 'overview' },
  map: { type: 'overview' },
  nodes: { type: 'lens', lens: 'nodes' },
  node: { type: 'lens', lens: 'nodes' },
  no: { type: 'lens', lens: 'nodes' },
  storage: { type: 'lens', lens: 'storage' },
  access: { type: 'lens', lens: 'access' },
};

const kindNames = (k: KubeKind) => [k.kind.toLowerCase(), k.resource, ...(k.shortNames || [])];

/**
 * Resolves a command such as "po", "deploy kube-system" or "svc all"
 * to what the map should show. Returns null for unknown commands.
 */
export const resolveKubeCommand = (input: string, kinds: KubeKind[]): KubeCommand | null => {
  const [word, namespace] = input.trim().toLowerCase().split(/\s+/);
  if (!word) return null;

  // Kinds are sorted core group first, so "po" prefers core Pods over any CRD alias
  const kind = kinds.find((k) => kindNames(k).includes(word));
  const root = ROOT_WORDS[word] || (kind ? { type: 'kind' as const, group: kind.group, kind: kind.kind } : null);
  if (!root) return null;

  if (namespace === undefined) return { root };
  return { root, namespace: namespace === 'all' || namespace === '*' ? '' : namespace };
};
