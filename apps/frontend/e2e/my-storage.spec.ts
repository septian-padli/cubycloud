import {
	chromium,
	expect,
	test,
	type BrowserContext,
	type Locator,
	type Page
} from '@playwright/test';
import {
	existsSync,
	mkdtempSync,
	mkdirSync,
	readFileSync,
	renameSync,
	rmSync,
	utimesSync,
	writeFileSync
} from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { restartControlDir } from './restart-control.js';

async function expectTabColors(tab: Locator, selected: boolean) {
	const colors = await tab.evaluate((element) => {
		const style = getComputedStyle(element);
		const tokenColor = (token: string) => {
			const probe = document.createElement('span');
			probe.style.color = `var(${token})`;
			document.body.append(probe);
			const color = getComputedStyle(probe).color;
			probe.remove();
			return color;
		};
		return {
			color: style.color,
			primary: tokenColor('--primary'),
			muted: tokenColor('--text-muted'),
			borderBottomColor: style.borderBottomColor,
			borderBottomWidth: style.borderBottomWidth
		};
	});
	expect(colors.color).toBe(selected ? colors.primary : colors.muted);
	if (selected) {
		expect(colors.borderBottomColor).toBe(colors.primary);
		expect(colors.borderBottomWidth).toBe('2px');
	} else {
		expect(colors.borderBottomColor).toBe('rgba(0, 0, 0, 0)');
		expect(colors.borderBottomWidth).toBe('2px');
	}
}

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
				admin: { username, email: 'storage-e2e@example.test', password, recovery_key: recoveryKey },
				storage_nodes: nodes.map((node) => ({ ...node, enabled: true })),
				placement_strategy: 'FILL_FIRST',
				cloud: { enabled: false }
			}
		});
		expect(setup.ok()).toBeTruthy();
	}
	// Setup is API-provisioned; clear its session and exercise the actual login form.
	await page.context().clearCookies();
	await page.goto('/login');
	await page.getByLabel('Username atau Email').fill(username);
	await page.getByRole('textbox', { name: 'Password' }).fill(password);
	await page.getByRole('button', { name: 'Masuk' }).click();
	await page.waitForURL('**/drive/home');
	await page.goto('/drive/my-storage');
	await expect(page.getByRole('heading', { name: 'My Storage' })).toBeVisible();
	await page.waitForFunction(async () => {
		const queue = await import('/src/lib/transfer-queue.svelte.ts');
		return queue.transferQueueState.ready && Boolean(queue.transferQueueState.ownerId);
	});
}

function itemRow(page: Page, name: string | RegExp): Locator {
	return page.getByRole('option').filter({ hasText: name });
}

async function openActions(page: Page, name: string) {
	await itemRow(page, name).getByRole('button', { name: 'Aksi berkas' }).click();
}

async function listing(page: Page, parentId = '') {
	const query = parentId ? `?parent_id=${encodeURIComponent(parentId)}` : '';
	const response = await page.request.get(`/api/v1/drive/my-storage${query}`);
	expect(response.ok()).toBeTruthy();
	return (await response.json()) as {
		items: {
			id: string;
			name: string;
			kind: 'file' | 'folder';
			size_bytes: number;
			parent_id: string | null;
		}[];
		breadcrumbs: { id: string; name: string }[];
	};
}

test('My Storage manages real files and folders end to end', async ({ page }) => {
	await finishSetup(page);

	const initialListing = await listing(page);
	for (const item of initialListing.items) {
		await page.request.delete(`/api/v1/drive/my-storage/${encodeURIComponent(item.id)}`);
	}
	await page.request.delete('/api/v1/drive/trash');
	await page.reload();
	await expect(page.getByRole('heading', { name: 'My Storage' })).toBeVisible();

	const base = mkdtempSync(join(tmpdir(), 'my-storage-e2e-upload-'));
	try {
		await page.getByRole('button', { name: 'Tambah' }).click();
		await page.getByRole('menuitem', { name: 'Folder Baru' }).click();
		await page.getByLabel('Nama folder').fill('collision-folder');
		await page.getByRole('button', { name: 'Buat' }).click();
		await expect(itemRow(page, 'collision-folder')).toBeVisible();
		await page.getByRole('button', { name: 'Tambah' }).click();
		await page.getByRole('menuitem', { name: 'Folder Baru' }).click();
		await page.getByLabel('Nama folder').fill('collision-folder');
		await page.getByRole('button', { name: 'Buat' }).click();
		const collisionModal = page.getByTestId('confirm-dialog');
		await expect(collisionModal).toBeVisible();
		expect(await collisionModal.textContent()).toContain('collision-folder');
		expect(await collisionModal.textContent()).toContain('(1)');
		await collisionModal.getByRole('button', { name: 'Gunakan Nama Ini' }).click();
		await expect(itemRow(page, /collision-folder \(1\)/)).toBeVisible();

		const simpleFile = join(base, 'notes.txt');
		const originalContent = 'x'.repeat(512 * 1024);
		writeFileSync(simpleFile, originalContent);
		const network = await page.context().newCDPSession(page);
		await network.send('Network.enable');
		await network.send('Network.emulateNetworkConditions', {
			offline: false,
			latency: 50,
			downloadThroughput: 10 * 1024 * 1024,
			uploadThroughput: 64 * 1024
		});
		await page.getByTestId('upload-file-input').setInputFiles(simpleFile);
		const uploadQueue = page.getByTestId('upload-queue');
		await expect(uploadQueue).toBeVisible();
		const progressbar = uploadQueue.getByRole('progressbar');
		await expect
			.poll(async () => Number(await progressbar.getAttribute('aria-valuenow')))
			.toBeGreaterThan(0);
		await expect
			.poll(async () => Number(await progressbar.getAttribute('aria-valuenow')))
			.toBeLessThan(100);
		await expect(
			uploadQueue.getByRole('button', { name: 'Tutup notifikasi unggahan' })
		).toHaveCount(0);
		await expect(uploadQueue.getByRole('button', { name: 'Buka antrean unggahan' })).toBeVisible();
		await network.send('Network.emulateNetworkConditions', {
			offline: false,
			latency: 0,
			downloadThroughput: -1,
			uploadThroughput: -1
		});
		await expect(itemRow(page, 'notes.txt')).toBeVisible();
		await expect(progressbar).toHaveAttribute('aria-valuenow', '100');
		await network.detach();
		const topNav = page.getByLabel('Cari berkas').locator('xpath=ancestor::header');
		const navBounds = await topNav.boundingBox();
		const initialQueueBounds = await uploadQueue.boundingBox();
		expect(navBounds).toBeTruthy();
		expect(initialQueueBounds).toBeTruthy();
		expect(initialQueueBounds!.y).toBeGreaterThanOrEqual(navBounds!.y + navBounds!.height);
		const successToast = page.locator('[data-sonner-toast][data-type="success"]').first();
		await expect(successToast).toBeVisible();
		const toastStyles = await successToast.evaluate((el) => {
			const s = window.getComputedStyle(el);
			return { bg: s.backgroundColor, border: s.borderColor };
		});
		expect(toastStyles.bg).toBe('rgb(255, 255, 255)');
		expect(toastStyles.border).toBe('rgb(96, 108, 56)');
		await expect(page.getByText('Unggahan selesai')).toBeVisible();
		const queueHeader = uploadQueue.locator('button').first();
		await expect(queueHeader).toHaveAttribute('aria-expanded', 'false');
		const expandQueueButton = uploadQueue.getByRole('button', { name: 'Buka antrean unggahan' });
		const dismissQueueButton = uploadQueue.getByRole('button', {
			name: 'Tutup notifikasi unggahan'
		});
		await expect(dismissQueueButton).toBeVisible();
		const chevronBounds = await expandQueueButton.boundingBox();
		const dismissBounds = await dismissQueueButton.boundingBox();
		const titleBounds = await queueHeader.boundingBox();
		expect(chevronBounds).toBeTruthy();
		expect(dismissBounds).toBeTruthy();
		expect(titleBounds).toBeTruthy();
		expect(chevronBounds!.x).toBeLessThan(dismissBounds!.x);
		expect(
			Math.abs(
				chevronBounds!.y +
					chevronBounds!.height / 2 -
					(dismissBounds!.y + dismissBounds!.height / 2)
			)
		).toBeLessThanOrEqual(2);
		expect(
			Math.abs(
				dismissBounds!.y + dismissBounds!.height / 2 - (titleBounds!.y + titleBounds!.height / 2)
			)
		).toBeLessThanOrEqual(4);
		await expandQueueButton.click();
		await expect(
			uploadQueue.getByRole('button', { name: 'Ciutkan antrean unggahan' })
		).toBeVisible();
		await expect(uploadQueue.getByTestId('upload-queue-item')).toContainText('notes.txt');
		await uploadQueue.getByRole('button', { name: 'Tutup notifikasi unggahan' }).click();
		await expect(page.getByTestId('upload-queue')).toHaveCount(0);
		let root = await listing(page);
		const original = root.items.find((item) => item.name === 'notes.txt');
		expect(original?.kind).toBe('file');
		expect(original?.size_bytes).toBe(Buffer.byteLength(originalContent));

		// A same-path upload is kept as a separately named item; the original bytes stay intact.
		writeFileSync(simpleFile, 'replacement candidate');
		await page.getByTestId('upload-file-input').setInputFiles(simpleFile);
		await expect
			.poll(async () => (await listing(page)).items.filter((item) => item.kind === 'file').length)
			.toBe(2);
		const reopenedQueue = page.getByTestId('upload-queue');
		await expect(reopenedQueue).toBeVisible();
		await expect(reopenedQueue.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '100');
		const queueBounds = await reopenedQueue.boundingBox();
		expect(queueBounds).toBeTruthy();
		await page.mouse.move(queueBounds!.x + 20, queueBounds!.y + 20);
		await page.mouse.down();
		await page.mouse.move(queueBounds!.x + 140, queueBounds!.y + 20, { steps: 3 });
		await page.mouse.up();
		await expect(reopenedQueue).toHaveCount(0);
		root = await listing(page);
		const suffixed = root.items.find((item) => item.kind === 'file' && item.id !== original?.id);
		expect(suffixed?.name).toMatch(/^notes(?: \(\d+\))?\.txt$/);
		expect(suffixed?.name).not.toBe('notes.txt');
		expect(root.items.find((item) => item.id === original?.id)?.size_bytes).toBe(
			Buffer.byteLength(originalContent)
		);
		await page.route('**/api/v1/drive/my-storage/*/duplicate', async (route) => {
			await route.fulfill({
				status: 500,
				contentType: 'application/json',
				body: JSON.stringify({ error: 'Gagal menduplikasi item' })
			});
		});
		await openActions(page, 'notes.txt');
		await page.getByRole('menuitem', { name: 'Duplikat' }).click();
		const errorToast = page.locator('[data-sonner-toast][data-type="error"]').first();
		await expect(errorToast).toBeVisible();
		const errorStyles = await errorToast.evaluate((el) => {
			const s = window.getComputedStyle(el);
			return { bg: s.backgroundColor, border: s.borderColor };
		});
		expect(errorStyles.bg).toBe('rgb(255, 255, 255)');
		expect(errorStyles.border).toBe('rgb(236, 0, 63)');
		await page.unroute('**/api/v1/drive/my-storage/*/duplicate');
		await openActions(page, 'notes.txt');
		await page.getByRole('menuitem', { name: 'Duplikat' }).click();
		await expect
			.poll(async () => (await listing(page)).items.filter((item) => item.kind === 'file').length)
			.toBe(3);
		root = await listing(page);
		const duplicateFile = root.items.find(
			(item) => item.kind === 'file' && item.id !== original?.id && item.id !== suffixed?.id
		);
		expect(duplicateFile?.name).toMatch(/^notes \(\d+\)\.txt$/);
		expect(duplicateFile?.name).not.toBe(suffixed?.name);
		expect(duplicateFile?.size_bytes).toBe(Buffer.byteLength(originalContent));
		const originalDownload = await page.request.get(
			`/api/v1/drive/my-storage/${encodeURIComponent(original!.id)}/download`
		);
		const duplicateDownload = await page.request.get(
			`/api/v1/drive/my-storage/${encodeURIComponent(duplicateFile!.id)}/download`
		);
		expect(originalDownload.ok()).toBeTruthy();
		expect(duplicateDownload.ok()).toBeTruthy();
		expect((await originalDownload.body()).toString()).toBe(originalContent);
		expect((await duplicateDownload.body()).toString()).toBe(originalContent);

		await openActions(page, 'notes.txt');
		const fileDownload = page.waitForEvent('download');
		await page.getByRole('menuitem', { name: 'Unduh' }).click();
		const downloadedFile = await fileDownload;
		const downloadedPath = join(base, 'downloaded-notes.txt');
		await downloadedFile.saveAs(downloadedPath);
		expect(readFileSync(downloadedPath, 'utf8')).toBe(originalContent);
		// Drag and drop across the whole page (bg-app area) outside the explorer card.
		await page.evaluate(() => {
			const dt = new DataTransfer();
			const file = new File(['dropped workspace content'], 'dropped-page.txt', {
				type: 'text/plain'
			});
			dt.items.add(file);
			window.dispatchEvent(new DragEvent('dragenter', { dataTransfer: dt, bubbles: true }));
		});
		await expect(page.getByText('Lepas berkas di mana saja untuk mengunggah')).toBeVisible();
		await page.evaluate(() => {
			const dt = new DataTransfer();
			const file = new File(['dropped workspace content'], 'dropped-page.txt', {
				type: 'text/plain'
			});
			dt.items.add(file);
			window.dispatchEvent(new DragEvent('drop', { dataTransfer: dt, bubbles: true }));
		});
		await expect(itemRow(page, 'dropped-page.txt')).toBeVisible();
		await page
			.getByTestId('upload-queue')
			.getByRole('button', { name: 'Tutup notifikasi unggahan' })
			.click();
		// Drag and drop an entire directory hierarchy using webkitGetAsEntry with nested files and empty subfolders
		await page.evaluate(() => {
			const originalGetAsEntry = DataTransferItem.prototype.webkitGetAsEntry;
			DataTransferItem.prototype.webkitGetAsEntry = function () {
				return {
					isFile: false,
					isDirectory: true,
					name: 'dropped-dir',
					createReader: () => {
						let readBatch = 0;
						return {
							readEntries: (cb: (entries: unknown[]) => void) => {
								if (readBatch === 0) {
									readBatch++;
									cb([
										{
											isFile: true,
											isDirectory: false,
											name: 'inside.txt',
											file: (fileCb: (f: File) => void) =>
												fileCb(new File(['hello inside dir'], 'inside.txt', { type: 'text/plain' }))
										},
										{
											isFile: true,
											isDirectory: false,
											name: '.DS_Store',
											file: (fileCb: (f: File) => void) =>
												fileCb(new File(['junk'], '.DS_Store', { type: 'text/plain' }))
										},
										{
											isFile: false,
											isDirectory: true,
											name: 'empty-sub',
											createReader: () => ({
												readEntries: (emptyCb: (entries: unknown[]) => void) => emptyCb([])
											})
										}
									]);
								} else {
									cb([]);
								}
							}
						};
					}
				} as unknown as FileSystemEntry;
			};
			const dt = new DataTransfer();
			const dummy = new File([''], 'dropped-dir', { type: '' });
			dt.items.add(dummy);
			window.dispatchEvent(new DragEvent('drop', { dataTransfer: dt, bubbles: true }));
			DataTransferItem.prototype.webkitGetAsEntry = originalGetAsEntry;
		});
		await expect(itemRow(page, 'dropped-dir')).toBeVisible();
		await page
			.getByTestId('upload-queue')
			.getByRole('button', { name: 'Tutup notifikasi unggahan' })
			.click();
		const droppedDirListing = await listing(page);
		const droppedDirEntry = droppedDirListing.items.find(
			(item) => item.name === 'dropped-dir' && item.kind === 'folder'
		);
		expect(droppedDirEntry).toBeTruthy();
		const droppedDirChildren = await listing(page, droppedDirEntry!.id);
		expect(droppedDirChildren.items.map((i) => i.name)).toContain('inside.txt');
		expect(droppedDirChildren.items.map((i) => i.name)).toContain('empty-sub');
		expect(droppedDirChildren.items.map((i) => i.name)).not.toContain('.DS_Store');

		// Select a real directory so the browser supplies webkitRelativePath values.
		const sourceFolder = join(base, 'project');
		mkdirSync(join(sourceFolder, 'nested'), { recursive: true });
		writeFileSync(join(sourceFolder, 'root.txt'), 'root file');
		writeFileSync(join(sourceFolder, 'nested', 'child.txt'), 'nested file');
		await page.getByTestId('upload-folder-input').setInputFiles(sourceFolder);
		await expect(itemRow(page, 'project')).toBeVisible();
		root = await listing(page);
		const project = root.items.find((item) => item.name === 'project' && item.kind === 'folder');
		expect(project).toBeTruthy();
		let projectContents = await listing(page, project!.id);
		expect(projectContents.items.map((item) => item.name)).toContain('root.txt');
		const nested = projectContents.items.find(
			(item) => item.name === 'nested' && item.kind === 'folder'
		);
		expect(nested).toBeTruthy();
		expect((await listing(page, nested!.id)).items.map((item) => item.name)).toContain('child.txt');
		await page
			.getByTestId('upload-queue')
			.getByRole('button', { name: 'Tutup notifikasi unggahan' })
			.click();
		const partialFolder = join(base, 'partial-folder');
		mkdirSync(partialFolder);
		writeFileSync(join(partialFolder, 'kept.txt'), 'persist this file');
		writeFileSync(join(partialFolder, 'failed.txt'), 'upload-failure-sentinel');
		await page.route('**/api/v1/drive/my-storage/upload**', async (route) => {
			if (new URL(route.request().url()).searchParams.get('filename') === 'failed.txt') {
				await route.fulfill({
					status: 400,
					contentType: 'application/json',
					body: JSON.stringify({ error: 'upload_rejected' })
				});
				return;
			}
			await route.continue();
		});
		await page.getByTestId('upload-folder-input').setInputFiles(partialFolder);
		await expect(page.getByText('1 unggahan gagal')).toBeVisible({ timeout: 15_000 });
		const warningToast = page.locator('[data-sonner-toast][data-type="warning"]').first();
		await expect(warningToast).toBeVisible();
		const warningStyles = await warningToast.evaluate((el) => {
			const s = window.getComputedStyle(el);
			return { bg: s.backgroundColor, border: s.borderColor };
		});
		expect(warningStyles.bg).toBe('rgb(255, 255, 255)');
		expect(warningStyles.border).toBe('rgb(188, 108, 37)');
		const partialQueue = page.getByTestId('upload-queue');
		await partialQueue.getByRole('button', { name: 'Buka antrean unggahan' }).click();
		await expect(
			partialQueue.getByTestId('upload-queue-item').filter({ hasText: 'failed.txt' })
		).toBeVisible();
		const failedProgress = await partialQueue
			.getByRole('progressbar')
			.getAttribute('aria-valuenow');
		expect(Number(failedProgress)).toBeLessThan(100);
		await partialQueue.getByRole('button', { name: 'Tutup notifikasi unggahan' }).click();
		await page.unroute('**/api/v1/drive/my-storage/upload**');
		const partialRoot = await listing(page);
		const partialEntry = partialRoot.items.find(
			(item) => item.kind === 'folder' && item.name === 'partial-folder'
		);
		expect(partialEntry).toBeTruthy();
		const partialContents = await listing(page, partialEntry!.id);
		expect(partialContents.items.map((item) => item.name)).toContain('kept.txt');
		expect(partialContents.items.map((item) => item.name)).not.toContain('failed.txt');

		await openActions(page, 'project');
		const folderDownload = page.waitForEvent('download');
		await page.getByRole('menuitem', { name: 'Unduh' }).click();
		const downloadedZip = await folderDownload;
		const zipPath = join(base, 'project.zip');
		await downloadedZip.saveAs(zipPath);
		expect(readFileSync(zipPath).subarray(0, 2).toString()).toBe('PK');
		expect(downloadedZip.suggestedFilename()).toMatch(/\.zip$/i);

		// Rename and move the nested file through the visible action dialogs.
		await itemRow(page, 'project').dblclick();
		await page.waitForURL(/folder=/);
		await openActions(page, 'root.txt');
		await page.getByRole('menuitem', { name: 'Ganti nama' }).click();
		await page.getByRole('textbox', { name: 'Nama' }).fill('renamed.txt');
		await page.getByRole('button', { name: 'Simpan' }).click();
		await expect(itemRow(page, 'renamed.txt')).toBeVisible();
		projectContents = await listing(page, project!.id);
		const renamed = projectContents.items.find((item) => item.name === 'renamed.txt');
		expect(renamed).toBeTruthy();

		await openActions(page, 'renamed.txt');
		await page.getByRole('menuitem', { name: 'Pindahkan ke' }).click();
		const moveDialog1 = page.getByTestId('move-dialog');
		await expect(moveDialog1).toBeVisible();
		await expect(moveDialog1.getByRole('heading', { name: 'Pindahkan' })).toBeVisible();
		await expect(moveDialog1.getByText('renamed.txt')).toBeVisible();
		await expect(moveDialog1.getByTestId('move-tab-organisasi')).toBeDisabled();
		const moveTabs = moveDialog1.getByRole('tablist');
		const driveTab = moveTabs.getByRole('tab', { name: 'Drive Saya' });
		const organizationTab = moveTabs.getByRole('tab', { name: 'Organisasi' });
		await expect(moveTabs).toBeVisible();
		await expect(driveTab).toHaveAttribute('aria-selected', 'true');
		await expect(organizationTab).toHaveAttribute('aria-selected', 'false');
		await expect(organizationTab).toBeDisabled();
		await expectTabColors(driveTab, true);
		await expectTabColors(organizationTab, false);
		await driveTab.click();
		await expect(driveTab).toHaveAttribute('aria-selected', 'true');
		await expect(organizationTab).toHaveAttribute('aria-selected', 'false');
		await expectTabColors(driveTab, true);
		await expectTabColors(organizationTab, false);
		await expect(moveDialog1.getByText('Drive Saya', { exact: true })).toBeVisible();
		// Drill down into project folder
		await moveDialog1.getByTestId(`move-folder-${project!.id}`).click();
		// Click Pindahkan on nested folder row
		await moveDialog1.getByTestId(`btn-move-to-${nested!.id}`).click();
		await expect(moveDialog1).not.toBeVisible();
		await itemRow(page, 'nested').dblclick();
		await page.waitForURL(/folder=/);

		await openActions(page, 'renamed.txt');
		await page.getByRole('menuitem', { name: 'Ganti nama' }).click();
		await page.getByRole('textbox', { name: 'Nama' }).fill('child.txt');
		await page.getByRole('button', { name: 'Simpan' }).click();
		const renameModal = page.getByTestId('confirm-dialog');
		await expect(renameModal).toBeVisible();
		expect(await renameModal.textContent()).toContain('child');
		expect(await renameModal.textContent()).toContain('(1)');
		await renameModal.getByRole('button', { name: 'Gunakan Nama Ini' }).click();
		await expect(itemRow(page, 'child (1).txt')).toBeVisible();
		expect((await listing(page, nested!.id)).items.map((item) => item.name)).toContain(
			'child (1).txt'
		);
		const moveConflictFile = join(base, 'child.txt');
		writeFileSync(moveConflictFile, 'move collision');
		await page.goto('/drive/my-storage');
		await page.getByTestId('upload-file-input').setInputFiles(moveConflictFile);
		await expect(itemRow(page, 'child.txt')).toBeVisible();
		await openActions(page, 'child.txt');
		await page.getByRole('menuitem', { name: 'Pindahkan ke' }).click();
		const moveDialog2 = page.getByTestId('move-dialog');
		await expect(moveDialog2).toBeVisible();
		// Drill down into project folder
		await moveDialog2.getByTestId(`move-folder-${project!.id}`).click();
		// Drill down into nested folder
		await moveDialog2.getByTestId(`move-folder-${nested!.id}`).click();
		// Click Pindahkan in footer while inside nested folder
		await moveDialog2.getByTestId('btn-move-footer').click();
		const moveModal = page.getByTestId('confirm-dialog');
		await expect(moveModal).toBeVisible();
		expect(await moveModal.textContent()).toContain('child');
		expect(await moveModal.textContent()).toContain('(2)');
		await moveModal.getByRole('button', { name: 'Gunakan Nama Ini' }).click();
		await expect
			.poll(async () => (await listing(page, nested!.id)).items.map((item) => item.name))
			.toContain('child (2).txt');

		// Duplicate the folder and verify its descendants exist in the real API listing.
		await page.goto('/drive/my-storage');
		await openActions(page, 'project');
		await page.getByRole('menuitem', { name: 'Duplikat' }).click();
		await expect.poll(async () => (await listing(page)).items.length).toBeGreaterThan(3);
		root = await listing(page);
		const duplicate = root.items.find(
			(item) => item.kind === 'folder' && item.name !== 'project' && /project/.test(item.name)
		);
		expect(duplicate).toBeTruthy();
		const duplicateItems = await listing(page, duplicate!.id);
		const duplicateNested = duplicateItems.items.find(
			(item) => item.kind === 'folder' && item.name === 'nested'
		);
		expect(duplicateNested).toBeTruthy();
		const duplicateNestedItems = await listing(page, duplicateNested!.id);
		expect(duplicateNestedItems.items.map((item) => item.name)).toContain('child (2).txt');
		const duplicateChildId = duplicateNestedItems.items.find(
			(item) => item.name === 'child (1).txt'
		)?.id;
		expect(duplicateChildId).toBeTruthy();

		// Folder deletion must disclose recursive impact and invoke the native confirmation.
		await openActions(page, duplicate!.name);
		await page.getByRole('menuitem', { name: 'Hapus ke Trash' }).click();
		const deleteModal = page.getByTestId('confirm-dialog');
		await expect(deleteModal).toBeVisible();
		const deleteText = await deleteModal.textContent();
		expect(deleteText).toContain('dan semua isinya');
		expect(deleteText).toContain(duplicate!.name);
		await deleteModal.getByRole('button', { name: 'Hapus ke Trash' }).click();
		await expect
			.poll(async () => (await listing(page)).items.some((item) => item.id === duplicate!.id))
			.toBe(false);
		const deletedChild = await page.request.get(
			`/api/v1/drive/my-storage/${encodeURIComponent(duplicateChildId!)}/download`
		);
		expect(deletedChild.ok()).toBe(false);

		// Trash Lifecycle Verification
		// 1. Soft-delete dropped-page.txt to Trash so we have multiple items in Trash
		await openActions(page, 'dropped-page.txt');
		await page.getByRole('menuitem', { name: 'Hapus ke Trash' }).click();
		const deleteDroppedModal = page.getByTestId('confirm-dialog');
		await expect(deleteDroppedModal).toBeVisible();
		await deleteDroppedModal.getByRole('button', { name: 'Hapus ke Trash' }).click();
		await expect(itemRow(page, 'dropped-page.txt')).not.toBeVisible();

		// 2. Navigate to /drive/trash
		await page.goto('/drive/trash');
		await expect(page.getByRole('heading', { name: 'Trash' })).toBeVisible();

		// Verify ordering: dropped-page.txt was deleted after duplicate folder, so dropped-page.txt appears first
		const trashRows = page.getByRole('option');
		await expect(trashRows.first()).toContainText('dropped-page.txt');
		await expect(itemRow(page, duplicate!.name)).toBeVisible();

		// 3. Test Restore: restore duplicate folder
		const dupRow = itemRow(page, duplicate!.name);
		await dupRow.getByRole('button', { name: 'Aksi trash' }).click();
		await page.getByRole('menuitem', { name: 'Pulihkan' }).click();
		await expect(dupRow).not.toBeVisible();

		// Verify duplicate folder is back in My Storage
		await page.goto('/drive/my-storage');
		await expect(itemRow(page, duplicate!.name)).toBeVisible();

		// 4. Test Single Permanent Delete on dropped-page.txt
		await page.goto('/drive/trash');
		const droppedTrashRow = page.getByRole('option').filter({ hasText: 'dropped-page.txt' });
		await expect(droppedTrashRow).toBeVisible();
		await droppedTrashRow.getByRole('button', { name: 'Aksi trash' }).click();
		await page.getByRole('menuitem', { name: 'Hapus permanen' }).click();
		const singlePurgeModal = page.getByTestId('confirm-dialog');
		await expect(singlePurgeModal).toBeVisible();
		await singlePurgeModal.getByRole('button', { name: 'Hapus permanen' }).click();
		await expect(droppedTrashRow).not.toBeVisible();
		// 5. Test Empty Trash (Kosongkan Sampah)
		await page.goto('/drive/my-storage');
		await openActions(page, duplicate!.name);
		await page.getByRole('menuitem', { name: 'Hapus ke Trash' }).click();
		await page
			.getByTestId('confirm-dialog')
			.getByRole('button', { name: 'Hapus ke Trash' })
			.click();

		await page.goto('/drive/trash');
		await expect(itemRow(page, duplicate!.name)).toBeVisible();
		await page.getByTestId('empty-trash-btn').click();
		const emptyModal = page.getByRole('dialog');
		await expect(emptyModal).toBeVisible();
		await emptyModal.getByRole('button', { name: 'Hapus permanen semua' }).click();
		await expect(page.getByText('Trash kosong')).toBeVisible();

		// 6. Test Restore with Missing Parent Fallback to Root
		await page.goto('/drive/my-storage');
		await page.getByRole('button', { name: 'Tambah' }).click();
		await page.getByRole('menuitem', { name: 'Folder Baru' }).click();
		await page.getByLabel('Nama folder').fill('temp-parent');
		await page.getByRole('button', { name: 'Buat' }).click();
		await expect(itemRow(page, 'temp-parent')).toBeVisible();

		await itemRow(page, 'temp-parent').dblclick();
		await page.waitForURL(/folder=/);

		const fallbackFile = join(base, 'fallback-doc.txt');
		writeFileSync(fallbackFile, 'fallback content');
		await page.getByTestId('upload-file-input').setInputFiles(fallbackFile);
		await expect(itemRow(page, 'fallback-doc.txt')).toBeVisible();

		await openActions(page, 'fallback-doc.txt');
		await page.getByRole('menuitem', { name: 'Hapus ke Trash' }).click();
		await page
			.getByTestId('confirm-dialog')
			.getByRole('button', { name: 'Hapus ke Trash' })
			.click();
		await expect(itemRow(page, 'fallback-doc.txt')).not.toBeVisible();

		await page.goto('/drive/my-storage');
		await openActions(page, 'temp-parent');
		await page.getByRole('menuitem', { name: 'Hapus ke Trash' }).click();
		await page
			.getByTestId('confirm-dialog')
			.getByRole('button', { name: 'Hapus ke Trash' })
			.click();

		await page.goto('/drive/trash');
		const fallbackTrashRow = itemRow(page, 'fallback-doc.txt');
		await expect(fallbackTrashRow).toBeVisible();
		await fallbackTrashRow.getByRole('button', { name: 'Aksi trash' }).click();
		await page.getByRole('menuitem', { name: 'Pulihkan' }).click();

		const missingParentModal = page.getByRole('dialog');
		await expect(missingParentModal).toBeVisible();
		await expect(missingParentModal).toContainText('Folder asal tidak ditemukan');
		await missingParentModal.getByRole('button', { name: 'Pulihkan ke Root' }).click();
		await expect(fallbackTrashRow).not.toBeVisible();

		await page.goto('/drive/my-storage');
		await expect(itemRow(page, 'fallback-doc.txt')).toBeVisible();

		// 7. Test Drag-and-Drop Move & Conflict Detection
		await page.getByRole('button', { name: 'Tambah' }).click();
		await page.getByRole('menuitem', { name: 'Folder Baru' }).click();
		await page.getByLabel('Nama folder').fill('move-target');
		await page.getByRole('button', { name: 'Buat' }).click();
		await expect(itemRow(page, 'move-target')).toBeVisible();

		const moveFile = join(base, 'move-sample.txt');
		writeFileSync(moveFile, 'move sample content');
		await page.getByTestId('upload-file-input').setInputFiles(moveFile);
		await expect(itemRow(page, 'move-sample.txt')).toBeVisible();

		// Drag move-sample.txt into move-target
		const sourceRow = page.locator('[draggable="true"]').filter({ hasText: 'move-sample.txt' });
		const targetRow = page.locator('[data-folder-id]').filter({ hasText: 'move-target' });
		await sourceRow.dragTo(targetRow);
		await expect(itemRow(page, 'move-sample.txt')).not.toBeVisible();

		// Verify item is inside move-target
		await itemRow(page, 'move-target').dblclick();
		await page.waitForURL(/folder=/);
		await expect(itemRow(page, 'move-sample.txt')).toBeVisible();

		// Create conflict-sample.txt inside move-target
		const conflictTargetFile = join(base, 'conflict-sample.txt');
		writeFileSync(conflictTargetFile, 'inside move target');
		await page.getByTestId('upload-file-input').setInputFiles(conflictTargetFile);
		await expect(itemRow(page, 'conflict-sample.txt')).toBeVisible();

		// Go back to root and upload another conflict-sample.txt
		await page.goto('/drive/my-storage');
		const conflictRootFile = join(base, 'conflict-sample-root.txt');
		writeFileSync(conflictRootFile, 'in root storage');
		// Rename to conflict-sample.txt in root
		const rootUploadFile = join(base, 'conflict-sample.txt');
		writeFileSync(rootUploadFile, 'conflict root content');
		await page.getByTestId('upload-file-input').setInputFiles(rootUploadFile);
		await expect(itemRow(page, 'conflict-sample.txt')).toBeVisible();

		// Drag conflict-sample.txt onto move-target to trigger conflict dialog
		const conflictSource = page
			.locator('[draggable="true"]')
			.filter({ hasText: 'conflict-sample.txt' });
		await conflictSource.dragTo(targetRow);

		// Expect conflict dialog "Nama Sudah Digunakan"
		const conflictModal = page.getByRole('dialog');
		await expect(conflictModal).toBeVisible();
		await expect(conflictModal).toContainText('Nama Sudah Digunakan');
		await conflictModal.getByRole('button', { name: 'Batal' }).click();
		await expect(conflictModal).not.toBeVisible();
		await expect(itemRow(page, 'conflict-sample.txt')).toBeVisible();

		// 8. Test Starred (Bintang) Lifecycle
		// Star move-target folder and conflict-sample.txt from my-storage
		await openActions(page, 'move-target');
		await page.getByRole('menuitem', { name: 'Bintang' }).click();

		await openActions(page, 'conflict-sample.txt');
		await page.getByRole('menuitem', { name: 'Bintang' }).click();

		// Navigate to /drive/starred
		await page.goto('/drive/starred');
		await expect(itemRow(page, 'move-target')).toBeVisible();
		await expect(itemRow(page, 'conflict-sample.txt')).toBeVisible();

		// Test Folder Navigation from Starred
		await itemRow(page, 'move-target').dblclick();
		await page.waitForURL(/\/drive\/my-storage\?folder=/);
		await expect(itemRow(page, 'move-sample.txt')).toBeVisible();

		// Return to Starred
		await page.goto('/drive/starred');
		await expect(itemRow(page, 'conflict-sample.txt')).toBeVisible();

		// Test Download from Starred
		const downloadPromise = page.waitForEvent('download');
		await openActions(page, 'conflict-sample.txt');
		await page.getByRole('menuitem', { name: 'Unduh' }).click();
		const download = await downloadPromise;
		expect(download.suggestedFilename()).toBe('conflict-sample.txt');

		// Test Unstar from Starred
		const conflictStarredRow = itemRow(page, 'conflict-sample.txt');
		await conflictStarredRow.getByRole('button', { name: 'Hapus Bintang' }).click();
		await expect(conflictStarredRow).not.toBeVisible();

		// Test Delete from Starred
		const targetStarredRow = itemRow(page, 'move-target');
		await expect(targetStarredRow).toBeVisible();
		await targetStarredRow.getByRole('button', { name: 'Aksi berkas' }).click();
		await page.getByRole('menuitem', { name: 'Hapus ke Trash' }).click();

		const deleteConfirmModal = page.getByRole('dialog');
		await expect(deleteConfirmModal).toBeVisible();
		await deleteConfirmModal.getByRole('button', { name: 'Hapus ke Trash' }).click();
		await expect(targetStarredRow).not.toBeVisible();

		// Verify move-target is in Trash
		await page.goto('/drive/trash');
		await expect(itemRow(page, 'move-target')).toBeVisible();
	} finally {
		rmSync(base, { recursive: true, force: true });
	}
});

// Import via Vite in the browser; the Playwright Node runner cannot compile this Svelte-rune module.
test('the same persisted upload cannot run in two browser tabs at once', async ({ page }) => {
	await finishSetup(page);
	const ownerId = await page.evaluate(async () => {
		const queue = await import('/src/lib/transfer-queue.svelte.ts');
		return queue.transferQueueState.ownerId;
	});
	expect(ownerId).toBeTruthy();

	const transferId = await page.evaluate(async () => {
		const queue = await import('/src/lib/transfer-queue.svelte.ts');
		const file = new File(['cross-tab upload'], 'cross-tab.txt', { lastModified: 123 });
		const [id] = await queue.enqueueTransfers(queue.transferQueueState.ownerId, 'my-storage', [
			{ name: file.name, relativePath: file.name, parentId: null, file }
		]);
		let releaseDriver: (() => void) | undefined;
		let signalStarted: (() => void) | undefined;
		const started = new Promise<void>((resolve) => (signalStarted = resolve));
		(
			window as typeof window & {
				__releaseTransferDriver?: () => void;
				__transferDriverPromise?: Promise<boolean>;
			}
		).__releaseTransferDriver = () => releaseDriver?.();
		(
			window as typeof window & { __transferDriverPromise?: Promise<boolean> }
		).__transferDriverPromise = queue.runTransfer(
			id,
			file,
			async (_file, _record, options) => {
				await options.onPhase('uploading');
				signalStarted?.();
				await new Promise<void>((resolve) => (releaseDriver = resolve));
				await options.onPhase('committing');
			},
			async () => {}
		);
		await started;
		return id;
	});

	const secondTab = await page.context().newPage();
	try {
		await secondTab.goto('/drive/my-storage');
		const attempt = await secondTab.evaluate(
			async ({ id, ownerId }) => {
				const queue = await import('/src/lib/transfer-queue.svelte.ts');
				await queue.loadTransferQueue(ownerId, 'my-storage');
				const file = new File(['cross-tab upload'], 'cross-tab.txt', { lastModified: 123 });
				let driverCalled = false;
				const result = await queue.runTransfer(
					id,
					file,
					async () => {
						driverCalled = true;
					},
					async () => {}
				);
				return { result, driverCalled };
			},
			{ id: transferId, ownerId }
		);
		expect(attempt).toEqual({ result: false, driverCalled: false });
	} finally {
		await page.evaluate(async () => {
			const transferWindow = window as typeof window & {
				__releaseTransferDriver?: () => void;
				__transferDriverPromise?: Promise<boolean>;
			};
			transferWindow.__releaseTransferDriver?.();
			await transferWindow.__transferDriverPromise;
		});
		await secondTab.close();
	}
});

test('logout pauses an active upload and preserves it for the next login', async ({ page }) => {
	test.setTimeout(120_000);
	await finishSetup(page);

	const filename = 'logout-resume.bin';
	const original = Buffer.alloc(1024 * 1024, 0x6c);
	const sourceDir = mkdtempSync(join(tmpdir(), 'my-storage-logout-upload-'));
	const filePath = join(sourceDir, filename);
	writeFileSync(filePath, original);
	const network = await page.context().newCDPSession(page);
	await network.send('Network.enable');

	try {
		await network.send('Network.emulateNetworkConditions', {
			offline: false,
			latency: 25,
			downloadThroughput: 10 * 1024 * 1024,
			uploadThroughput: 64 * 1024
		});
		await page.getByTestId('upload-file-input').setInputFiles(filePath);
		const queue = page.getByTestId('upload-queue');
		await expect(queue).toBeVisible();
		const openQueue = queue.getByRole('button', { name: 'Buka antrean unggahan' });
		if (await openQueue.count()) await openQueue.click();
		const transfer = queue.getByTestId('upload-queue-item').filter({ hasText: filename });
		await expect(transfer).toHaveAttribute('data-state', 'uploading');
		await expect
			.poll(async () =>
				Number(
					await queue
						.getByRole('progressbar', { name: 'Progress unggahan' })
						.getAttribute('aria-valuenow')
				)
			)
			.toBeGreaterThan(0);

		await page.getByRole('button', { name: 'Menu akun' }).click();
		await page.getByRole('menuitem', { name: 'Logout' }).click();
		await page.waitForURL('**/login');
		await page.getByLabel('Username atau Email').fill(username);
		await page.getByRole('textbox', { name: 'Password' }).fill(password);
		await page.getByRole('button', { name: 'Masuk' }).click();
		await page.waitForURL('**/drive/home');
		await page.goto('/drive/my-storage');
		await page.waitForFunction(async () => {
			const transferQueue = await import('/src/lib/transfer-queue.svelte.ts');
			return (
				transferQueue.transferQueueState.ready && Boolean(transferQueue.transferQueueState.ownerId)
			);
		});

		const resumedQueue = page.getByTestId('upload-queue');
		await expect(resumedQueue).toBeVisible();
		const reopenQueue = resumedQueue.getByRole('button', { name: 'Buka antrean unggahan' });
		if (await reopenQueue.count()) await reopenQueue.click();
		const pausedTransfer = resumedQueue
			.getByTestId('upload-queue-item')
			.filter({ hasText: filename });
		await expect(pausedTransfer).toHaveAttribute('data-state', 'paused');

		await network.send('Network.emulateNetworkConditions', {
			offline: false,
			latency: 0,
			downloadThroughput: -1,
			uploadThroughput: -1
		});
		await pausedTransfer.getByRole('button', { name: `Lanjutkan: ${filename}` }).click();
		await page.getByTestId('resume-upload-file-input').setInputFiles(filePath);
		await expect(pausedTransfer).toHaveAttribute('data-state', 'completed', { timeout: 30_000 });
		const matches = (await listing(page)).items.filter((item) => item.name === filename);
		expect(matches).toHaveLength(1);
		expect(matches[0].size_bytes).toBe(original.length);
	} finally {
		await network
			.send('Network.emulateNetworkConditions', {
				offline: false,
				latency: 0,
				downloadThroughput: -1,
				uploadThroughput: -1
			})
			.catch(() => {});
		await network.detach().catch(() => {});
		rmSync(sourceDir, { recursive: true, force: true });
	}
});

test('resumable upload survives refresh, backend restart, and tab closure, then rejects a mismatched file', async ({
	page
}) => {
	test.setTimeout(120_000);
	await finishSetup(page);
	const restartResultPath = join(restartControlDir, 'restart-result.json');
	const restartRequestPath = join(restartControlDir, 'restart-request.json');
	const filename = 'resumable-recovery.bin';
	const original = Buffer.alloc(8 * 1024 * 1024, 0x61);
	const sourceDir = mkdtempSync(join(tmpdir(), 'my-storage-resume-upload-'));
	const originalDir = join(sourceDir, 'original');
	const mismatchedDir = join(sourceDir, 'mismatched');
	mkdirSync(originalDir);
	mkdirSync(mismatchedDir);
	const originalPath = join(originalDir, filename);
	const mismatchedPath = join(mismatchedDir, filename);
	writeFileSync(originalPath, original);
	writeFileSync(mismatchedPath, Buffer.alloc(original.length, 0x62));
	const modifiedAt = new Date('2026-01-01T00:00:00.000Z');
	utimesSync(originalPath, modifiedAt, modifiedAt);
	utimesSync(mismatchedPath, modifiedAt, modifiedAt);
	const browserContext = page.context();

	try {
		const network = await page.context().newCDPSession(page);
		await network.send('Network.enable');
		await network.send('Network.emulateNetworkConditions', {
			offline: false,
			latency: 25,
			downloadThroughput: 10 * 1024 * 1024,
			uploadThroughput: 256 * 1024
		});

		try {
			await page.getByTestId('upload-file-input').setInputFiles(originalPath);
			const queue = page.getByTestId('upload-queue');
			await expect(queue).toBeVisible();
			const openQueue = queue.getByRole('button', { name: 'Buka antrean unggahan' });
			if (await openQueue.count()) await openQueue.click();
			const transfer = queue.getByTestId('upload-queue-item').filter({ hasText: filename });
			await expect(transfer).toHaveAttribute('data-state', 'uploading', { timeout: 20_000 });
			const progress = queue.getByRole('progressbar', { name: 'Progress unggahan' });
			await expect
				.poll(async () => Number(await progress.getAttribute('aria-valuenow')))
				.toBeGreaterThan(0);
			await expect
				.poll(async () => Number(await progress.getAttribute('aria-valuenow')))
				.toBeLessThan(100);
			await transfer.getByRole('button', { name: `Jeda ${filename}` }).click();
			await expect(transfer).toHaveAttribute('data-state', 'paused');
		} finally {
			await network
				.send('Network.emulateNetworkConditions', {
					offline: false,
					latency: 0,
					downloadThroughput: -1,
					uploadThroughput: -1
				})
				.catch(() => {});
			await network.detach().catch(() => {});
		}

		await page.reload();
		await page.waitForFunction(async () => {
			const queue = await import('/src/lib/transfer-queue.svelte.ts');
			return queue.transferQueueState.ready && Boolean(queue.transferQueueState.ownerId);
		});
		const queue = page.getByTestId('upload-queue');
		await expect(queue).toBeVisible();
		const openQueue = queue.getByRole('button', { name: 'Buka antrean unggahan' });
		if (await openQueue.count()) await openQueue.click();
		const transfer = queue.getByTestId('upload-queue-item').filter({ hasText: filename });
		await expect(transfer).toHaveAttribute('data-state', 'paused');
		const persistedFingerprint = await page.evaluate(async (name) => {
			const { transferQueueState } = await import('/src/lib/transfer-queue.svelte.ts');
			return transferQueueState.items.find((item) => item.name === name)?.fingerprint;
		}, filename);
		expect(persistedFingerprint).toMatch(/^[a-f\d]{64}$/i);
		const fileAvailableWithoutReselection = await page.evaluate(async (name) => {
			const { transferQueueState, getInMemoryTransferFile } =
				await import('/src/lib/transfer-queue.svelte.ts');
			const record = transferQueueState.items.find((item) => item.name === name);
			return record ? Boolean(getInMemoryTransferFile(record.id)) : null;
		}, filename);
		expect(fileAvailableWithoutReselection).toBe(false);

		await expect.poll(() => existsSync(restartResultPath), { timeout: 120_000 }).toBeTruthy();
		const previousBackend = JSON.parse(readFileSync(restartResultPath, 'utf8')) as {
			generation: number;
			pid: number;
		};
		const requestId = `restart-${Date.now()}`;
		const temporaryRequestPath = `${restartRequestPath}.${process.pid}.tmp`;
		writeFileSync(temporaryRequestPath, JSON.stringify({ requestId }));
		renameSync(temporaryRequestPath, restartRequestPath);
		await expect
			.poll(
				() => {
					const result = JSON.parse(readFileSync(restartResultPath, 'utf8')) as {
						requestId: string | null;
						generation: number;
						pid: number;
					};
					return result.requestId === requestId ? result : null;
				},
				{ timeout: 120_000 }
			)
			.toMatchObject({ requestId, generation: previousBackend.generation + 1 });
		const restartedBackend = JSON.parse(readFileSync(restartResultPath, 'utf8')) as {
			pid: number;
		};
		expect(restartedBackend.pid).not.toBe(previousBackend.pid);
		await expect
			.poll(async () => (await page.request.get('/api/v1/setup/status')).ok(), { timeout: 120_000 })
			.toBeTruthy();

		await page.close();
		const reopenedPage = await browserContext.newPage();
		await reopenedPage.goto('/drive/my-storage');
		await reopenedPage.waitForFunction(async () => {
			const queue = await import('/src/lib/transfer-queue.svelte.ts');
			return queue.transferQueueState.ready && Boolean(queue.transferQueueState.ownerId);
		});
		await expect(reopenedPage.getByRole('heading', { name: 'My Storage' })).toBeVisible();
		const reopenedQueue = reopenedPage.getByTestId('upload-queue');
		await expect(reopenedQueue).toBeVisible();
		const reopenQueue = reopenedQueue.getByRole('button', { name: 'Buka antrean unggahan' });
		await expect(reopenQueue).toBeVisible();
		await reopenQueue.click();
		const reopenedTransfer = reopenedQueue
			.getByTestId('upload-queue-item')
			.filter({ hasText: filename });
		await expect(reopenedTransfer).toHaveAttribute('data-state', 'paused');

		await reopenedTransfer.getByRole('button', { name: `Lanjutkan: ${filename}` }).click();
		await reopenedPage.getByTestId('resume-upload-file-input').setInputFiles(mismatchedPath);
		await expect(reopenedTransfer).toHaveAttribute('data-state', 'waiting-for-file', {
			timeout: 30_000
		});
		await reopenedTransfer.getByRole('button', { name: `Pilih file: ${filename}` }).click();
		await reopenedPage.getByTestId('resume-upload-file-input').setInputFiles(originalPath);
		await expect(reopenedTransfer).toHaveAttribute('data-state', 'completed', { timeout: 30_000 });

		const matches = (await listing(reopenedPage)).items.filter((item) => item.name === filename);
		expect(matches).toHaveLength(1);
		expect(matches[0].size_bytes).toBe(original.length);
		const download = await reopenedPage.request.get(
			`/api/v1/drive/my-storage/${matches[0].id}/download`
		);
		expect(download.ok()).toBeTruthy();
		expect(await download.body()).toEqual(original);
	} finally {
		rmSync(sourceDir, { recursive: true, force: true });
	}
});

test('paused upload survives closing and reopening the browser', async ({ page }) => {
	test.setTimeout(120_000);
	await finishSetup(page);

	const sourceDir = mkdtempSync(join(tmpdir(), 'my-storage-browser-reopen-'));
	const profileDir = join(sourceDir, 'profile');
	const filename = 'browser-reopen-resume.bin';
	const original = Buffer.alloc(8 * 1024 * 1024, 0x63);
	const originalPath = join(sourceDir, filename);
	writeFileSync(originalPath, original);
	const baseURL = new URL(page.url()).origin;
	const launchBrowser = () =>
		chromium.launchPersistentContext(profileDir, {
			baseURL,
			headless: true,
			viewport: { width: 1280, height: 720 }
		});
	let browserContext: BrowserContext | undefined;

	try {
		browserContext = await launchBrowser();
		let browserPage = browserContext.pages()[0] ?? (await browserContext.newPage());
		await browserPage.goto('/login');
		await browserPage.getByLabel('Username atau Email').fill(username);
		await browserPage.getByRole('textbox', { name: 'Password' }).fill(password);
		await browserPage.getByRole('button', { name: 'Masuk' }).click();
		await browserPage.waitForURL('**/drive/home');
		await browserPage.goto('/drive/my-storage');
		await browserPage.waitForFunction(async () => {
			const queue = await import('/src/lib/transfer-queue.svelte.ts');
			return queue.transferQueueState.ready && Boolean(queue.transferQueueState.ownerId);
		});

		const network = await browserContext.newCDPSession(browserPage);
		await network.send('Network.enable');
		try {
			await network.send('Network.emulateNetworkConditions', {
				offline: false,
				latency: 25,
				downloadThroughput: 10 * 1024 * 1024,
				uploadThroughput: 256 * 1024
			});
			await browserPage.getByTestId('upload-file-input').setInputFiles(originalPath);
			const queue = browserPage.getByTestId('upload-queue');
			await expect(queue).toBeVisible();
			const openQueue = queue.getByRole('button', { name: 'Buka antrean unggahan' });
			if (await openQueue.count()) await openQueue.click();
			const transfer = queue.getByTestId('upload-queue-item').filter({ hasText: filename });
			await expect(transfer).toHaveAttribute('data-state', 'uploading', { timeout: 20_000 });
			const progress = queue.getByRole('progressbar', { name: 'Progress unggahan' });
			await expect
				.poll(async () => Number(await progress.getAttribute('aria-valuenow')))
				.toBeGreaterThan(0);
			await expect
				.poll(async () => Number(await progress.getAttribute('aria-valuenow')))
				.toBeLessThan(100);
			await transfer.getByRole('button', { name: `Jeda ${filename}` }).click();
			await expect(transfer).toHaveAttribute('data-state', 'paused');
		} finally {
			await network
				.send('Network.emulateNetworkConditions', {
					offline: false,
					latency: 0,
					downloadThroughput: -1,
					uploadThroughput: -1
				})
				.catch(() => {});
			await network.detach().catch(() => {});
		}

		await browserContext.close();
		browserContext = await launchBrowser();
		browserPage = browserContext.pages()[0] ?? (await browserContext.newPage());
		await browserPage.goto('/drive/my-storage');
		const loginForm = browserPage.getByLabel('Username atau Email');
		if (await loginForm.count()) {
			await loginForm.fill(username);
			await browserPage.getByRole('textbox', { name: 'Password' }).fill(password);
			await browserPage.getByRole('button', { name: 'Masuk' }).click();
			await browserPage.waitForURL('**/drive/home');
			await browserPage.goto('/drive/my-storage');
		}
		await browserPage.waitForFunction(async () => {
			const queue = await import('/src/lib/transfer-queue.svelte.ts');
			return queue.transferQueueState.ready && Boolean(queue.transferQueueState.ownerId);
		});
		await expect(browserPage.getByRole('heading', { name: 'My Storage' })).toBeVisible();
		const queue = browserPage.getByTestId('upload-queue');
		await expect(queue).toBeVisible();
		const openQueue = queue.getByRole('button', { name: 'Buka antrean unggahan' });
		if (await openQueue.count()) await openQueue.click();
		const transfer = queue.getByTestId('upload-queue-item').filter({ hasText: filename });
		await expect(transfer).toHaveAttribute('data-state', 'paused');

		const fingerprint = await browserPage.evaluate(async (name) => {
			const { transferQueueState } = await import('/src/lib/transfer-queue.svelte.ts');
			return transferQueueState.items.find((item) => item.name === name)?.fingerprint;
		}, filename);
		expect(fingerprint).toMatch(/^[a-f\d]{64}$/i);
		await transfer.getByRole('button', { name: `Lanjutkan: ${filename}` }).click();
		await browserPage.getByTestId('resume-upload-file-input').setInputFiles(originalPath);
		await expect(transfer).toHaveAttribute('data-state', 'completed', { timeout: 30_000 });

		const matches = (await listing(browserPage)).items.filter((item) => item.name === filename);
		expect(matches).toHaveLength(1);
		expect(matches[0].size_bytes).toBe(original.length);
		const download = await browserPage.request.get(
			`/api/v1/drive/my-storage/${matches[0].id}/download`
		);
		expect(download.ok()).toBeTruthy();
		expect(await download.body()).toEqual(original);
	} finally {
		await browserContext?.close().catch(() => {});
		rmSync(sourceDir, { recursive: true, force: true });
	}
});
test('cancelling an active TUS upload deletes its server session without creating a file', async ({
	page
}) => {
	test.setTimeout(120_000);
	await finishSetup(page);

	const filename = 'cancelled-resumable.bin';
	const sourceDir = mkdtempSync(join(tmpdir(), 'my-storage-cancel-upload-'));
	const filePath = join(sourceDir, filename);
	writeFileSync(filePath, Buffer.alloc(4 * 1024 * 1024, 0x63));
	const network = await page.context().newCDPSession(page);
	await network.send('Network.enable');

	try {
		await network.send('Network.emulateNetworkConditions', {
			offline: false,
			latency: 25,
			downloadThroughput: 10 * 1024 * 1024,
			uploadThroughput: 256 * 1024
		});

		const createResponsePromise = page.waitForResponse(
			(response) =>
				response.url().includes('/api/v1/drive/my-storage/upload-tus') &&
				response.request().method() === 'POST'
		);
		await page.getByTestId('upload-file-input').setInputFiles(filePath);
		const createResponse = await createResponsePromise;
		expect(createResponse.status()).toBe(201);
		const uploadLocation = createResponse.headers()['location'];
		expect(uploadLocation).toBeTruthy();

		const queue = page.getByTestId('upload-queue');
		await expect(queue).toBeVisible();
		const openQueue = queue.getByRole('button', { name: 'Buka antrean unggahan' });
		if (await openQueue.count()) await openQueue.click();
		const transfer = queue.getByTestId('upload-queue-item').filter({ hasText: filename });
		await expect(transfer).toHaveAttribute('data-state', 'uploading', { timeout: 20_000 });
		await expect
			.poll(async () =>
				Number(
					await queue
						.getByRole('progressbar', { name: 'Progress unggahan' })
						.getAttribute('aria-valuenow')
				)
			)
			.toBeGreaterThan(0);

		const deleteResponsePromise = page.waitForResponse(
			(response) =>
				response.url().includes('/api/v1/drive/my-storage/upload-tus/') &&
				response.request().method() === 'DELETE'
		);
		await transfer.getByRole('button', { name: `Batalkan ${filename}` }).click();
		expect((await deleteResponsePromise).status()).toBe(204);
		await expect(transfer).toHaveCount(0);

		const missingSession = await page.request.head(uploadLocation, {
			headers: { 'Tus-Resumable': '1.0.0' }
		});
		expect(missingSession.status()).toBe(404);
		expect((await listing(page)).items.filter((item) => item.name === filename)).toHaveLength(0);
	} finally {
		await network
			.send('Network.emulateNetworkConditions', {
				offline: false,
				latency: 0,
				downloadThroughput: -1,
				uploadThroughput: -1
			})
			.catch(() => {});
		await network.detach().catch(() => {});
		rmSync(sourceDir, { recursive: true, force: true });
	}
});
