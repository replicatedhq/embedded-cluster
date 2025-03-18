import { Page, Expect } from '@playwright/test';

export const vaidateAppAndClusterReady = async (page: Page, expect: Expect, initialTimeout: number) => {
  await page.getByRole('link', { name: 'Dashboard', exact: true }).click();
  await expect(page.locator('#app')).toContainText('Currently deployed version', { timeout: initialTimeout });
  await expect(page.locator('#app')).toContainText('Ready', { timeout: 90000 });
  await expect(page.locator('#app')).toContainText('Up to date');
};
