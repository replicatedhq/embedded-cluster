import { test, expect } from '@playwright/test';
import { login } from '../shared';

test('deploy airgap upgrade', async ({ page }) => {
  test.setTimeout(15 * 60 * 1000); // 15 minutes
  await login(page);
  await expect(page.locator('#app')).toContainText('Airgap Update');
  await page.getByRole('button', { name: 'Deploy', exact: true }).click();
  await expect(page.locator('.Modal-body')).toBeVisible();
  await page.getByRole('button', { name: 'Yes, Deploy' }).click();
  await expect(page.locator('#app')).toContainText('Updating cluster', { timeout: 90000 });
  if (process.env.SKIP_CLUSTER_UPGRADING_CHECK !== 'true') {
    await expect(page.locator('.Modal-body')).toContainText('Cluster update in progress', { timeout: 120000 });
  }
  await expect(page.locator('#app')).toContainText('Currently deployed version', { timeout: 600000 });
  await expect(page.locator('#app')).toContainText('Up to date', { timeout: 30000 });
  await expect(page.locator('#app')).toContainText('Ready');
});
