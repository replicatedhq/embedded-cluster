export const login = async (page) => {
  await page.goto('/');
  await page.getByPlaceholder('password').click();
  await page.getByPlaceholder('password').fill('password');
  await page.getByRole('button', { name: 'Log in' }).click();
  await page.waitForTimeout(3000); // sleep before reloading page
};
