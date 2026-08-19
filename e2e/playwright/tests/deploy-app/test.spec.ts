import { test, expect } from '@playwright/test';
import { login, deployApp } from '../shared';

test('deploy app', async ({ page }) => {
  test.setTimeout(5 * 60 * 1000); // 5 minutes - admin console can be slow after long setup
  await login(page);
  await deployApp(page, expect);
});
