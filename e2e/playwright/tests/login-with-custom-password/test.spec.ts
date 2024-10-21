import { test, expect } from '@playwright/test';
import { login } from '../shared';

test('login with custom password', async ({ page }) => {
  test.setTimeout(30 * 1000); // 30 seconds
  await login(page, process.env.ADMIN_CONSOLE_PASSWORD);
  await expect(page.locator('.NavItem').getByText('Cluster Management')).toBeVisible();
});
