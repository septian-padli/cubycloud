import { expect, test, type Page } from '@playwright/test';

const adminUsername = 'storage-e2e-admin';
const adminPassword = 'StorageE2E-password-2026';
const recoveryKey = 'ABCDE-FGHKL-MNPQR-STUVW-XYZ23-45678';

async function finishSetupAndLogin(page: Page) {
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
				admin: {
					username: adminUsername,
					email: 'storage-stats@example.test',
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

	await page.context().clearCookies();
	await page.goto('/login');
	await page.locator('#login-identifier').fill(adminUsername);
	await page.locator('#login-password').fill(adminPassword);
	await page.getByRole('button', { name: /Masuk|Sign in/i }).click();
	await page.waitForURL('**/drive/home');
}

async function uploadFile(
	page: Page,
	filename: string,
	sizeBytes: number,
	parentId?: string
): Promise<{ id: string; name: string }> {
	const query = parentId
		? `?filename=${encodeURIComponent(filename)}&parent_id=${encodeURIComponent(parentId)}`
		: `?filename=${encodeURIComponent(filename)}`;
	const buffer = Buffer.alloc(sizeBytes, 'a');
	const response = await page.request.post(`/api/v1/drive/my-storage/upload${query}`, {
		multipart: {
			file: {
				name: filename,
				mimeType: 'text/plain',
				buffer
			}
		}
	});
	expect(response.ok()).toBeTruthy();
	return (await response.json()) as { id: string; name: string };
}

async function createFolder(
	page: Page,
	name: string,
	parentId?: string
): Promise<{ id: string; name: string }> {
	const response = await page.request.post('/api/v1/drive/my-storage/folders', {
		data: {
			name,
			parent_id: parentId ?? null
		}
	});
	expect(response.ok()).toBeTruthy();
	return (await response.json()) as { id: string; name: string };
}

test.describe('Storage Stats Largest Files', () => {
	test('displays only user My Storage files, nested folders, correct locations, excludes trash & shared, sorts descending, and supports infinite scroll', async ({
		page
	}) => {
		await finishSetupAndLogin(page);
		const cleanRes = await page.request.get('/api/v1/drive/my-storage');
		if (cleanRes.ok()) {
			const { items } = (await cleanRes.json()) as { items: { id: string }[] };
			for (const item of items) {
				await page.request.delete(`/api/v1/drive/my-storage/${encodeURIComponent(item.id)}`);
			}
			await page.request.delete('/api/v1/drive/trash');
		}

		// 1. Create nested folders: Work -> Reports
		const workFolder = await createFolder(page, 'Work');
		const reportsFolder = await createFolder(page, 'Reports', workFolder.id);

		// 2. Upload files with specific sizes
		// Biggest file in subfolder (25 KB)
		const hugeFile = await uploadFile(page, 'nested-huge-report.pdf', 25_000, reportsFolder.id);

		// Medium file in root (12 KB)
		const mediumFile = await uploadFile(page, 'root-medium-file.txt', 12_000);

		// Small file in root (3 KB)
		const smallFile = await uploadFile(page, 'root-small-notes.txt', 3_000);

		// Trashed file (35 KB) — uploaded then deleted to trash
		const trashed = await uploadFile(page, 'should-be-trashed.bin', 35_000);
		const deleteRes = await page.request.delete(`/api/v1/drive/my-storage/${trashed.id}`);
		expect(deleteRes.ok()).toBeTruthy();

		// Organization shared file (50 KB) — uploaded to org drive, not user's personal My Storage
		const orgBuffer = Buffer.alloc(50_000, 'o');
		const orgRes = await page.request.post(
			'/api/v1/drive/organization/upload?filename=shared-org-document.pdf',
			{
				multipart: {
					file: {
						name: 'shared-org-document.pdf',
						mimeType: 'application/pdf',
						buffer: orgBuffer
					}
				}
			}
		);
		expect(orgRes.ok()).toBeTruthy();

		// Upload 22 additional small files (100-200 bytes) to have > 20 files total for infinite scroll test
		for (let i = 1; i <= 22; i++) {
			const num = String(i).padStart(2, '0');
			await uploadFile(page, `bulk-file-${num}.txt`, 100 + i);
		}

		// 3. Navigate to /drive/storage-stats
		await page.goto('/drive/storage-stats');

		// 4. Verify Quota Details and personal storage summary UI remain visible
		await expect(
			page.getByRole('heading', { name: 'Detail kuota & pemakaian My Storage' })
		).toBeVisible();

		// 5. Verify Largest files header
		await expect(page.getByRole('heading', { name: 'Berkas terbesar' })).toBeVisible();

		// 6. Verify nested-huge-report.pdf is present with name, size, modified date, and location
		const hugeRow = page.getByRole('option').filter({ hasText: hugeFile.name });
		await expect(hugeRow).toBeVisible();
		await expect(hugeRow).toContainText('My Storage / Work / Reports');
		await expect(hugeRow).toContainText('Diubah');

		// 7. Verify root-medium-file.txt is present with location My Storage
		const mediumRow = page.getByRole('option').filter({ hasText: mediumFile.name });
		await expect(mediumRow).toBeVisible();
		await expect(mediumRow).toContainText('My Storage');

		// 8. Verify root-small-notes.txt is present
		const smallRow = page.getByRole('option').filter({ hasText: smallFile.name });
		await expect(smallRow).toBeVisible();
		await expect(smallRow).toContainText('My Storage');

		// 9. Exclude Trash and Shared files
		await expect(page.getByRole('option').filter({ hasText: 'should-be-trashed.bin' })).toHaveCount(
			0
		);
		await expect(
			page.getByRole('option').filter({ hasText: 'shared-org-document.pdf' })
		).toHaveCount(0);

		// 10. Verify order descending by size:
		// nested-huge-report.pdf (25 KB) must appear before root-medium-file.txt (12 KB), which appears before root-small-notes.txt (3 KB)
		const allVisibleOptions = page.getByRole('option');
		const hugeIndex = await allVisibleOptions
			.allTextContents()
			.then((texts) => texts.findIndex((t) => t.includes(hugeFile.name)));
		const mediumIndex = await allVisibleOptions
			.allTextContents()
			.then((texts) => texts.findIndex((t) => t.includes(mediumFile.name)));
		const smallIndex = await allVisibleOptions
			.allTextContents()
			.then((texts) => texts.findIndex((t) => t.includes(smallFile.name)));

		expect(hugeIndex).toBeGreaterThanOrEqual(0);
		expect(mediumIndex).toBeGreaterThanOrEqual(0);
		expect(smallIndex).toBeGreaterThanOrEqual(0);
		expect(hugeIndex).toBeLessThan(mediumIndex);
		expect(mediumIndex).toBeLessThan(smallIndex);

		// 11. Test Infinite Scrolling
		// Initial count should be 20 items (first page)
		const initialCount = await allVisibleOptions.count();
		expect(initialCount).toBe(20);

		// Sentinel / Load more button should be present since total is 25 items
		const sentinel = page.getByTestId('infinite-scroll-sentinel');
		await expect(sentinel).toBeVisible();

		// Scroll to sentinel or click load more button
		const loadMoreBtn = sentinel.getByRole('button', { name: 'Muat lebih banyak' });
		if (await loadMoreBtn.isVisible()) {
			await loadMoreBtn.click();
		} else {
			await sentinel.scrollIntoViewIfNeeded();
		}

		// After scrolling/loading more, item count should increase (25 total items loaded)
		await expect.poll(async () => allVisibleOptions.count()).toBeGreaterThan(20);
		const finalCount = await allVisibleOptions.count();
		expect(finalCount).toBe(25);
	});
});
