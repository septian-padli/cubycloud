import { expect, test, type Locator, type Page } from '@playwright/test';
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

const username = 'multiselect-e2e-admin';
const password = 'MultiSelect-password-2026';
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
				admin: { username, email: 'multiselect-e2e@example.test', password, recovery_key: recoveryKey },
				storage_nodes: nodes.map((node) => ({ ...node, enabled: true })),
				placement_strategy: 'FILL_FIRST',
				cloud: { enabled: false }
			}
		});
		expect(setup.ok()).toBeTruthy();
	}

	await page.context().clearCookies();
	await page.goto('/login');
	await page.getByLabel('Username atau Email').fill(username);
	await page.getByRole('textbox', { name: 'Password' }).fill(password);
	await page.getByRole('button', { name: 'Masuk' }).click();
	await page.waitForURL('**/drive/home');
	await page.goto('/drive/my-storage');
}

function itemRow(page: Page, name: string | RegExp): Locator {
	return page.getByRole('option').filter({ hasText: name });
}

test('Multi-select actions in List & Grid view with bulk bar, move, delete, zip, and drag-drop', async ({ page }) => {
	await finishSetup(page);

	const baseDir = mkdtempSync(join(tmpdir(), 'multiselect-e2e-'));
	const fileA = join(baseDir, 'doc-alpha.txt');
	const fileB = join(baseDir, 'doc-beta.txt');
	const fileC = join(baseDir, 'doc-gamma.txt');
	writeFileSync(fileA, 'Alpha content 123456');
	writeFileSync(fileB, 'Beta content 789012');
	writeFileSync(fileC, 'Gamma content 345678');

	try {
		// 1. Create a target folder: "ArchiveFolder"
		await page.getByRole('button', { name: 'Tambah' }).click();
		await page.getByRole('menuitem', { name: 'Folder Baru' }).click();
		await page.getByLabel('Nama folder').fill('ArchiveFolder');
		await page.getByRole('button', { name: 'Buat' }).click();
		await expect(itemRow(page, 'ArchiveFolder')).toBeVisible();

		// 2. Upload files
		await page.getByTestId('upload-file-input').setInputFiles([fileA, fileB, fileC]);
		await expect(itemRow(page, 'doc-alpha.txt')).toBeVisible();
		await expect(itemRow(page, 'doc-beta.txt')).toBeVisible();
		await expect(itemRow(page, 'doc-gamma.txt')).toBeVisible();

		// Verify star button is not in the row directly (only in actions menu)
		const alphaRow = itemRow(page, 'doc-alpha.txt');
		await expect(alphaRow.getByTitle('Bintang')).toHaveCount(0);
		await expect(alphaRow.getByTitle('Hapus Bintang')).toHaveCount(0);

		// Bulk action bar should not be visible when nothing selected
		await expect(page.getByTestId('bulk-action-bar')).toBeHidden();

		// 3. Selection in List View: click icon selection button
		await page.getByRole('button', { name: 'Pilih doc-alpha.txt' }).click();
		await expect(page.getByTestId('bulk-action-bar')).toBeVisible();
		await expect(page.getByTestId('selected-count-label')).toHaveText('1 item dipilih');

		// Select second item
		await page.getByRole('button', { name: 'Pilih doc-beta.txt' }).click();
		await expect(page.getByTestId('selected-count-label')).toHaveText('2 item dipilih');

		// Verify indeterminate state
		const selectAllBox = page.getByTestId('select-all-checkbox');
		expect(await selectAllBox.evaluate((el: HTMLInputElement) => el.indeterminate)).toBeTruthy();

		// Click select all checkbox -> selects all items (4 items)
		await selectAllBox.click();
		await expect(page.getByTestId('selected-count-label')).toHaveText('4 item dipilih');
		expect(await selectAllBox.evaluate((el: HTMLInputElement) => el.checked)).toBeTruthy();

		// Click select all again -> deselects all
		await selectAllBox.click();
		await expect(page.getByTestId('bulk-action-bar')).toBeHidden();

		// 4. Test Grid View Selection & Star removal
		await page.getByRole('button', { name: 'Tampilan Grid' }).click();
		const alphaCard = itemRow(page, 'doc-alpha.txt');
		await expect(alphaCard.getByTitle('Bintang')).toHaveCount(0);

		// Click top-left selection checkbox on Grid card
		await page.getByRole('button', { name: 'Pilih doc-alpha.txt' }).click();
		await expect(page.getByTestId('bulk-action-bar')).toBeVisible();
		await expect(page.getByTestId('selected-count-label')).toHaveText('1 item dipilih');

		// Switch back to List view
		await page.getByRole('button', { name: 'Tampilan List' }).click();
		await expect(page.getByTestId('selected-count-label')).toHaveText('1 item dipilih');

		// Deselect
		await page.getByRole('button', { name: 'Pilih doc-alpha.txt' }).click();
		await expect(page.getByTestId('bulk-action-bar')).toBeHidden();

		// 5. Test Bulk Move via Modal
		await page.getByRole('button', { name: 'Pilih doc-alpha.txt' }).click();
		await page.getByRole('button', { name: 'Pilih doc-beta.txt' }).click();

		await page.getByTestId('bulk-actions-menu-trigger').click();
		await page.getByTestId('bulk-action-move').click();

		const moveDialog = page.getByTestId('move-dialog');
		await expect(moveDialog).toBeVisible();
		await expect(moveDialog.getByText('2 item dipilih')).toBeVisible();

		// Select ArchiveFolder in MoveDialog
		await moveDialog.getByTestId(/^btn-move-to-/).first().click();
		await expect(moveDialog).toBeHidden();

		// Both alpha and beta are moved into ArchiveFolder
		await expect(itemRow(page, 'doc-alpha.txt')).toHaveCount(0);
		await expect(itemRow(page, 'doc-beta.txt')).toHaveCount(0);

		// Navigate into ArchiveFolder
		await itemRow(page, 'ArchiveFolder').dblclick();
		await page.waitForURL(/folder=/);
		await expect(itemRow(page, 'doc-alpha.txt')).toBeVisible();
		await expect(itemRow(page, 'doc-beta.txt')).toBeVisible();

		// 6. Test Bulk Download as ZIP with progress task in top-right queue
		await page.getByRole('button', { name: 'Pilih doc-alpha.txt' }).click();
		await page.getByRole('button', { name: 'Pilih doc-beta.txt' }).click();

		await page.getByTestId('bulk-actions-menu-trigger').click();
		await page.getByTestId('bulk-action-download').click();

		// Progress task queue appears in top-right
		const uploadQueue = page.getByTestId('upload-queue');
		await expect(uploadQueue).toBeVisible();
		await expect(uploadQueue.getByText(/Men-zip-kan 2 item/)).toBeVisible();

		// Task finishes with done
		await expect.poll(async () => {
			return await uploadQueue.textContent();
		}, { timeout: 15000 }).toContain('Men-zip-kan 2 item Selesai');

		// 7. Test Bulk Delete to Trash with confirmation dialog
		await page.getByTestId('bulk-actions-menu-trigger').click();
		await page.getByTestId('bulk-action-delete').click();

		const confirmDialog = page.getByTestId('confirm-dialog');
		await expect(confirmDialog).toBeVisible();
		await expect(confirmDialog.getByText(/Pindahkan 2 Item ke Trash/i)).toBeVisible();
		await confirmDialog.getByRole('button', { name: 'Hapus ke Trash' }).click();

		await expect(itemRow(page, 'doc-alpha.txt')).toHaveCount(0);
		await expect(itemRow(page, 'doc-beta.txt')).toHaveCount(0);
		await expect(page.getByText('Folder kosong')).toBeVisible();

		// 8. Test Drag and Drop multi-item into folder
		// Navigate back to root
		await page.getByRole('navigation', { name: 'Breadcrumb' }).getByRole('button', { name: 'My Storage' }).click();
		await page.waitForURL((url) => !url.searchParams.get('folder'));
		await expect(itemRow(page, 'doc-gamma.txt')).toBeVisible();
		await expect(itemRow(page, 'ArchiveFolder')).toBeVisible();

		// Upload two more files to test drag-and-drop
		const fileD = join(baseDir, 'drag-one.txt');
		const fileE = join(baseDir, 'drag-two.txt');
		writeFileSync(fileD, 'Drag One');
		writeFileSync(fileE, 'Drag Two');
		await page.getByTestId('upload-file-input').setInputFiles([fileD, fileE]);
		await expect(itemRow(page, 'drag-one.txt')).toBeVisible();
		await expect(itemRow(page, 'drag-two.txt')).toBeVisible();

		// Select both drag files
		await page.getByRole('button', { name: 'Pilih drag-one.txt' }).click();
		await page.getByRole('button', { name: 'Pilih drag-two.txt' }).click();
		await expect(page.getByTestId('selected-count-label')).toHaveText('2 item dipilih');

		// Drag selected item onto ArchiveFolder
		await itemRow(page, 'drag-one.txt').dragTo(itemRow(page, 'ArchiveFolder'));

		// Both dragged items should be moved into ArchiveFolder
		await expect(itemRow(page, 'drag-one.txt')).toHaveCount(0);
		await expect(itemRow(page, 'drag-two.txt')).toHaveCount(0);

		// Navigate into ArchiveFolder and verify both arrived
		await itemRow(page, 'ArchiveFolder').dblclick();
		await page.waitForURL(/folder=/);
		await expect(itemRow(page, 'drag-one.txt')).toBeVisible();
		await expect(itemRow(page, 'drag-two.txt')).toBeVisible();
	} finally {
		try {
			rmSync(baseDir, { recursive: true, force: true });
		} catch {
			// ignore cleanup error
		}
	}
});
