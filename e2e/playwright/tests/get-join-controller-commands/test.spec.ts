import { test, expect } from '@playwright/test';
import { login } from '../shared';

test('get join controller commands', async ({ page }) => {
  await login(page);
  await page.locator('.NavItem').getByText('Cluster Management', { exact: true }).click();
  await page.getByRole('button', { name: 'Add node', exact: true }).click();
  await expect(page.locator('.Modal-body')).toBeVisible();
  await expect(page.getByRole('heading')).toContainText('Add a Node');
  await expect(page.getByText('Roles:')).toBeVisible();
  await expect(page.locator('#controller-testNodeType')).toBeChecked();
  const joinCommand1 = await page.locator('.react-prism.language-bash').nth(0).textContent();
  const joinCommand2 = await page.locator('.react-prism.language-bash').nth(1).textContent();
  const joinCommand3 = await page.locator('.react-prism.language-bash').nth(2).textContent();
  console.log(JSON.stringify({commands: [joinCommand1, joinCommand2, joinCommand3]}));
});
