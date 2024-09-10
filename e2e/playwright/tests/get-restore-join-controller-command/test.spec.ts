import { test, expect } from '@playwright/test';
import { login } from '../shared';

test('get restore join controller command', async ({ page }) => {
  await login(page);
  await expect(page.getByText('Nodes')).toBeVisible();
  await expect(page.getByText('Select one or more roles to assign to the new node')).toBeVisible();
  await expect(page.getByText('Roles:')).toBeVisible();
  await expect(page.locator('#controller-testNodeType')).toBeChecked();
  const joinCommand = await page.locator('.react-prism.language-bash').first().textContent();
  console.log(`{"command":"${joinCommand}"}`);
});
