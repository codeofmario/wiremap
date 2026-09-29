import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { formatAge, pluralKind } from './format';

describe('pluralKind', () => {
  it.each([
    ['Deployment', 'Deployments'],
    ['Pod', 'Pods'],
    ['Ingress', 'Ingresses'],
    ['StorageClass', 'StorageClasses'],
    ['NetworkPolicy', 'NetworkPolicies'],
    ['Gateway', 'Gateways'],
    ['Box', 'Boxes'],
    ['Patch', 'Patches'],
    ['Mesh', 'Meshes'],
    ['Endpoints', 'Endpoints'],
    ['ComponentStatus', 'ComponentStatuses'],
  ])('%s → %s', (kind, plural) => {
    expect(pluralKind(kind)).toBe(plural);
  });
});

describe('formatAge', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-01-10T12:00:00Z'));
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it.each([
    ['2026-01-10T11:59:30Z', '30s'],
    ['2026-01-10T11:15:00Z', '45m'],
    ['2026-01-10T07:00:00Z', '5h'],
    ['2026-01-07T12:00:00Z', '3d'],
    ['2026-01-10T12:00:05Z', '0s'],
  ])('%s → %s', (iso, age) => {
    expect(formatAge(iso)).toBe(age);
  });
});
