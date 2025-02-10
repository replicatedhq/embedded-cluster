import { Page, Expect } from '@playwright/test';
import { vaidateAppAndClusterReady } from '.';

export const deployEC18AppVersion = async (page: Page, expect: Expect) => {
  await expect(page.getByRole('button', { name: 'Add node', exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Continue' }).click();
  await expect(page.locator('h3')).toContainText('The First Config Group');
  await page.locator('input[type="text"]').click();
  await page.locator('input[type="text"]').fill('initial-hostname.com');
  await page.locator('input[type="password"]').click();
  await page.locator('input[type="password"]').fill('password');
  await page.getByRole('button', { name: 'Continue' }).click();
  await expect(page.getByText('Preflight checks', { exact: true })).toBeVisible({ timeout: 10 * 1000 });
  await expect(page.getByRole('button', { name: 'Re-run' })).toBeVisible({ timeout: 10 * 1000 });
  await expect(page.locator('#app')).toContainText('Embedded Cluster Installation CRD exists');
  await expect(page.locator('#app')).toContainText('Embedded Cluster Config CRD exists');
  await page.getByRole('button', { name: 'Deploy' }).click();
  await vaidateAppAndClusterReady(page, expect, 90000);
};