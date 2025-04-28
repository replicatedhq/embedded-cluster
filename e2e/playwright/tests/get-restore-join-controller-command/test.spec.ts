import { test, expect } from '@playwright/test';
import { login } from '../shared';

test('get restore join controller command', async ({ page }) => {
  await login(page);
  await page.getByRole('button', { name: 'Add node', exact: true }).click();
  await expect(page.locator('#controller-testNodeType')).toBeChecked();
  await expect(page.locator('.CodeSnippet-copy')).toBeVisible();
  const joinCommand1 = await page.locator('.react-prism.language-bash').nth(0).textContent();
  const joinCommand2 = await page.locator('.react-prism.language-bash').nth(1).textContent();
  const joinCommand3 = await page.locator('.react-prism.language-bash').nth(2).textContent();
  console.log(`{"command":"${joinCommand1} && ${joinCommand2} && ${joinCommand3}"}`);
});
