export const wizardModes = ["install", "upgrade"] as const;

// WizardMode tells us in which mode the installer wizard is running, upgrade or install
export type WizardMode = (typeof wizardModes)[number];

export function isWizardMode(value: string): value is WizardMode {
  return (wizardModes as readonly string[]).includes(value);
}
