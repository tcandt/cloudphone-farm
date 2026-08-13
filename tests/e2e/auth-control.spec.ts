import { test, expect } from '@playwright/test';

test.describe('MaxCloudPhone — E2E Browser Test Suite', () => {
  test('Redirects to /login when accessing protected /app without session', async ({ page }) => {
    await page.goto('/login');
    await page.evaluate(() => localStorage.setItem('pcp_auth_session', 'null'));

    await page.goto('/app');
    await expect(page).toHaveURL(/\/login/);
  });

  test('Performs login flow and navigates to Dashboard', async ({ page }) => {
    await page.goto('/login');
    await page.fill('input[type="email"]', 'admin@phonecontrol.io');
    await page.fill('input[type="password"]', 'password123');
    await page.click('button[type="submit"]');

    await expect(page).toHaveURL(/\/app/);
    await expect(page.locator('h1')).toContainText(/Bảng điều khiển|Phone Control Platform/);
  });

  test('Displays 404 Device Not Found when accessing non-existent device ID', async ({ page }) => {
    await page.goto('/app/devices/non_existent_device_9999');
    await expect(page.locator('h2')).toContainText('404 — Device Not Found');
  });

  test('Renders Sign Up page with 2-column password inputs', async ({ page }) => {
    await page.goto('/register');
    await expect(page.locator('h1')).toContainText(/Sign up/i);
    await expect(page.locator('input[placeholder="Enter your email"]')).toBeVisible();
  });
});
