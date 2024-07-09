import { test, expect } from '@playwright/test';
import { login } from '../shared';

test('deploy upgrade', async ({ page }) => {
  test.setTimeout(15 * 60 * 1000); // 15 minutes
  await login(page);
  await page.getByRole('link', { name: 'Version history', exact: true }).click();
  const rowLocator = '../../..'; // '//./ancestor::div[@class="available-update-row"]'
  await page.getByText(process.env.APP_UPGRADE_VERSION, { exact: true })
    .locator(rowLocator)
    .getByRole('button', { name: 'Deploy', exact: true }).click();
  const iframe = page.frameLocator('#upgrade-service-iframe');
  await expect(iframe.locator('.ConfigArea--wrapper')).toBeVisible({ timeout: 20 * 1000 });
  await iframe.getByRole('button', { name: 'Next', exact: true }).click();
  await iframe.getByRole('button', { name: 'Next: Confirm and deploy', exact: true }).click({ timeout: 10 * 1000 });
  await iframe.getByRole('button', { name: 'Deploy', exact: true }).click();
  await expect(page.locator('.Modal-body').getByText('Cluster update in progress')).toBeVisible();
  await expect(page.locator('.Modal-body').getByText('Cluster update in progress')).not.toBeVisible({ timeout: 60 * 1000 });
  await page.getByRole('link', { name: 'Dashboard', exact: true }).click();
  await expect(page.locator('.VersionCard-content--wrapper')).toContainText(process.env.APP_UPGRADE_VERSION);
});
