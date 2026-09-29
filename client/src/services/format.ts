/** Relative age of an ISO timestamp, kubectl style: 45s, 3m, 2h, 5d. */
export const formatAge = (iso: string): string => {
  const seconds = Math.max(0, Math.round((Date.now() - new Date(iso).getTime()) / 1000));
  if (seconds < 60) return `${seconds}s`;
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m`;
  if (seconds < 86400) return `${Math.floor(seconds / 3600)}h`;
  return `${Math.floor(seconds / 86400)}d`;
};

/** "Deployment" → "Deployments", "Ingress" → "Ingresses", "NetworkPolicy" → "NetworkPolicies", "Endpoints" stays */
export const pluralKind = (kind: string): string => {
  if (/(ss|us|x|ch|sh)$/.test(kind)) return `${kind}es`;
  // Kinds already named in the plural, e.g. Endpoints
  if (kind.endsWith('s')) return kind;
  if (/[^aeiou]y$/.test(kind)) return `${kind.slice(0, -1)}ies`;
  return `${kind}s`;
};
