import { test, expect } from '@playwright/test';
import { login, vaidateAppAndClusterReady } from '../shared';

test('validate restore app', async ({ page }) => {
  test.setTimeout(5 * 60 * 1000); // 5 minutes
  await login(page);
  await vaidateAppAndClusterReady(page, expect, 90 * 1000);
});
