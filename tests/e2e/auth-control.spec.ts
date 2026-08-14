import { test, expect } from '@playwright/test';

const mockAdminSession = {
  user_id: 'usr_owner_01',
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
    await expect(page.locator('text=Active')).toBeVisible();

    // Perform canvas touch click at center of content area
    await canvas.click();
    await expect(page.locator('text=Touch accepted at')).toBeVisible();
  });

  // Test 5: Command without Lease Rejection & UI Lock Verification
  test('5. Control lock overlay blocks unleased control and rejects command execution', async ({ page }) => {
    await page.goto('/login');
    await page.evaluate((session) => {
      localStorage.setItem('pcp_auth_session', JSON.stringify(session));
    }, mockAdminSession);

    await page.goto('/app/devices/dev_s7_001');

    // Open Control Modal but do NOT acquire lease
    await page.click('button:has-text("Xin quyền điều khiển")');
    const canvas = page.locator('canvas');
    await expect(canvas).toBeVisible();

    // Verify Interactive Control Lock UI overlay is present
    await expect(page.locator('text=Interactive Control Lock')).toBeVisible();
    await expect(page.locator('button:has-text("Home")')).toBeDisabled();

    // Click lock overlay backdrop
    await page.locator('text=Interactive Control Lock').click();
    await expect(page.locator('text=Interactive Control Lock')).toBeVisible();
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

  // Test 7: Dual Independent Streams without Session Collision
  test('7. Renders Dual Independent Video Streams and verifies independent lifecycle', async ({ page }) => {
    await page.goto('/login');
    await page.evaluate((session) => {
      localStorage.setItem('pcp_auth_session', JSON.stringify(session));
    }, mockAdminSession);

    await page.goto('/app/live-monitor');
    await expect(page.locator('h1')).toContainText(/Authorized LIVE Monitor Console/i);

    // Verify two active canvases exist
    const canvases = page.locator('canvas');
    await expect(canvases).toHaveCount(2);

    // Retrieve session IDs of both independent streams
    const sessionId1 = await canvases.nth(0).getAttribute('data-session-id');
    const sessionId2 = await canvases.nth(1).getAttribute('data-session-id');

    expect(sessionId1).toBeTruthy();
    expect(sessionId2).toBeTruthy();
    expect(sessionId1).not.toBe(sessionId2);

    // Stop stream on Card 1
    const stopButtons = page.locator('button[title="Stop Stream"]');
    await stopButtons.nth(0).click();

    // Verify Card 1 is stopped while Card 2 canvas remains active
    await expect(canvases).toHaveCount(1);
    const remainingSessionId = await canvases.nth(0).getAttribute('data-session-id');
    expect(remainingSessionId).toBe(sessionId2);
  });
});
