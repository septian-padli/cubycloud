import { writeFileSync, mkdirSync } from 'node:fs';
import { join } from 'node:path';
import { tmpdir } from 'node:os';
import { expect, test, type Page } from '@playwright/test';

const admin = { username: 'storage-e2e-admin', password: 'StorageE2E-password-2026' };

async function setup(page: Page) {
	const status = await page.request.get('/api/v1/setup/status');
	expect(status.ok()).toBeTruthy();
	if (!((await status.json()) as { is_setup_completed: boolean }).is_setup_completed) {
		const scan = await page.request.get('/api/v1/setup/scan-nodes');
		const { nodes } = (await scan.json()) as { nodes: { path: string; capacity_bytes: number }[] };
		const response = await page.request.post('/api/v1/setup/complete', {
			data: {
				admin: {
					...admin,
					email: 'topbar-upload@example.test',
					recovery_key: 'ABCDE-FGHKL-MNPQR-STUVW-XYZ23-45678'
				},
				storage_nodes: nodes.map((node) => ({ ...node, enabled: true })),
				placement_strategy: 'FILL_FIRST',
				cloud: { enabled: false }
			}
		});
		expect(response.ok()).toBeTruthy();
	}

	await page.context().clearCookies();
	await page.goto('/login');
	await page.locator('#login-identifier').fill(admin.username);
	await page.locator('#login-password').fill(admin.password);
	await page.getByRole('button', { name: /Masuk|Sign in/i }).click();
	await page.waitForURL('**/drive/home');
}

test('Topbar upload button automatically uploads files to the root of My Drive from any page or subfolder', async ({
	page
}) => {
	test.setTimeout(120_000);
	await setup(page);

	const tempDir = join(tmpdir(), `topbar-test-${Date.now()}`);
	mkdirSync(tempDir, { recursive: true });

	const file1 = join(tempDir, 'topbar-from-home.txt');
	writeFileSync(file1, 'Uploaded from home page via topbar');

	const file2 = join(tempDir, 'topbar-from-subfolder.txt');
	writeFileSync(file2, 'Uploaded from inside subfolder via topbar');

	const uploadFromHeader = async (filePath: string) => {
		const chooserPromise = page.waitForEvent('filechooser');
		await page.getByTestId('topbar-upload-button').click();
		await (await chooserPromise).setFiles(filePath);
	};
	// 1. On /drive/home, upload file1 using the topbar upload input
	await page.goto('/drive/home');
	await expect(page.getByRole('heading', { name: 'Beranda' })).toBeVisible();

	await uploadFromHeader(file1);
	await expect(page.getByText('“topbar-from-home.txt” berhasil diunggah')).toBeVisible();

	// 2. Go to /drive/my-storage and verify topbar-from-home.txt is in the root directory
	await page.goto('/drive/my-storage');
	await expect(page.getByText('topbar-from-home.txt')).toBeVisible();

	// 3. Create a subfolder in My Storage
	const folderRes = await page.request.post('/api/v1/drive/my-storage/folders', {
		data: { name: 'Subfolder Alpha', parent_id: null }
	});
	expect(folderRes.ok()).toBeTruthy();
	const subfolder = (await folderRes.json()) as { id: string };

	// Navigate into the subfolder
	await page.goto(`/drive/my-storage?folder=${subfolder.id}`);
	await expect(page.getByText('Subfolder Alpha')).toBeVisible();

	await uploadFromHeader(file2);
	await expect(page.getByText('“topbar-from-subfolder.txt” berhasil diunggah')).toBeVisible();

	// file2 must NOT be in the subfolder
	await expect(page.getByText('topbar-from-subfolder.txt')).toHaveCount(0);

	// 5. Navigate back to root of My Storage and verify file2 is present in the root
	await page.goto('/drive/my-storage');
	await expect(page.getByText('topbar-from-subfolder.txt')).toBeVisible();
});
