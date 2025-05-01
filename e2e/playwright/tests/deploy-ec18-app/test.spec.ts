import { test, expect } from '@playwright/test';
import { login, deployEC18App } from '../shared';

test('deploy ec18 app version', async ({ page }) => {
  test.setTimeout(2 * 60 * 1000); // 2 minutes
  await login(page);
  await deployEC18App(page, expect);
});
