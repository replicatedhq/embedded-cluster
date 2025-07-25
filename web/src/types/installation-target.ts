export const installationTargets = ['linux', 'kubernetes'] as const;

export type InstallationTarget = typeof installationTargets[number];

export function isInstallationTarget(value: string): value is InstallationTarget {
  return (installationTargets as readonly string[]).includes(value);
}
