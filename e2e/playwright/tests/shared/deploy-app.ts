export const deployApp = async (page, expect) => {
  await expect(page.getByRole('button', { name: 'Add node', exact: true })).toBeVisible();
  await page.getByRole('button', { name: 'Continue' }).click();
  await expect(page.locator('h3')).toContainText('The First Config Group');
  await page.locator('input[type="text"]').click();
  await page.locator('input[type="text"]').fill('abc');
  await page.locator('input[type="password"]').click();
  await page.locator('input[type="password"]').fill('password');
  await page.getByRole('button', { name: 'Continue' }).click();
  await page.getByRole('button', { name: 'Deploy' }).click();
  await expect(page.locator('#app')).toContainText('Currently deployed version', { timeout: 90000 });
  await expect(page.locator('#app')).toContainText('Ready', { timeout: 30000 });
  await expect(page.locator('#app')).toContainText('Up to date');
};
