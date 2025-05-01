import { test, expect } from '@playwright/test';
import { login, deployEC23App } from '../shared';

test('deploy ec23 app', async ({ page }) => {
  test.setTimeout(2 * 60 * 1000); // 2 minutes
  await login(page);
  await deployEC23App(page, expect);
});
