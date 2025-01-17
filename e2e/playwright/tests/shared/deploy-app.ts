import { Page, Expect } from '@playwright/test';

export const deployApp = async (page: Page, expect: Expect) => {
  await expect(page.getByText('Optionally add nodes to the cluster'),).toBeVisible();
  await expect(page.locator('.react-prism.language-bash')).toBeVisible();
  const joinCommand = await page.locator('.react-prism.language-bash').first().textContent();
  await expect(joinCommand).toContain('sudo');
  await page.getByRole('button', { name: 'Continue' }).click();
  await expect(page.locator('h3')).toContainText('The First Config Group');
  await page.locator('input[type="text"]').click();
  await page.locator('input[type="text"]').fill('initial-hostname.com');
  await page.locator('input[type="password"]').click();
  await page.locator('input[type="password"]').fill('password');
  await page.getByRole('button', { name: 'Continue' }).click();
  await expect(page.getByText('Validate the environment')).toBeVisible({ timeout: 10 * 1000 });
  await expect(page.getByRole('button', { name: 'Rerun' })).toBeVisible({ timeout: 10 * 1000 });
  await expect(page.locator('#app')).toContainText('Embedded Cluster Installation CRD exists');
  await expect(page.locator('#app')).toContainText('Embedded Cluster Config CRD exists');
  await page.getByRole('button', { name: 'Deploy' }).click();
  await expect(page.locator('#app')).toContainText('Currently deployed version', { timeout: 90000 });
  await expect(page.locator('#app')).toContainText('Ready', { timeout: 45000 });
  await expect(page.locator('#app')).toContainText('Up to date');
};