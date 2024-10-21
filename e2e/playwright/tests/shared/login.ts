export const login = async (page, password = 'password') => {
  await page.goto('/');
  await page.getByPlaceholder('password').click();
  await page.getByPlaceholder('password').fill(password);
  await page.getByRole('button', { name: 'Log in' }).click();
};
