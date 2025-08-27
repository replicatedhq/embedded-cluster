import { test, expect } from '@playwright/test';
import { login, deployApp } from '../shared';

test('create backup', async ({ page }) => {
  test.setTimeout(5 * 60 * 1000); // 5 minutes
  await login(page);
  await page.locator('.NavItem').getByText('Disaster Recovery', { exact: true }).click();
  await expect(page.getByText('Backup settings')).toBeVisible();
  await page.getByPlaceholder('Bucket name').click();
  await page.getByPlaceholder('Bucket name').fill(process.env.DR_S3_BUCKET);
  await page.getByPlaceholder('/path/to/destination').click();
  await page.getByPlaceholder('/path/to/destination').fill(process.env.DR_S3_PREFIX);
  await page.getByPlaceholder('key ID').click();
  await page.getByPlaceholder('key ID').fill(process.env.DR_ACCESS_KEY_ID);
  await page.getByPlaceholder('access key').click();
  await page.getByPlaceholder('access key').fill(process.env.DR_SECRET_ACCESS_KEY);
  await page.getByPlaceholder('http[s]://hostname[:port]').click();
  await page.getByPlaceholder('http[s]://hostname[:port]').fill(process.env.DR_S3_ENDPOINT);
  await page.getByPlaceholder('us-east-1').click();
  await page.getByPlaceholder('us-east-1').fill(process.env.DR_S3_REGION);

  const storageCard = page.locator('[data-testid="snapshots-storage-settings-card"]');
  await expect(storageCard.getByRole('button', { name: 'Update storage settings' })).toBeVisible();
  await storageCard.getByRole('button', { name: 'Update storage settings' }).click();
  await expect(storageCard.locator('.Loader')).toBeVisible();
  await expect(storageCard.getByRole('button', { name: 'Updating', exact: true })).toBeDisabled();
  await expect(storageCard.getByRole('button', { name: 'Update storage settings' })).not.toBeVisible();
  await expect(storageCard.locator('form')).toContainText('Settings updated', { timeout: 90000 });
  await expect(storageCard.locator('.Loader')).not.toBeVisible();
  await expect(storageCard.getByRole('button', { name: 'Update storage settings' })).toBeEnabled();

  await page.locator('.subnav-item').getByText('Backups', { exact: true }).click();
  await expect(page.locator('#app')).toContainText('No backups yet');
  await page.getByRole('button', { name: 'Start a backup' }).click();
  await expect(page.locator('#app')).toContainText('In Progress');
  await expect(page.locator('#app')).toContainText('Completed', { timeout: 300000 });
});
