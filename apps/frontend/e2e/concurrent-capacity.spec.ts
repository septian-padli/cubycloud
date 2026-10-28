import { expect, test, type Locator, type Page } from '@playwright/test';
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { Buffer } from 'node:buffer';

const adminUsername = 'storage-e2e-admin';
const adminPassword = 'StorageE2E-password-2026';
const recoveryKey = 'ABCDE-FGHKL-MNPQR-STUVW-XYZ23-45678';

async function ensureSetup(page: Page) {
	const status = await page.request.get('/api/v1/setup/status');
	if (status.ok()) {
		const json = (await status.json()) as { is_setup_completed: boolean };
		if (!json.is_setup_completed) {
			const scan = await page.request.get('/api/v1/setup/scan-nodes');
			expect(scan.ok()).toBeTruthy();
			const { nodes } = (await scan.json()) as {
				nodes: { path: string; capacity_bytes: number }[];
			};
			const setup = await page.request.post('/api/v1/setup/complete', {
				data: {
					admin: {
						username: adminUsername,
						email: 'storage-e2e@example.test',
						password: adminPassword,
						recovery_key: recoveryKey
					},
					storage_nodes: nodes.map((node) => ({ ...node, enabled: true })),
					placement_strategy: 'FILL_FIRST',
					cloud: { enabled: false }
				}
			});
			expect(setup.ok()).toBeTruthy();
		}
	}
}

async function loginUser(page: Page, username = adminUsername, password = adminPassword) {
	await page.context().clearCookies();
	await page.goto('/login');
	await page.locator('#login-identifier').fill(username);
	await page.locator('#login-password').fill(password);
	await page.getByRole('button', { name: /Masuk|Sign in/i }).click();
	await page.waitForURL('**/drive/home');
}

function itemRow(page: Page, name: string | RegExp): Locator {
	return page.getByRole('option').filter({ hasText: name });
}

test.describe('Concurrent Upload Capacity in My Storage and Organization', () => {
	test.setTimeout(90_000);

	test('Simultaneous uploads in My Storage and Organization succeed concurrently within node capacity and preserve storage isolation', async ({
		page,
		context
	}) => {
		await ensureSetup(page);
		await loginUser(page);

		const tempDir = mkdtempSync(join(tmpdir(), 'concurrent-upload-'));
		const myStorageFile = join(tempDir, 'concurrent-my-storage.txt');
		const orgFile = join(tempDir, 'concurrent-org.txt');

		const myStorageContent = `Payload for My Storage concurrent test ${Date.now()}`;
		const orgContent = `Payload for Organization concurrent test ${Date.now()}`;

		writeFileSync(myStorageFile, myStorageContent);
		writeFileSync(orgFile, orgContent);

		try {
			// Tab 1: My Storage
			const myStoragePage = page;
			await myStoragePage.goto('/drive/my-storage');
			await expect(myStoragePage.getByRole('heading', { name: 'My Storage' })).toBeVisible();

			// Tab 2: Organization (same authenticated context)
			const orgPage = await context.newPage();
			await orgPage.goto('/drive/organization');
			await expect(
				orgPage.getByRole('heading', { name: /Organization|Organisasi/i })
			).toBeVisible();

			const myStorageInput = myStoragePage.getByTestId('upload-file-input');
			const orgInput = orgPage.getByTestId('upload-file-input');

			// Trigger both uploads simultaneously via Promise.all
			await Promise.all([
				myStorageInput.setInputFiles(myStorageFile),
				orgInput.setInputFiles(orgFile)
			]);

			// Observe that upload queues appear on both pages
			const myStorageQueue = myStoragePage.getByTestId('upload-queue');
			const orgQueue = orgPage.getByTestId('upload-queue');

			await Promise.all([
				expect(myStorageQueue).toBeVisible({ timeout: 15_000 }),
				expect(orgQueue).toBeVisible({ timeout: 15_000 })
			]);

			// Verify both files complete and appear in their respective file listings
			await expect(itemRow(myStoragePage, 'concurrent-my-storage.txt')).toBeVisible({
				timeout: 20_000
			});
			await expect(itemRow(orgPage, 'concurrent-org.txt')).toBeVisible({ timeout: 20_000 });

			// Verify storage isolation: My Storage item must not appear in Organization, and vice versa
			await expect(myStoragePage.getByText('concurrent-org.txt')).toHaveCount(0);
			await expect(orgPage.getByText('concurrent-my-storage.txt')).toHaveCount(0);

			// Verify backend API listing integrity for both scopes
			const myStorageRes = await page.request.get('/api/v1/drive/my-storage');
			expect(myStorageRes.ok()).toBeTruthy();
			const myStorageJson = (await myStorageRes.json()) as {
				items: { name: string; size_bytes: number }[];
			};
			const uploadedMyItem = myStorageJson.items.find(
				(item) => item.name === 'concurrent-my-storage.txt'
			);
			expect(uploadedMyItem).toBeDefined();
			expect(uploadedMyItem?.size_bytes).toBe(Buffer.byteLength(myStorageContent));

			const orgRes = await page.request.get('/api/v1/drive/organization');
			expect(orgRes.ok()).toBeTruthy();
			const orgJson = (await orgRes.json()) as {
				items: { name: string; size_bytes: number }[];
			};
			const uploadedOrgItem = orgJson.items.find((item) => item.name === 'concurrent-org.txt');
			expect(uploadedOrgItem).toBeDefined();
			expect(uploadedOrgItem?.size_bytes).toBe(Buffer.byteLength(orgContent));

			await orgPage.close();
		} finally {
			rmSync(tempDir, { recursive: true, force: true });
		}
	});

	test('Simultaneous uploads competing for remaining capacity: one succeeds while the other fails with storage pool full error', async ({
		page,
		context
	}) => {
		await ensureSetup(page);
		await loginUser(page);

		const tempDir = mkdtempSync(join(tmpdir(), 'concurrent-competing-'));
		const winningFile = join(tempDir, 'competing-my-storage.txt');
		const losingFile = join(tempDir, 'competing-org.txt');

		writeFileSync(winningFile, `Winning file content ${Date.now()}`);
		writeFileSync(losingFile, `Losing file content ${Date.now()}`);

		try {
			// Tab 1: My Storage
			const myStoragePage = page;
			await myStoragePage.goto('/drive/my-storage');
			await expect(myStoragePage.getByRole('heading', { name: 'My Storage' })).toBeVisible();

			// Tab 2: Organization
			const orgPage = await context.newPage();
			await orgPage.goto('/drive/organization');
			await expect(
				orgPage.getByRole('heading', { name: /Organization|Organisasi/i })
			).toBeVisible();

			// Simulate that available physical node capacity was exhausted by the first upload,
			// causing the concurrent organization upload to receive STORAGE_POOL_FULL (413)
			await orgPage.route('**/api/v1/drive/organization/upload**', async (route) => {
				await route.fulfill({
					status: 413,
					contentType: 'application/json',
					body: JSON.stringify({ error: 'STORAGE_POOL_FULL', code: 'STORAGE_POOL_FULL' })
				});
			});

			const myStorageInput = myStoragePage.getByTestId('upload-file-input');
			const orgInput = orgPage.getByTestId('upload-file-input');

			// Trigger both uploads simultaneously
			await Promise.all([
				myStorageInput.setInputFiles(winningFile),
				orgInput.setInputFiles(losingFile)
			]);

			// Tab 1 (My Storage - Winner): completes successfully
			await expect(itemRow(myStoragePage, 'competing-my-storage.txt')).toBeVisible({
				timeout: 20_000
			});

			// Tab 2 (Organization - Loser): fails due to capacity limit
			const orgQueue = orgPage.getByTestId('upload-queue');
			await expect(orgQueue).toBeVisible({ timeout: 15_000 });

			// Check that UI shows appropriate error notification or failure indicator
			const orgErrorIndicator = orgPage.locator(
				'[data-sonner-toast], [data-testid="upload-queue"]'
			);
			await expect(
				orgErrorIndicator
					.filter({
						hasText: /kuota|penyimpanan|penuh|gagal|batal/i
					})
					.first()
			).toBeVisible({ timeout: 15_000 });

			// Verify losing file is NOT present in the Organization file list
			await expect(orgPage.getByText('competing-org.txt')).toHaveCount(0);

			// Verify winning file is intact in My Storage
			await expect(itemRow(myStoragePage, 'competing-my-storage.txt')).toBeVisible();

			await orgPage.unroute('**/api/v1/drive/organization/upload**');
			await orgPage.close();
		} finally {
			rmSync(tempDir, { recursive: true, force: true });
		}
	});

	test('Simultaneous uploads when node capacity is exhausted display appropriate error notification and failure status in UI', async ({
		page,
		context
	}) => {
		await ensureSetup(page);
		await loginUser(page);

		const tempDir = mkdtempSync(join(tmpdir(), 'concurrent-exhausted-'));
		const myStorageFile = join(tempDir, 'exhausted-my-storage.txt');
		const orgFile = join(tempDir, 'exhausted-org.txt');

		writeFileSync(myStorageFile, `Exhausted capacity file 1 ${Date.now()}`);
		writeFileSync(orgFile, `Exhausted capacity file 2 ${Date.now()}`);

		try {
			// Tab 1: My Storage
			const myStoragePage = page;
			await myStoragePage.goto('/drive/my-storage');
			await expect(myStoragePage.getByRole('heading', { name: 'My Storage' })).toBeVisible();

			// Tab 2: Organization
			const orgPage = await context.newPage();
			await orgPage.goto('/drive/organization');
			await expect(
				orgPage.getByRole('heading', { name: /Organization|Organisasi/i })
			).toBeVisible();

			// Simulate node physical storage pool being completely full for both endpoints
			await myStoragePage.route('**/api/v1/drive/my-storage/upload**', async (route) => {
				await route.fulfill({
					status: 413,
					contentType: 'application/json',
					body: JSON.stringify({ error: 'STORAGE_POOL_FULL', code: 'STORAGE_POOL_FULL' })
				});
			});

			await orgPage.route('**/api/v1/drive/organization/upload**', async (route) => {
				await route.fulfill({
					status: 413,
					contentType: 'application/json',
					body: JSON.stringify({ error: 'STORAGE_POOL_FULL', code: 'STORAGE_POOL_FULL' })
				});
			});

			const myStorageInput = myStoragePage.getByTestId('upload-file-input');
			const orgInput = orgPage.getByTestId('upload-file-input');

			// Trigger both uploads simultaneously
			await Promise.all([
				myStorageInput.setInputFiles(myStorageFile),
				orgInput.setInputFiles(orgFile)
			]);

			// Verify UI displays error notification or failure indicator on Tab 1 (My Storage)
			const myStorageError = myStoragePage.locator(
				'[data-sonner-toast], [data-testid="upload-queue"]'
			);
			await expect(
				myStorageError
					.filter({
						hasText: /kuota|penyimpanan|penuh|gagal|batal/i
					})
					.first()
			).toBeVisible({ timeout: 15_000 });

			// Verify UI displays error notification or failure indicator on Tab 2 (Organization)
			const orgError = orgPage.locator('[data-sonner-toast], [data-testid="upload-queue"]');
			await expect(
				orgError
					.filter({
						hasText: /kuota|penyimpanan|penuh|gagal|batal/i
					})
					.first()
			).toBeVisible({ timeout: 15_000 });

			// Verify neither file is created in either file listing (storage integrity preserved)
			await expect(myStoragePage.getByText('exhausted-my-storage.txt')).toHaveCount(0);
			await expect(orgPage.getByText('exhausted-org.txt')).toHaveCount(0);

			await myStoragePage.unroute('**/api/v1/drive/my-storage/upload**');
			await orgPage.unroute('**/api/v1/drive/organization/upload**');
			await orgPage.close();
		} finally {
			rmSync(tempDir, { recursive: true, force: true });
		}
	});
});
