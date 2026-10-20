import { expect, test, type Page } from '@playwright/test';
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

const adminUsername = 'storage-reactive-admin';
const adminPassword = 'StorageReactive-password-2026';
const recoveryKey = 'ABCDE-FGHKL-MNPQR-STUVW-XYZ23-45678';

const testUsername = 'quota-user-test';
const testPassword = 'QuotaPassword123!';

async function setupAndLogin(page: Page) {
	const status = await page.request.get('/api/v1/setup/status');
	expect(status.ok()).toBeTruthy();
	const statusJson = (await status.json()) as { is_setup_completed: boolean };
	if (!statusJson.is_setup_completed) {
		const scan = await page.request.get('/api/v1/setup/scan-nodes');
		expect(scan.ok()).toBeTruthy();
		const { nodes } = (await scan.json()) as { nodes: { path: string; capacity_bytes: number }[] };
		expect(nodes.length).toBeGreaterThan(0);
		const setup = await page.request.post('/api/v1/setup/complete', {
			data: {
				admin: { username: adminUsername, email: 'admin-reactive@example.test', password: adminPassword, recovery_key: recoveryKey },
				storage_nodes: nodes.map((node) => ({ ...node, enabled: true })),
				placement_strategy: 'FILL_FIRST',
				cloud: { enabled: false }
			}
		});
		expect(setup.ok()).toBeTruthy();
	}

	// Login as admin to ensure test user with 1GB quota exists
	await page.context().clearCookies();
	const adminLogin = await page.request.post('/api/v1/auth/login', {
		data: { identifier: adminUsername, password: adminPassword }
	});
	expect(adminLogin.ok()).toBeTruthy();

	// Create regular user with 1GB quota if not exists
	const createUserRes = await page.request.post('/api/v1/admin/users', {
		data: {
			username: testUsername,
			email: 'quota-user@example.test',
			password: testPassword,
			role: 'regular',
			quota_mode: 'CUSTOM',
			quota_gb: 1
		}
	});
	// If already exists (409), that's fine
	expect([201, 409]).toContain(createUserRes.status());

	// Now login via UI as the test user with 1GB quota
	await page.context().clearCookies();
	await page.goto('/login');
	await page.getByLabel('Username atau Email').fill(testUsername);
	await page.getByRole('textbox', { name: 'Password' }).fill(testPassword);
	await page.getByRole('button', { name: 'Masuk' }).click();
	await page.waitForURL('**/drive/home');
}

test('Storage quota updates reactively without reload and syncs across tabs', async ({ page, context }) => {
	await setupAndLogin(page);

	// Create a temporary 100MB file (0.1 GB)
	const tempDir = mkdtempSync(join(tmpdir(), 'quota-reactive-'));
	const testFilePath = join(tempDir, 'test-quota-100mb.bin');
	const fileBuffer = Buffer.alloc(100 * 1024 * 1024, 0x41); // 100 MB
	writeFileSync(testFilePath, fileBuffer);

	try {
		await page.goto('/drive/my-storage');
		await page.waitForLoadState('networkidle');

		// Check initial storage text on Tab 1 (should be 0 / 1 GB)
		const sidebarQuota1 = page.locator('aside').getByText(/\/ 1 GB/i);
		await expect(sidebarQuota1).toBeVisible();
		await expect(sidebarQuota1).toContainText('0 / 1 GB');

		// Open Tab 2 in the same browser context (same session/cookies)
		const page2 = await context.newPage();
		await page2.goto('/drive/my-storage');
		await page2.waitForLoadState('networkidle');
		const sidebarQuota2 = page2.locator('aside').getByText(/\/ 1 GB/i);
		await expect(sidebarQuota2).toBeVisible();
		await expect(sidebarQuota2).toContainText('0 / 1 GB');

		// Tab 1: Upload the 100MB file
		const fileInput = page.getByTestId('upload-file-input');
		await fileInput.setInputFiles(testFilePath);

		// Wait for the uploaded file to appear in My Storage file listing
		const uploadedItem = page.getByRole('option').filter({ hasText: 'test-quota-100mb.bin' });
		await expect(uploadedItem).toBeVisible({ timeout: 20000 });

		// Tab 1: Verify storage widget in Tab 1 updated to 0.1 / 1 GB WITHOUT reload
		await expect(sidebarQuota1).toContainText('0.1 / 1 GB', { timeout: 10000 });

		// Tab 2: Verify storage widget in Tab 2 synced via BroadcastChannel to 0.1 / 1 GB WITHOUT reload
		await expect(sidebarQuota2).toContainText('0.1 / 1 GB', { timeout: 10000 });

		// Tab 1: Move file to Trash
		await uploadedItem.getByRole('button', { name: 'Aksi berkas' }).click();
		await page.getByRole('menuitem', { name: 'Hapus ke Trash' }).click();
		const deleteModal = page.getByTestId('confirm-dialog');
		await expect(deleteModal).toBeVisible();
		await deleteModal.getByRole('button', { name: 'Hapus ke Trash' }).click();
		await expect(uploadedItem).toHaveCount(0);
		// Tab 1: Go to /drive/trash and empty trash
		await page.goto('/drive/trash');
		await page.waitForLoadState('networkidle');
		const emptyTrashBtn = page.getByRole('button', { name: /Kosongkan Sampah/i });
		await expect(emptyTrashBtn).toBeVisible();
		await emptyTrashBtn.click();

		const confirmPurgeBtn = page.getByRole('button', { name: 'Hapus permanen semua' });
		await expect(confirmPurgeBtn).toBeVisible();
		await confirmPurgeBtn.click();

		// Tab 1: Storage quota updates back down to 0 / 1 GB WITHOUT reload
		await expect(sidebarQuota1).toContainText('0 / 1 GB', { timeout: 10000 });

		// Tab 2: Storage quota on Tab 2 also updates back down to 0 / 1 GB WITHOUT reload
		await expect(sidebarQuota2).toContainText('0 / 1 GB', { timeout: 10000 });

		await page2.close();
	} finally {
		rmSync(tempDir, { recursive: true, force: true });
	}
});

test('Storage quota handles single trash purge and failed upload gracefully', async ({ page }) => {
	await setupAndLogin(page);

	const tempDir = mkdtempSync(join(tmpdir(), 'quota-edge-'));
	const testFilePath = join(tempDir, 'single-purge-test.bin');
	writeFileSync(testFilePath, Buffer.alloc(100 * 1024 * 1024, 0x42)); // 100 MB

	try {
		await page.goto('/drive/my-storage');
		await page.waitForLoadState('networkidle');

		const sidebarQuota = page.locator('aside').getByText(/\/ 1 GB/i);
		await expect(sidebarQuota).toContainText('0 / 1 GB');

		// 1. Upload file
		await page.getByTestId('upload-file-input').setInputFiles(testFilePath);
		const item = page.getByRole('option').filter({ hasText: 'single-purge-test.bin' });
		await expect(item).toBeVisible({ timeout: 20000 });
		await expect(sidebarQuota).toContainText('0.1 / 1 GB', { timeout: 10000 });

		// 2. Move to trash
		await item.getByRole('button', { name: 'Aksi berkas' }).click();
		await page.getByRole('menuitem', { name: 'Hapus ke Trash' }).click();
		const deleteModal = page.getByTestId('confirm-dialog');
		await expect(deleteModal).toBeVisible();
		await deleteModal.getByRole('button', { name: 'Hapus ke Trash' }).click();
		await expect(item).toHaveCount(0);

		// 3. Go to Trash and purge single item
		await page.goto('/drive/trash');
		await page.waitForLoadState('networkidle');
		const trashItem = page.getByRole('option').filter({ hasText: 'single-purge-test.bin' });
		await expect(trashItem).toBeVisible();
		await trashItem.getByRole('button', { name: 'Aksi trash' }).click();
		await page.getByRole('menuitem', { name: 'Hapus permanen' }).click();
		const singlePurgeModal = page.getByTestId('confirm-dialog');
		await expect(singlePurgeModal).toBeVisible();
		await singlePurgeModal.getByRole('button', { name: 'Hapus permanen' }).click();
		await expect(trashItem).toHaveCount(0);

		// Quota decreases back to 0 / 1 GB without reload
		await expect(sidebarQuota).toContainText('0 / 1 GB', { timeout: 10000 });

		// 4. Edge case: simulated quota exceeded (413) upload
		await page.goto('/drive/my-storage');
		await page.waitForLoadState('networkidle');
		await page.route('**/api/v1/drive/my-storage/upload**', async (route) => {
			await route.fulfill({
				status: 413,
				contentType: 'application/json',
				body: JSON.stringify({ error: 'quota exceeded', code: 'USER_QUOTA_EXCEEDED' })
			});
		});

		await page.getByTestId('upload-file-input').setInputFiles(testFilePath);
		const queue = page.getByTestId('upload-queue');
		await expect(queue).toBeVisible();
		// Quota remains accurate (0 / 1 GB) and doesn't get corrupted
		await expect(sidebarQuota).toContainText('0 / 1 GB', { timeout: 10000 });

		await page.unroute('**/api/v1/drive/my-storage/upload**');
	} finally {
		rmSync(tempDir, { recursive: true, force: true });
	}
});
