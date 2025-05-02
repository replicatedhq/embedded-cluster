import { Page, Expect } from '@playwright/test';
import { vaidateAppAndClusterReady } from '.';

export const deployApp = async (page: Page, expect: Expect) => {
  if (process.env.IS_MULTI_NODE_ENABLED === 'true') {
    await expect(page.getByText('Add nodes to the cluster')).toBeVisible();
    const joinCommand1 = await page.locator('.react-prism.language-bash').nth(0).textContent();
    const joinCommand2 = await page.locator('.react-prism.language-bash').nth(1).textContent();
    const joinCommand3 = await page.locator('.react-prism.language-bash').nth(2).textContent();
    expect(joinCommand1).toContain('curl');
    expect(joinCommand2).toContain('tar');
    expect(joinCommand3).toContain('sudo');
    await page.getByRole('button', { name: 'Continue' }).click();
  }
  await expect(page.locator('h3')).toContainText('The First Config Group');
  await page.locator('input[type="text"]').click();
  await page.locator('input[type="text"]').fill('initial-hostname.com');
  await page.locator('input[type="password"]').click();
  await page.locator('input[type="password"]').fill('password');
  await page.getByRole('button', { name: 'Continue' }).click();
  await expect(page.getByText('Validate the environment')).toBeVisible({ timeout: 10 * 1000 });
  await expect(page.getByRole('button', { name: 'Rerun' })).toBeVisible({ timeout: 10 * 1000 });
  await expect(page.locator('#app')).toContainText('The Volume Snapshots CRD exists');
  await page.getByRole('button', { name: 'Deploy' }).click();
  await vaidateAppAndClusterReady(page, expect, 90000);
};
