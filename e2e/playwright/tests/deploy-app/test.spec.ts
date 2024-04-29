import { test, expect } from '@playwright/test';
import { login, deployApp } from '../shared';

test('deploy app', async ({ page }) => {
  test.setTimeout(60 * 1000); // 1 minute
  await login(page);
  await deployApp(page, expect);
});
