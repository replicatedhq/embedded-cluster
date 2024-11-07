import { Page } from '@playwright/test';

export const login = async (page: Page, password = 'password') => {
  await page.goto('/');
  await page.getByPlaceholder('password').click();
  await page.getByPlaceholder('password').fill(password);
  await page.getByRole('button', { name: 'Log in' }).click();
};
