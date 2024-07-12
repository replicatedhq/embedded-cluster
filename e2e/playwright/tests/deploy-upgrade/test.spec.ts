import { test, expect } from '@playwright/test';
import { login } from '../shared';

test('deploy upgrade', async ({ page }) => {
  test.setTimeout(15 * 60 * 1000); // 15 minutes
  await login(page);
  await page.getByRole('link', { name: 'Version history', exact: true }).click();
  // TODO NOW: uncomment this
  // await page.locator('.available-update-row', { hasText: process.env.APP_UPGRADE_VERSION }).getByRole('button', { name: 'Deploy', exact: true }).click();
  // TODO NOW: remove this line
  await page.getByText(process.env.APP_UPGRADE_VERSION, { exact: true }).locator('../../..').getByRole('button', { name: 'Deploy', exact: true }).click();
  const iframe = page.frameLocator('#upgrade-service-iframe');
  await expect(iframe.locator('h3')).toContainText('The First Config Group', { timeout: 20 * 1000 });
  await expect(iframe.locator('input[type="text"]')).toHaveValue('initial-hostname.com');
  await iframe.locator('input[type="text"]').click();
  await iframe.locator('input[type="text"]').fill('updated-hostname.com');
  await iframe.locator('input[type="password"]').click();
  await iframe.locator('input[type="password"]').fill('updated password');
  await iframe.getByRole('button', { name: 'Next', exact: true }).click();
  await expect(iframe.getByText('Preflight checks', { exact: true })).toBeVisible({ timeout: 10 * 1000 });
  await expect(iframe.getByRole('button', { name: 'Re-run' })).toBeVisible({ timeout: 10 * 1000 });
  await expect(iframe.locator('#app')).toContainText('Embedded Cluster Installation CRD exists');
  await expect(iframe.locator('#app')).toContainText('Embedded Cluster Config CRD exists');
  await expect(iframe.getByRole('button', { name: 'Back: Config' })).toBeVisible();
  await iframe.getByRole('button', { name: 'Next: Confirm and deploy' }).click();
  await expect(iframe.locator('#app')).toContainText('All preflight checks passed');
  await expect(iframe.getByRole('button', { name: 'Back: Preflight checks' })).toBeVisible();
  await iframe.getByRole('button', { name: 'Deploy', exact: true }).click();

  if (process.env.SKIP_CLUSTER_UPGRADE_CHECK !== 'true') {
    await expect(page.locator('.Modal-body')).toContainText('Cluster update in progress');
    await expect(page.locator('.Modal-body').getByText('Cluster update in progress')).not.toBeVisible({ timeout: 10 * 60 * 1000 });
  }

  await expect(page.locator('.available-update-row', { hasText: process.env.APP_UPGRADE_VERSION })).not.toBeVisible({ timeout: 10 * 1000 });
  await expect(page.locator('.VersionHistoryRow', { hasText: process.env.APP_UPGRADE_VERSION })).toContainText('Currently deployed version', { timeout: 30 * 1000 });
  await page.getByRole('link', { name: 'Dashboard', exact: true }).click();
  await expect(page.locator('.VersionCard-content--wrapper')).toContainText(process.env.APP_UPGRADE_VERSION);
  await expect(page.locator('#app')).toContainText('Currently deployed version');
  await expect(page.locator('#app')).toContainText('Ready', { timeout: 30 * 1000 });
  await expect(page.locator('#app')).toContainText('Up to date');
});
