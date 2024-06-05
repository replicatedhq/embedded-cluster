import { test, expect } from '@playwright/test';
import { login, deployApp } from '../shared';

test('create backup', async ({ page }) => {
  test.setTimeout(5 * 60 * 1000); // 5 minutes
  await login(page);
  await page.locator('.NavItem').getByText('Disaster Recovery', { exact: true }).click();
  await expect(page.getByText('Backup settings')).toBeVisible();
  await page.getByPlaceholder('Bucket name').click();
  await page.getByPlaceholder('Bucket name').fill(process.env.DR_AWS_S3_BUCKET);
  await page.getByPlaceholder('/path/to/destination').click();
  await page.getByPlaceholder('/path/to/destination').fill(process.env.DR_AWS_S3_PREFIX);
  await page.getByPlaceholder('key ID').click();
  await page.getByPlaceholder('key ID').fill(process.env.DR_AWS_ACCESS_KEY_ID);
  await page.getByPlaceholder('access key').click();
  await page.getByPlaceholder('access key').fill(process.env.DR_AWS_SECRET_ACCESS_KEY);
  await page.getByPlaceholder('http[s]://hostname[:port]').click();
  await page.getByPlaceholder('http[s]://hostname[:port]').fill(process.env.DR_AWS_S3_ENDPOINT);
  await page.getByPlaceholder('us-east-1').click();
  await page.getByPlaceholder('us-east-1').fill(process.env.DR_AWS_S3_REGION);
  await page.getByRole('button', { name: 'Update storage settings' }).click();
  await expect(page.locator('.Loader')).toBeVisible();
  await expect(page.getByRole('button', { name: 'Updating', exact: true })).toBeDisabled();
  await expect(page.getByRole('button', { name: 'Update storage settings' })).not.toBeVisible();
  await expect(page.locator('form')).toContainText('Settings updated', { timeout: 60000 });
  await expect(page.locator('.Loader')).not.toBeVisible();
  await expect(page.getByRole('button', { name: 'Update storage settings' })).toBeEnabled();
  await page.locator('.subnav-item').getByText('Backups', { exact: true }).click();
  await expect(page.locator('#app')).toContainText('No backups yet');
  await page.getByRole('button', { name: 'Start a backup' }).click();
  await expect(page.locator('#app')).toContainText('In Progress');
  await expect(page.locator('#app')).toContainText('Completed', { timeout: 300000 });
});
