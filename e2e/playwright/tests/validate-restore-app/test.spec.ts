import { test, expect } from '@playwright/test';
import { vaidateAppAndClusterReady } from '../shared';

test('validate restore app', async ({ page }) => {
  await vaidateAppAndClusterReady(page, expect, 90 * 1000);
});
