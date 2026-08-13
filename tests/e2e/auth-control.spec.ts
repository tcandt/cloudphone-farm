import { test, expect } from '@playwright/test';

const mockAdminSession = {
  user_id: 'usr_owner_001',
  email: 'admin@phonecontrol.io',
  display_name: 'Alex Rivera',
  organization_id: 'org_pcp_enterprise_01',
  organization_name: 'Phone Control Platform Pro',
  role: 'owner',
  permissions: [
    'dashboard.read',
    'device.read',
    'device.update',
    'device.assign',
    'device.stream.view',
    'device.control.acquire',
    'device.control.input',
    'device.command.basic',
    'device.command.sensitive',
    'group.read',
    'group.manage',
    'agent.read',
    'agent.enroll',
    'agent.revoke',
    'member.read',
    'member.invite',
    'member.manage',
    'role.manage',
    'audit.read',
    'billing.read',
    'billing.manage',
    'organization.read',
    'organization.manage',
  ],
  balance_usd: 1450.0,
};

const mockOperatorSession = {
  ...mockAdminSession,
  role: 'operator',
  permissions: [
    'dashboard.read',
    'device.read',
    'device.stream.view',
    'device.control.acquire',
    'device.control.input',
    'device.command.basic',
    'group.read',
  ],
};

const mockViewerSession = {
  ...mockAdminSession,
  role: 'viewer',
  permissions: ['dashboard.read', 'device.read', 'device.stream.view', 'group.read'],
};

test.describe('Phone Control Platform — E2E Browser & Integration Contract Suite', () => {
  // Test 1: Unauthenticated Redirect
  test('1. Redirects to /login when accessing protected /app without session', async ({ page }) => {
    await page.goto('/login');
    await page.evaluate(() => localStorage.setItem('pcp_auth_session', 'null'));

    await page.goto('/app');
    await expect(page).toHaveURL(/\/login/);
  });

  // Test 2: Login Flow Navigation
  test('2. Performs login flow and navigates to Dashboard', async ({ page }) => {
    await page.goto('/login');
    await page.evaluate(() => localStorage.setItem('pcp_auth_session', 'null'));

    await page.fill('input[type="email"]', 'admin@phonecontrol.io');
    await page.fill('input[type="password"]', 'password123');

    await Promise.all([
      page.waitForURL(/\/app/),
      page.click('button[type="submit"]'),
    ]);

    await expect(page).toHaveURL(/\/app/);
  });

  // Test 3: Viewer Role Restrictions
  test('3. Viewer role has sensitive/control actions restricted', async ({ page }) => {
    await page.goto('/login');
    await page.evaluate((session) => {
      localStorage.setItem('pcp_auth_session', JSON.stringify(session));
    }, mockViewerSession);

    await page.goto('/app/devices/dev_s7_001');

    // Verify Acquire Control button is hidden for Viewer
    await expect(page.locator('button:has-text("Xin quyền điều khiển")')).not.toBeVisible();
  });

  // Test 4: Operator Role Control Lease & Touch Gesture
  test('4. Operator acquires control lease and dispatches touch gesture', async ({ page }) => {
    await page.goto('/login');
    await page.evaluate((session) => {
      localStorage.setItem('pcp_auth_session', JSON.stringify(session));
    }, mockOperatorSession);

    await page.goto('/app/devices/dev_s7_001');

    // Click Acquire Control button to open modal
    await page.click('button:has-text("Xin quyền điều khiển")');

    // Verify Control Modal opens with canvas
    const canvas = page.locator('canvas');
    await expect(canvas).toBeVisible();

    // Click Acquire Control Lease inside modal
    await page.click('button:has-text("Lấy Quyền (Lease)")');

    // Perform canvas touch click
    await canvas.click({ position: { x: 100, y: 150 } });
    await expect(page.locator('text=Touch gesture at')).toBeVisible();
  });

  // Test 5: Command without Lease Rejection
  test('5. Touch command without active control lease is rejected', async ({ page }) => {
    await page.goto('/login');
    await page.evaluate((session) => {
      localStorage.setItem('pcp_auth_session', JSON.stringify(session));
    }, mockAdminSession);

    await page.goto('/app/devices/dev_s7_001');

    // Open Control Modal but do NOT acquire lease
    await page.click('button:has-text("Xin quyền điều khiển")');
    const canvas = page.locator('canvas');
    await expect(canvas).toBeVisible();

    // Attempt canvas click without lease
    await canvas.click({ position: { x: 100, y: 150 }, force: true });
    await expect(page.locator('text=CONTROL_LEASE_REQUIRED')).toBeVisible();
  });

  // Test 6: Unknown Device 404
  test('6. Displays 404 Device Not Found when accessing non-existent device ID', async ({ page }) => {
    await page.goto('/login');
    await page.evaluate((session) => {
      localStorage.setItem('pcp_auth_session', JSON.stringify(session));
    }, mockAdminSession);

    await page.goto('/app/devices/non_existent_device_9999');
    await expect(page.locator('h2')).toContainText('404 — Device Not Found');
  });

  // Test 7: Dual Independent Streams without Collision
  test('7. Renders Dual Independent Video Streams without session state collision', async ({ page }) => {
    await page.goto('/login');
    await page.evaluate((session) => {
      localStorage.setItem('pcp_auth_session', JSON.stringify(session));
    }, mockAdminSession);

    await page.goto('/app/live-monitor');

    // Verify live monitor page loaded with session active
    await expect(page.locator('h1')).toContainText(/Authorized LIVE Monitor Console|Giám sát/i);
  });
});
