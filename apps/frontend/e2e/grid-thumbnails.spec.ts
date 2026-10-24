import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { expect, test, type Page } from '@playwright/test';

const admin = { username: 'storage-e2e-admin', password: 'StorageE2E-password-2026' };
const video = readFileSync(join(import.meta.dirname, 'fixtures/preview.webm'));

function pdfFixture(): Buffer {
	const stream = 'BT /F1 18 Tf 50 150 Td (CubyCloud Grid Thumbnail PDF) Tj ET';
	const objects = [
		'<< /Type /Catalog /Pages 2 0 R >>',
		'<< /Type /Pages /Kids [3 0 R] /Count 1 >>',
		'<< /Type /Page /Parent 2 0 R /MediaBox [0 0 300 220] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>',
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

const testFiles = [
	{ name: 'thumb-image.png', mime: 'image/png', buffer: Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAIAAAACCAYAAABytg0kAAAAFElEQVR42mNk+M9QzwAEjDAGACUcA/5bH8t7AAAAAElFTkSuQmCC', 'base64') },
	{ name: 'thumb-video.webm', mime: 'video/webm', buffer: video },
	{ name: 'thumb-doc.pdf', mime: 'application/pdf', buffer: pdfFixture() },
	{ name: 'thumb-data.json', mime: 'application/json', buffer: Buffer.from('{"key":"value"}') }
];

async function setup(page: Page) {
	const status = await page.request.get('/api/v1/setup/status');
	expect(status.ok()).toBeTruthy();
	if (!(await status.json() as { is_setup_completed: boolean }).is_setup_completed) {
		const scan = await page.request.get('/api/v1/setup/scan-nodes');
		const { nodes } = await scan.json() as { nodes: { path: string; capacity_bytes: number }[] };
		const response = await page.request.post('/api/v1/setup/complete', {
			data: {
				admin: { ...admin, email: 'grid-thumb@example.test', recovery_key: 'ABCDE-FGHKL-MNPQR-STUVW-XYZ23-45678' },
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

test('Grid mode renders client-side thumbnail preview for image, video, and PDF, with double-click opening preview modal', async ({ page }) => {
	test.setTimeout(120_000);
	await setup(page);

	// Create test folder
	const folderRes = await page.request.post('/api/v1/drive/my-storage/folders', {
		data: { name: 'Thumbnails Test', parent_id: null }
	});
	expect(folderRes.ok()).toBeTruthy();
	const folder = (await folderRes.json()) as { id: string };

	// Upload files
	for (const file of testFiles) {
		const qs = new URLSearchParams({ filename: file.name, parent_id: folder.id });
		const uploadRes = await page.request.post(`/api/v1/drive/my-storage/upload?${qs}`, {
			multipart: { file: { name: file.name, mimeType: file.mime, buffer: file.buffer } }
		});
		expect(uploadRes.ok(), `Upload ${file.name}`).toBeTruthy();
	}

	// Navigate to folder in my-storage
	await page.goto(`/drive/my-storage?folder=${folder.id}`);
	await expect(page.getByText('Thumbnails Test')).toBeVisible();
	await expect(page.getByText('thumb-image.png', { exact: true })).toBeVisible();
	// Ensure grid mode; clicking an already-active toggle would switch back to list view.
	const gridToggleBtn = page.getByRole('button', { name: 'Tampilan Kisi' });
	if ((await gridToggleBtn.getAttribute('aria-pressed')) !== 'true') {
		await gridToggleBtn.click();
	}

	// 1. Verify image thumbnail
	const imageCard = page.locator('div[role="option"]').filter({ hasText: 'thumb-image.png' });
	await expect(imageCard).toBeVisible();
	const imageThumb = imageCard.getByTestId('grid-image-thumbnail');
	await expect(imageThumb).toBeVisible();
	await expect.poll(() => imageThumb.evaluate((img: HTMLImageElement) => img.naturalWidth)).toBeGreaterThan(0);

	// 2. Verify video thumbnail (static frame, paused, no inline playback)
	const videoCard = page.locator('div[role="option"]').filter({ hasText: 'thumb-video.webm' });
	await expect(videoCard).toBeVisible();
	const videoThumb = videoCard.getByTestId('grid-video-thumbnail');
	await expect(videoThumb).toBeVisible();
	// Video must remain paused
	const isPaused = await videoThumb.evaluate((v: HTMLVideoElement) => v.paused);
	expect(isPaused).toBe(true);

	// 3. Verify PDF thumbnail
	const pdfCard = page.locator('div[role="option"]').filter({ hasText: 'thumb-doc.pdf' });
	await expect(pdfCard).toBeVisible();
	const pdfThumb = pdfCard.getByTestId('grid-pdf-thumbnail');
	await expect(pdfThumb).toBeVisible();

	// 4. Verify non-previewable file (JSON) falls back to icon
	const jsonCard = page.locator('div[role="option"]').filter({ hasText: 'thumb-data.json' });
	await expect(jsonCard).toBeVisible();
	await expect(jsonCard.getByTestId('grid-fallback-icon')).toBeVisible();

	// 5. Test double click video card opens preview modal
	await videoCard.dblclick();
	const dialog = page.getByRole('dialog');
	await expect(dialog).toBeVisible();
	await expect(dialog.getByText('thumb-video.webm')).toBeVisible();
	await expect(dialog.getByTestId('video-preview')).toBeVisible();
	await dialog.getByRole('button', { name: 'Tutup' }).first().click();
	await expect(dialog).not.toBeVisible();

	// 6. Test double click image card opens preview modal
	await imageCard.dblclick();
	await expect(dialog).toBeVisible();
	await expect(dialog.getByText('thumb-image.png')).toBeVisible();
	await expect(dialog.getByTestId('image-preview')).toBeVisible();
	await dialog.getByRole('button', { name: 'Tutup' }).first().click();
	await expect(dialog).not.toBeVisible();

	// 7. Test double click PDF card opens preview modal
	await pdfCard.dblclick();
	await expect(dialog).toBeVisible();
	await expect(dialog.getByText('thumb-doc.pdf')).toBeVisible();
	await expect(dialog.locator('canvas')).toBeVisible();
	await dialog.getByRole('button', { name: 'Tutup' }).first().click();
	await expect(dialog).not.toBeVisible();
});
