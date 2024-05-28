import { test, expect } from '@playwright/test';
import { login, deployApp } from '../shared';

test('deploy app', async ({ page }) => {
  test.setTimeout(2 * 60 * 1000); // 2 minutes
  await login(page);
  await deployApp(page, expect);
});
