import { Page, expect } from '@playwright/test';

export const login = async (page: Page, password = 'password') => {
  await page.goto('/');
  await page.getByPlaceholder('password').click();
  await page.getByPlaceholder('password').fill(password);
  await page.getByRole('button', { name: 'Log in' }).click();
  // Wait for the login overlay to disappear before asserting the logged-in UI.
  await expect(page.getByPlaceholder('password')).toBeHidden({ timeout: 60 * 1000 });
  await expect(page.locator('#app')).not.toContainText('Log in', { timeout: 10 * 1000 });
};
