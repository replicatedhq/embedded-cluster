import { test, expect } from '@playwright/test';
import { login } from '../shared';

test('get join controller command', async ({ page }) => {
  await login(page);
  await page.locator('.NavItem').getByText('Cluster Management', { exact: true }).click();
  await expect(page.getByText("Optionally add nodes to the cluster"),).toBeVisible();
  await expect(page.getByText("Roles:")).toBeVisible();
  await page.getByText("controller-test", { exact: true }).toBeChecked()
  const joinCommand = await page.locator('.react-prism.language-bash').first().textContent();
  console.log(`{"command":"${joinCommand}"}`);
});
