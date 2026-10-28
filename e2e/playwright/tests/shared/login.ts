import { Page, expect, errors } from '@playwright/test';

export const login = async (page: Page, password = 'password') => {
  // CMX ingress can stall an asset request even after the HTML loads. Reload
  // before that request consumes the entire test timeout.
  for (let attempt = 1; attempt <= 3; attempt++) {
    try {
      await page.goto('/', { timeout: 30_000 });
      break;
    } catch (error) {
      if (!(error instanceof errors.TimeoutError) || attempt === 3) {
        throw error;
      }
      console.log(`Login page navigation timed out (attempt ${attempt}/3), retrying`);
    }
  }
  await page.getByPlaceholder('password').click();
  await page.getByPlaceholder('password').fill(password);
  await page.getByRole('button', { name: 'Log in' }).click();
  await expect(page.locator('#app')).not.toContainText('Log in', { timeout: 10 * 1000 });
};
