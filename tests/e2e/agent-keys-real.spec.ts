import { test, expect } from '@playwright/test';
import { execSync } from 'child_process';
import path from 'path';

test.describe('Phase 2 Final Gate - Real Backend E2E', () => {
  let rawToken: string;
  let tokenHash: string;

  test.beforeAll(() => {
    const out = execSync('go run ./cmd/e2e-session-seed/main.go seed', {
      cwd: path.resolve(process.cwd(), 'backend'),
      encoding: 'utf-8'
    });
    const parsed = JSON.parse(out);
    rawToken = parsed.raw_token;
    tokenHash = parsed.token_hash;
    console.log('session fixture created');
  });

  test.afterAll(() => {
    execSync(`go run ./cmd/e2e-session-seed/main.go cleanup ${tokenHash}`, {
      cwd: path.resolve(process.cwd(), 'backend')
    });
    console.log('session fixture removed');
  });

  test.beforeEach(async ({ page }) => {
    await page.evaluate(() => localStorage.clear()).catch(() => {});
  });

  test('Canonical Flow: Create, List, Edit, View, Revoke', async ({ page, context }) => {
    // 1. no cookie => V2 API returns 401
    await page.goto('/login');
    const noCookieStatus = await page.evaluate(async () => {
      const res = await fetch('/api/v2/agent-keys');
      return res.status;
    });
    expect(noCookieStatus).toBe(401);

    // 3. set real session cookie
    await context.addCookies([{
      name: '__Host-pcp_session',
      value: rawToken,
      domain: 'localhost',
      path: '/',
      secure: true,
      httpOnly: true
    }]);

    // Prove it works with the cookie
    const withCookieStatus = await page.evaluate(async () => {
      const res = await fetch('/api/v2/agent-keys');
      return res.status;
    });
    expect(withCookieStatus).toBe(200);

    // 4. open Token Keys UI
    await page.goto('/login');
    await page.evaluate(() => {
      localStorage.setItem('pcp_api_mode', 'http');
      // Provide UI bootstrap metadata only
      localStorage.setItem('pcp_auth_session', JSON.stringify({
        user_id: 'user_e2e_phase2',
        email: 'admin_e2e@phonecontrol.io',
        role: 'owner',
        permissions: ['agent.enroll', 'agent.revoke', 'agent.read'],
      }));
    });

    let v1Calls = 0;
    page.on('request', req => {
      if (req.url().includes('/api/v1/enrollment-tokens')) {
        v1Calls++;
      }
    });

    await page.goto('/app/agents');
    await expect(page.getByText('Danh sách Token Keys')).toBeVisible();

    // 5. Create Key
    await page.getByRole('button', { name: 'Tạo Token Key Mới' }).click();
    await page.fill('input[placeholder="Ví dụ: Farm Server 01"]', 'E2E Real Backend Key');
    await page.getByRole('button', { name: 'Tạo Token', exact: true }).click();

    // 6. raw_secret visible exactly once
    await expect(page.getByText('Tạo Token Thành Công')).toBeVisible();
    const secretInput = page.locator('input[readonly]');
    await expect(secretInput).toBeVisible();
    const rawSecret = await secretInput.inputValue();
    expect(rawSecret).toContain('cpk_');

    // 7. acknowledge close
    await page.getByRole('button', { name: 'Đã lưu & Đóng' }).click();
    await expect(page.getByText('Tạo Token Thành Công')).not.toBeVisible();

    // 8. reload
    await page.reload();
    await expect(page.getByText('Danh sách Token Keys')).toBeVisible();

    // 9. raw_secret cannot be recovered
    await expect(page.getByText('E2E Real Backend Key')).toBeVisible();
    await expect(page.locator('text=' + rawSecret)).not.toBeVisible();

    // 10. Edit
    const row = page.locator('tr').filter({ hasText: 'E2E Real Backend Key' }).first();
    await row.locator('button[title="Chỉnh sửa Token"]').click();
    await expect(page.getByText('Sửa Token Key')).toBeVisible();
    await page.fill('input[type="text"]', 'E2E Real Backend Key Updated');
    await page.getByRole('button', { name: 'Lưu thay đổi' }).click();
    
    // Using a more resilient check for update (Wait for success toast or UI update)
    await expect(page.getByText('E2E Real Backend Key Updated')).toBeVisible();

    // 11. View bindings
    await row.locator('button[title="Xem thiết bị đã kết nối"]').click();
    await expect(page.getByText('Lịch sử thiết bị')).toBeVisible();
    await page.locator('.fixed.inset-0').first().click({ position: { x: 10, y: 10 } });

    // 12. Revoke
    await row.locator('button[title="Thu hồi Token"]').click();
    await expect(page.getByText('Thu hồi Token Key', { exact: true })).toBeVisible();
    await page.getByRole('button', { name: 'Đồng ý thu hồi', exact: true }).click();
    
    // 13. key remains listed REVOKED
    await expect(row.getByText('Đã thu hồi')).toBeVisible();

    // 14. bindings remain readable
    await row.locator('button[title="Xem thiết bị đã kết nối"]').click();
    await expect(page.getByText('Lịch sử thiết bị')).toBeVisible();
    await page.locator('.fixed.inset-0').first().click({ position: { x: 10, y: 10 } });

    // 15. no Unrevoke action
    await expect(row.locator('button[title="Chỉnh sửa Token"]')).toBeHidden();
    await expect(row.locator('button[title="Thu hồi Token"]')).toBeHidden();

    // 16. assert ZERO /api/v1/enrollment-tokens calls
    expect(v1Calls).toBe(0);
  });
});
