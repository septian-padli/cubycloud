import { expect, test, type Locator, type Page } from '@playwright/test';
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

const username = 'storage-e2e-admin';
const password = 'StorageE2E-password-2026';
const recoveryKey = 'ABCDE-FGHKL-MNPQR-STUVW-XYZ23-45678';

async function finishSetup(page: Page) {
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
				admin: { username, email: 'compress-e2e@example.test', password, recovery_key: recoveryKey },
				storage_nodes: nodes.map((node) => ({ ...node, enabled: true })),
				placement_strategy: 'FILL_FIRST',
				cloud: { enabled: false }
			}
		});
		expect(setup.ok()).toBeTruthy();
	}

	await page.context().clearCookies();
	await page.goto('/login');
	await page.locator('#login-identifier').fill(username);
	await page.locator('#login-password').fill(password);
	await page.getByRole('button', { name: /Masuk|Sign in/i }).click();
	await page.waitForURL('**/drive/home');
	await page.goto('/drive/my-storage');
}

function itemRow(page: Page, name: string | RegExp): Locator {
	return page.getByRole('option').filter({ hasText: name });
}

test('Compress to ZIP creates persistent archive in action-origin folder with conflict resolution', async ({ page }) => {
	test.setTimeout(120_000);
	await finishSetup(page);

	// Clean my-storage
	const cleanRes = await page.request.get('/api/v1/drive/my-storage');
	if (cleanRes.ok()) {
		const { items } = (await cleanRes.json()) as { items: { id: string }[] };
		for (const item of items) {
			await page.request.delete(`/api/v1/drive/my-storage/${encodeURIComponent(item.id)}`);
		}
		await page.request.delete('/api/v1/drive/trash');
	}
	await page.goto('/drive/my-storage');
	await expect(page.getByRole('heading', { name: 'My Storage' })).toBeVisible();

	const baseDir = mkdtempSync(join(tmpdir(), 'compress-e2e-'));
	const fileA = join(baseDir, 'report-1.txt');
	const fileB = join(baseDir, 'report-2.txt');
	writeFileSync(fileA, 'Report Alpha Content');
	writeFileSync(fileB, 'Report Beta Content');

	try {
		// 1. Create a subfolder "Projects" upfront
		await page.getByRole('button', { name: 'Tambah' }).click();
		await page.getByRole('menuitem', { name: 'Folder Baru' }).click();
		await page.getByLabel('Nama folder').fill('Projects');
		await page.getByRole('button', { name: 'Buat' }).click();
		await expect(itemRow(page, 'Projects')).toBeVisible();

		// 2. Upload files to root
		await page.getByTestId('upload-file-input').setInputFiles([fileA, fileB]);
		await expect(itemRow(page, 'report-1.txt')).toBeVisible({ timeout: 15_000 });
		await expect(itemRow(page, 'report-2.txt')).toBeVisible({ timeout: 15_000 });

		// 3. Multi-select both files and click "Compress to ZIP"
		await page.getByRole('button', { name: 'Pilih report-1.txt' }).click();
		await page.getByRole('button', { name: 'Pilih report-2.txt' }).click();
		await expect(page.getByTestId('bulk-action-bar')).toBeVisible();
		await page.getByTestId('bulk-actions-menu-trigger').click();
		await page.getByTestId('bulk-action-compress').click();

		// New Archive.zip should appear in the root view
		await expect(itemRow(page, 'Archive.zip')).toBeVisible({ timeout: 15_000 });

		// 4. Compress again to test conflict-safe naming: Archive (1).zip
		await page.getByRole('button', { name: 'Pilih report-1.txt' }).click();
		await page.getByRole('button', { name: 'Pilih report-2.txt' }).click();
		await expect(page.getByTestId('bulk-action-bar')).toBeVisible();
		await page.getByTestId('bulk-actions-menu-trigger').click();
		await page.getByTestId('bulk-action-compress').click();

		await expect(itemRow(page, 'Archive (1).zip')).toBeVisible({ timeout: 15_000 });

		// 5. Test Single-item compression via Actions dropdown
		const singleRow = itemRow(page, 'report-1.txt');
		await singleRow.getByRole('button', { name: /Aksi berkas|File actions/i }).click();
		await page.getByTestId('action-compress').click();

		await expect(itemRow(page, 'report-1.zip')).toBeVisible({ timeout: 15_000 });

		// 6. Test Action-Origin folder placement:
		// Navigate inside Projects folder
		await itemRow(page, 'Projects').dblclick();
		await page.waitForURL(/folder=/);

		const subFile = join(baseDir, 'project-spec.txt');
		writeFileSync(subFile, 'Specification Details');

		await page.getByTestId('upload-file-input').setInputFiles([subFile]);
		await expect(itemRow(page, 'project-spec.txt')).toBeVisible({ timeout: 15_000 });
		const dismissQueue = page.getByRole('button', { name: 'Tutup notifikasi unggahan' });
		if (await dismissQueue.isVisible()) {
			await dismissQueue.click();
		}

		await page.getByRole('button', { name: 'Pilih project-spec.txt' }).click();
		await expect(page.getByTestId('bulk-action-bar')).toBeVisible();
		await page.getByTestId('bulk-actions-menu-trigger').click();
		await page.getByTestId('bulk-action-compress').click();

		// Should appear inside Projects folder
		await expect(itemRow(page, 'project-spec.zip')).toBeVisible({ timeout: 15_000 });
	} finally {
		rmSync(baseDir, { recursive: true, force: true });
	}
});
