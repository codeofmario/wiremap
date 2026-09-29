import { KubeRoot, KubeView } from '../types/kubernetes';

const PREFIX = '#/k8s/';
const ALL_NAMESPACES = '*';
const NO_NAMESPACE = '-';
const LENSES = ['nodes', 'storage', 'access'] as const;

export interface KubeRoute {
  cluster: string;
  /** "" for all namespaces */
  namespace: string;
  root: KubeRoot;
  /** Objects and nodes opened from the root, outermost first */
  drills: KubeView[];
}

export const defaultRoute = (cluster: string): KubeRoute => ({ cluster, namespace: '', root: { type: 'overview' }, drills: [] });

const kindSegment = (kind: string, group?: string) => (group ? `${kind}.${group}` : kind);

const splitKind = (segment: string) => {
  const [kind, ...group] = segment.split('.');
  return { kind, group: group.join('.') };
};

/**
 * Hash routes: #/k8s/{cluster}/{namespace|*}/{root}[/{Kind[.group]}/{namespace|-}/{name}]...
 *
 * The root is what the sidebar selected: `overview`, a lens (`nodes`, `storage`,
 * `access`) or a kind such as `Deployment.apps`. Each following triple opens
 * one object one level deeper, e.g. Deployment → Pod; `Node/-/{name}` opens a node.
 */
export const parseKubeHash = (hash: string): KubeRoute | null => {
  if (!hash.startsWith(PREFIX)) return null;
  const [cluster, namespace = ALL_NAMESPACES, rootSegment = 'overview', ...rest] = hash.slice(PREFIX.length).split('/').map(decodeURIComponent);
  if (!cluster) return null;

  const lens = LENSES.find((l) => l === rootSegment);
  const root: KubeRoot = lens
    ? { type: 'lens', lens }
    : rootSegment === 'overview' ? { type: 'overview' } : { type: 'kind', ...splitKind(rootSegment) };

  const drills: KubeView[] = [];
  for (let i = 0; i + 2 < rest.length; i += 3) {
    const { kind, group } = splitKind(rest[i]);
    const objectNamespace = rest[i + 1] === NO_NAMESPACE ? '' : rest[i + 1];
    const name = rest[i + 2];
    drills.push(kind === 'Node' && !group
      ? { level: 'node', name }
      : { level: 'object', namespace: objectNamespace, kind, group, name });
  }

  return { cluster, namespace: namespace === ALL_NAMESPACES ? '' : namespace, root, drills };
};

export const toKubeHash = ({ cluster, namespace, root, drills }: KubeRoute): string => {
  const rootSegment = root.type === 'overview' ? 'overview' : root.type === 'lens' ? root.lens : kindSegment(root.kind, root.group);
  const segments = [cluster, namespace || ALL_NAMESPACES, rootSegment];
  for (const view of drills) {
    if (view.level === 'node') segments.push('Node', NO_NAMESPACE, view.name || '');
    else segments.push(kindSegment(view.kind || '', view.group), view.namespace || NO_NAMESPACE, view.name || '');
  }
  return PREFIX + segments.map(encodeURIComponent).join('/');
};
