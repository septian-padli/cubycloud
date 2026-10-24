import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { expect, test, type Page } from '@playwright/test';

const admin = { username: 'storage-e2e-admin', password: 'StorageE2E-password-2026' };
const guestPassword = 'PreviewGuestPassword2026!';
const video = readFileSync(join(import.meta.dirname, 'fixtures/preview.webm'));

function pdfFixture(): Buffer {
	const stream = 'BT /F1 18 Tf 50 150 Td (CubyCloud preview PDF) Tj ET';
	const objects = [
		'<< /Type /Catalog /Pages 2 0 R >>',
		'<< /Type /Pages /Kids [3 0 R 4 0 R] /Count 2 >>',
		'<< /Type /Page /Parent 2 0 R /MediaBox [0 0 300 220] /Resources << /Font << /F1 5 0 R >> >> /Contents 6 0 R >>',
		'<< /Type /Page /Parent 2 0 R /MediaBox [0 0 300 220] /Resources << /Font << /F1 5 0 R >> >> /Contents 6 0 R >>',
		'<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>',
		`<< /Length ${stream.length} >>\nstream\n${stream}\nendstream`
	];
	let body = '%PDF-1.4\n';
	const offsets = [0];
	for (const [index, object] of objects.entries()) {
		offsets.push(Buffer.byteLength(body));
		body += `${index + 1} 0 obj\n${object}\nendobj\n`;
	}
	const xref = Buffer.byteLength(body);
	body += `xref\n0 ${objects.length + 1}\n0000000000 65535 f \n`;
	for (const offset of offsets.slice(1)) body += `${String(offset).padStart(10, '0')} 00000 n \n`;
	return Buffer.from(`${body}trailer\n<< /Size ${objects.length + 1} /Root 1 0 R >>\nstartxref\n${xref}\n%%EOF\n`);
}

const files = [
	{ name: 'preview-image.png', mime: 'image/png', buffer: Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jON8AAAAASUVORK5CYII=', 'base64') },
	{ name: 'preview-video.webm', mime: 'video/webm', buffer: video },
	{ name: 'preview-source.json', mime: 'application/json', buffer: Buffer.from('{"safe":"<script>alert(1)</script>"}') },
	{ name: 'preview-document.pdf', mime: 'application/pdf', buffer: pdfFixture() }
];

type Node = { id: string; name: string };

async function setup(page: Page) {
	const status = await page.request.get('/api/v1/setup/status');
	expect(status.ok()).toBeTruthy();
	if (!(await status.json() as { is_setup_completed: boolean }).is_setup_completed) {
		const scan = await page.request.get('/api/v1/setup/scan-nodes');
		const { nodes } = await scan.json() as { nodes: { path: string; capacity_bytes: number }[] };
		const response = await page.request.post('/api/v1/setup/complete', { data: {
			admin: { ...admin, email: 'preview-e2e@example.test', recovery_key: 'ABCDE-FGHKL-MNPQR-STUVW-XYZ23-45678' },
			storage_nodes: nodes.map((node) => ({ ...node, enabled: true })),
			placement_strategy: 'FILL_FIRST', cloud: { enabled: false }
		} });
		expect(response.ok()).toBeTruthy();
	}
	await login(page, admin.username, admin.password);
}

async function login(page: Page, username: string, password: string) {
	await page.context().clearCookies();
	await page.goto('/login');
	await page.locator('#login-identifier').fill(username);
	await page.locator('#login-password').fill(password);
	await page.getByRole('button', { name: /Masuk|Sign in/i }).click();
	await page.waitForURL('**/drive/home');
}

async function createFolder(page: Page, base: string, name: string, parent?: string): Promise<Node> {
	const response = await page.request.post(`/api/v1/drive/${base}/folders`, {
		data: { name, parent_id: parent ?? null }
	});
	expect(response.ok()).toBeTruthy();
	return response.json() as Promise<Node>;
}

async function upload(page: Page, base: string, folderId: string) {
	for (const file of files) {
		const qs = new URLSearchParams({ filename: file.name, parent_id: folderId });
		const response = await page.request.post(`/api/v1/drive/${base}/upload?${qs}`, {
			multipart: { file: { name: file.name, mimeType: file.mime, buffer: file.buffer } }
		});
		expect(response.ok(), `${base}: ${file.name}`).toBeTruthy();
	}
}

async function previewAll(page: Page) {
	for (const file of files) {
		await page.getByText(file.name, { exact: true }).first().dblclick();
		const dialog = page.getByRole('dialog');
		await expect(dialog.getByText(file.name, { exact: true }).first()).toBeVisible();
		await expect(page).toHaveURL(/preview=/);
		if (file.mime === 'image/png') {
			await expect(dialog.getByTestId('image-preview')).toHaveJSProperty('naturalWidth', 1);
		} else if (file.mime === 'video/webm') {
			await expect(dialog.getByTestId('video-preview')).toBeVisible();
			await expect.poll(() => dialog.locator('video').evaluate((element: HTMLVideoElement) => element.duration)).toBeGreaterThan(0);
			await dialog.locator('media-play-button').first().click();
			await expect.poll(() => dialog.locator('video').evaluate((element: HTMLVideoElement) => element.currentTime)).toBeGreaterThan(0);
		} else if (file.mime === 'application/pdf') {
			await expect(dialog.getByText('Halaman 1 dari 2')).toBeVisible();
			await expect(dialog.locator('canvas')).toBeVisible();
			await expect.poll(() => dialog.locator('canvas').evaluate((canvas: HTMLCanvasElement) => canvas.width > 0 && canvas.getContext('2d')!.getImageData(100, 100, 1, 1).data[3] > 0)).toBe(true);
			await dialog.getByRole('button', { name: 'Berikutnya' }).click();
			await expect(dialog.getByText('Halaman 2 dari 2')).toBeVisible();
		} else {
			await expect(dialog.getByTestId('text-preview')).toContainText('<script>alert(1)</script>');
			await expect(dialog.getByTestId('text-preview').locator('.token')).not.toHaveCount(0);
			expect(await dialog.getByTestId('text-preview').locator('script').count()).toBe(0);
		}
		await dialog.getByRole('button', { name: 'Tutup' }).first().click();
		await expect(dialog).not.toBeVisible();
	}
}

test('My Storage, Organization, Shared and public nested folder render image, Vidstack video, Prism text and PDF', async ({ page, browser }) => {
	test.setTimeout(240_000);
	await setup(page);
	const user = { username: `preview-reader-${Date.now()}`, email: `preview-reader-${Date.now()}@example.test`, password: 'PreviewReader2026!' };
	const created = await page.request.post('/api/v1/admin/users', { data: { ...user, quota_mode: 'DEFAULT' } });
	expect(created.status()).toBe(201);
	const reader = await created.json() as Node;

	const root = await createFolder(page, 'my-storage', `Preview Shared ${Date.now()}`);
	const nested = await createFolder(page, 'my-storage', 'Inside Preview Folder', root.id);
	await upload(page, 'my-storage', nested.id);
	const largeText = await page.request.post(`/api/v1/drive/my-storage/upload?filename=preview-large.txt&parent_id=${nested.id}`, {
		multipart: { file: { name: 'preview-large.txt', mimeType: 'text/plain', buffer: Buffer.alloc(1024 * 1024 + 1, 65) } }
	});
	expect(largeText.ok()).toBeTruthy();
	const unsupported = await page.request.post(`/api/v1/drive/my-storage/upload?filename=preview-unsupported.zip&parent_id=${nested.id}`, {
		multipart: { file: { name: 'preview-unsupported.zip', mimeType: 'application/zip', buffer: Buffer.from('ZIP') } }
	});
	expect(unsupported.ok()).toBeTruthy();
	const org = await createFolder(page, 'organization', `Preview Organization ${Date.now()}`);
	await upload(page, 'organization', org.id);

	await page.goto(`/drive/my-storage?folder=${nested.id}`);
	await expect(page.getByText(files[0].name, { exact: true })).toBeVisible();
	await previewAll(page);
	await page.getByText('preview-large.txt', { exact: true }).first().dblclick();
	await expect(page.getByRole('dialog').getByText(/maksimum 1 MB/)).toBeVisible();
	await expect(page.getByRole('dialog').getByRole('button', { name: 'Unduh' })).toBeVisible();
	await page.getByRole('dialog').getByRole('button', { name: 'Tutup' }).first().click();
	await page.getByText('preview-unsupported.zip', { exact: true }).first().dblclick();
	await expect(page.getByRole('dialog').getByText(/Tidak ada pratinjau/)).toBeVisible();
	await expect(page.getByRole('dialog').getByRole('button', { name: 'Unduh' })).toBeVisible();
	await page.getByRole('dialog').getByRole('button', { name: 'Tutup' }).first().click();
	await page.goto(`/drive/organization?folder=${org.id}`);
	await expect(page.getByText(files[0].name, { exact: true })).toBeVisible();
	await previewAll(page);

	// 1. Verify "Bagikan" action is hidden in Organization item menu
	await page.getByRole('button', { name: 'Aksi berkas' }).first().click();
	await expect(page.getByRole('menuitem', { name: 'Bagikan' })).toHaveCount(0);
	await page.keyboard.press('Escape');
	const grant = await page.request.put(`/api/v1/drive/items/${root.id}/shares/users`, { data: { grants: [{ user_id: reader.id, permission: 'viewer' }] } });
	expect(grant.ok()).toBeTruthy();
	const publicLink = await page.request.post(`/api/v1/drive/items/${root.id}/shares/links`, { data: { permission: 'viewer', password: guestPassword } });
	expect(publicLink.ok()).toBeTruthy();
	const { link } = await publicLink.json() as { link: { token: string } };
	const listing = await page.request.get(`/api/v1/drive/my-storage?parent_id=${nested.id}`);
	expect(listing.ok()).toBeTruthy();
	const { items: nestedItems } = await listing.json() as { items: Node[] };
	const image = nestedItems.find((item) => item.name === files[0].name);
	expect(image).toBeDefined();
	const singleFileLink = await page.request.post(`/api/v1/drive/items/${image!.id}/shares/links`, { data: { permission: 'viewer' } });
	expect(singleFileLink.ok()).toBeTruthy();
	const { link: singleLink } = await singleFileLink.json() as { link: { token: string } };

	// 2. Verify direct-link preview for subfolder item works without folder param
	await page.goto(`/drive/my-storage?preview=${image!.id}`);
	const directDialog = page.getByRole('dialog');
	await expect(directDialog.getByText(image!.name, { exact: true })).toBeVisible();
	await directDialog.getByRole('button', { name: 'Tutup' }).first().click();

	// 3. Star the item, navigate to /drive/starred, and verify preview opens via double-click
	const starRes = await page.request.post(`/api/v1/drive/my-storage/items/${image!.id}/star`);
	expect(starRes.ok()).toBeTruthy();
	await page.goto('/drive/starred');
	await expect(page.getByText(image!.name, { exact: true }).first()).toBeVisible();
	await page.getByText(image!.name, { exact: true }).first().dblclick();
	const starredDialog = page.getByRole('dialog');
	await expect(starredDialog.getByText(image!.name, { exact: true })).toBeVisible();
	await starredDialog.getByRole('button', { name: 'Tutup' }).first().click();
	await login(page, user.username, user.password);
	await page.goto('/drive/shared');
	await page.getByText(root.name, { exact: true }).first().dblclick();
	await page.getByText(nested.name, { exact: true }).first().dblclick();
	await expect(page.getByText(files[0].name, { exact: true })).toBeVisible();
	await previewAll(page);

	const guest = await browser.newContext();
	try {
		const guestPage = await guest.newPage();
		await guestPage.goto(`/s/${link.token}`);
		await expect(guestPage.getByText('Tautan dilindungi').first()).toBeVisible();
		await expect(guestPage.getByText(files[0].name, { exact: true })).toHaveCount(0);
		const forgedHeaders = {
			'X-Share-Unlock': '1',
			Cookie: `share_unlock_${link.token}=1`
		};
		const forgedShare = await guestPage.request.get(`/api/v1/s/${link.token}?unlock_token=1`, { headers: forgedHeaders });
		expect(forgedShare.ok()).toBeTruthy();
		expect((await forgedShare.json() as { unlocked: boolean }).unlocked).toBe(false);
		const forgedDownload = await guestPage.request.get(`/api/v1/s/${link.token}/download?file_id=${image!.id}&preview=1&unlock_token=1`, { headers: forgedHeaders });
		expect(forgedDownload.ok()).toBe(false);
		await guestPage.locator('#share-gate-password').fill(guestPassword);
		await guestPage.getByRole('button', { name: 'Buka tautan' }).click();
		await guestPage.getByText(nested.name, { exact: true }).first().dblclick();
		await expect(guestPage.getByText(files[0].name, { exact: true })).toBeVisible();
		await previewAll(guestPage);
		await guestPage.goto(`/s/${singleLink.token}`);
		await expect(guestPage.getByRole('button', { name: 'Pratinjau' })).toBeVisible();
		await guestPage.getByRole('button', { name: 'Pratinjau' }).click();
		await expect(guestPage.getByRole('dialog').getByTestId('image-preview')).toHaveJSProperty('naturalWidth', 1);
	} finally {
		await guest.close();
	}
});
