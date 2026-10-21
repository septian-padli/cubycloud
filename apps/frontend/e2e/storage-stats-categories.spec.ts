import { expect, test, type Page } from '@playwright/test';

const adminUsername = 'storage-e2e-admin';
const adminPassword = 'StorageE2E-password-2026';
const recoveryKey = 'ABCDE-FGHKL-MNPQR-STUVW-XYZ23-45678';

const unlimitedUsername = 'cat-stats-unlimited';
const unlimitedPassword = 'UnlimitedPassword-2026';

const limitedUsername = 'cat-stats-limited';
const limitedPassword = 'LimitedPassword-2026';
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
				admin: {
					username: adminUsername,
					email: 'cat-admin@example.test',
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

async function loginUser(page: Page, username: string, password: string) {
	await page.context().clearCookies();
	await page.goto('/login');
	await page.getByLabel('Username atau Email').fill(username);
	await page.getByRole('textbox', { name: 'Password' }).fill(password);
	await page.getByRole('button', { name: 'Masuk' }).click();
	await page.waitForURL('**/drive/home');
}

async function uploadFile(
	page: Page,
	filename: string,
	sizeBytes: number
): Promise<{ id: string; name: string }> {
	const buffer = Buffer.alloc(sizeBytes, 'a');
	const response = await page.request.post(
		`/api/v1/drive/my-storage/upload?filename=${encodeURIComponent(filename)}`,
		{
			multipart: {
				file: {
					name: filename,
					mimeType: 'text/plain',
					buffer
				}
			}
		}
	);
	expect(response.ok()).toBeTruthy();
	return (await response.json()) as { id: string; name: string };
}

test.describe('Storage Stats Category Breakdown & Quota Bar', () => {
	test('calculates real categories, isolates trash from file types, and reflects unlimited vs limited quota', async ({
		page
	}) => {
		await finishSetup(page);

		// 1. Login as Admin to provision isolated test users
		await loginUser(page, adminUsername, adminPassword);
		const cleanRes = await page.request.get('/api/v1/drive/my-storage');
		if (cleanRes.ok()) {
			const { items } = (await cleanRes.json()) as { items: { id: string }[] };
			for (const item of items) {
				await page.request.delete(`/api/v1/drive/my-storage/${encodeURIComponent(item.id)}`);
			}
			await page.request.delete('/api/v1/drive/trash');
		}
		// Super Admin is always unlimited: even with a nonzero usage, its bar must fill the track.
		const adminVideo = await uploadFile(page, 'super-admin-unlimited-video.mp4', 4 * 1024 * 1024);
		await page.goto('/drive/storage-stats');
		await expect(page.locator('text=telah digunakan')).toBeVisible();
		await expect(page.locator('text=dari')).not.toBeVisible();
		const adminTrack = page.locator(
			'div[role="img"][aria-label="Pemakaian kuota penyimpanan My Storage"]'
		);
		const adminFill = adminTrack.locator('div.flex.h-full');
		await expect(adminFill).toBeVisible();
		const adminTrackBox = await adminTrack.boundingBox();
		const adminFillBox = await adminFill.boundingBox();
		expect(adminTrackBox).not.toBeNull();
		expect(adminFillBox).not.toBeNull();
		expect(adminFillBox!.width).toBeCloseTo(adminTrackBox!.width, 0);
		const adminVideoSegment = adminTrack.locator('button[aria-label^="Video:"]');
		await expect(adminVideoSegment).toBeVisible();
		await adminVideoSegment.hover();
		await expect(page.getByText('Proporsi pemakaian', { exact: true })).toBeVisible();
		await expect(page.getByText('100%', { exact: true })).toBeVisible();
		const trashAdminVideo = await page.request.delete(`/api/v1/drive/my-storage/${adminVideo.id}`);
		expect(trashAdminVideo.ok()).toBeTruthy();
		const purgeAdminVideo = await page.request.delete(`/api/v1/drive/trash/${adminVideo.id}`);
		expect(purgeAdminVideo.ok()).toBeTruthy();

		// Create user with UNLIMITED quota
		const createUnlimitedRes = await page.request.post('/api/v1/admin/users', {
			data: {
				username: unlimitedUsername,
				email: 'cat-unlimited@example.test',
				password: unlimitedPassword,
				role: 'regular',
				quota_mode: 'UNLIMITED'
			}
		});
		expect([201, 409]).toContain(createUnlimitedRes.status());

		// Create user with 1 GB limited quota
		const createLimitedRes = await page.request.post('/api/v1/admin/users', {
			data: {
				username: limitedUsername,
				email: 'cat-limited@example.test',
				password: limitedPassword,
				role: 'regular',
				quota_mode: 'CUSTOM',
				quota_gb: 1
			}
		});
		expect([201, 409]).toContain(createLimitedRes.status());

		// 2. Login as the unlimited user
		await loginUser(page, unlimitedUsername, unlimitedPassword);
		// 2. Upload test files for Admin:
		// - 1 Video file (mp4): 4 MB (4 * 1024 * 1024)
		// - 1 Image file (png): 2 MB (2 * 1024 * 1024)
		// - 1 Document file (pdf): 1 MB (1 * 1024 * 1024)
		// - 1 Document file to be trashed (pdf): 3 MB (3 * 1024 * 1024)
		const video = await uploadFile(page, 'sample-clip.mp4', 4 * 1024 * 1024);
		const image = await uploadFile(page, 'sample-photo.png', 2 * 1024 * 1024);
		const doc = await uploadFile(page, 'sample-doc.pdf', 1 * 1024 * 1024);
		const toTrash = await uploadFile(page, 'trash-manual.pdf', 3 * 1024 * 1024);

		// Move toTrash to Trash
		const delRes = await page.request.delete(`/api/v1/drive/my-storage/${toTrash.id}`);
		expect(delRes.ok()).toBeTruthy();

		// 3. Verify backend category usage endpoint directly
		const catRes = await page.request.get('/api/v1/drive/my-storage/category-usage');
		expect(catRes.ok()).toBeTruthy();
		const catData = (await catRes.json()) as {
			total_bytes: number;
			total_gb: number;
			categories: { id: string; label: string; bytes: number; used_gb: number }[];
		};

		// Total should be 4 + 2 + 1 + 3 = 10 MB (10,485,760 bytes)
		expect(catData.total_bytes).toBe(10 * 1024 * 1024);

		const catMap = Object.fromEntries(catData.categories.map((c) => [c.id, c.bytes]));
		expect(catMap['video']).toBe(4 * 1024 * 1024);
		expect(catMap['image']).toBe(2 * 1024 * 1024);
		expect(catMap['document']).toBe(1 * 1024 * 1024); // trash-manual.pdf must NOT be in document!
		expect(catMap['trash']).toBe(3 * 1024 * 1024); // isolated in trash!

		// 4. Open UI at /drive/storage-stats
		await page.goto('/drive/storage-stats');

		// Header summary: should say "10 MB telah digunakan" (unlimited quota mode)
		const summaryText = page.locator('text=telah digunakan');
		await expect(summaryText).toBeVisible();
		await expect(page.locator('text=dari')).not.toBeVisible();

		// Check the segmented bar
		const track = page.locator(
			'div[role="img"][aria-label="Pemakaian kuota penyimpanan My Storage"]'
		);
		await expect(track).toBeVisible();

		// Inner filled container for unlimited should be width: 100%
		const filledContainer = track.locator('div.flex.h-full');
		await expect(filledContainer).toBeVisible();
		const styleAttr = await filledContainer.getAttribute('style');
		expect(styleAttr).toContain('width: 100%');
		const trackBox = await track.boundingBox();
		const filledBox = await filledContainer.boundingBox();
		expect(trackBox).not.toBeNull();
		expect(filledBox).not.toBeNull();
		expect(filledBox!.width).toBeCloseTo(trackBox!.width, 0);

		// Check segments on the bar: Video, Gambar, Dokumen, Trash
		const videoSegment = track.locator('button[aria-label^="Video:"]');
		const imageSegment = track.locator('button[aria-label^="Gambar:"]');
		const docSegment = track.locator('button[aria-label^="Dokumen:"]');
		const trashSegment = track.locator('button[aria-label^="Trash:"]');

		await expect(videoSegment).toBeVisible();
		await expect(imageSegment).toBeVisible();
		await expect(docSegment).toBeVisible();
		await expect(trashSegment).toBeVisible();
		const videoBox = await videoSegment.boundingBox();
		const imageBox = await imageSegment.boundingBox();
		const docBox = await docSegment.boundingBox();
		const trashBox = await trashSegment.boundingBox();
		expect(videoBox).not.toBeNull();
		expect(imageBox).not.toBeNull();
		expect(docBox).not.toBeNull();
		expect(trashBox).not.toBeNull();
		expect(videoBox!.width / filledBox!.width).toBeCloseTo(0.4, 1);
		expect(imageBox!.width / filledBox!.width).toBeCloseTo(0.2, 1);
		expect(docBox!.width / filledBox!.width).toBeCloseTo(0.1, 1);
		expect(trashBox!.width / filledBox!.width).toBeCloseTo(0.3, 1);

		// Hover on Video segment to verify popover content
		await videoSegment.hover();
		await expect(page.locator('text=Video').first()).toBeVisible();
		await expect(page.locator('text=4 MB').first()).toBeVisible();
		await expect(page.locator('text=Proporsi pemakaian').first()).toBeVisible();
		await expect(page.getByText('40%', { exact: true })).toBeVisible();

		// Hover on Trash segment to verify popover content
		await trashSegment.hover();
		await expect(page.locator('text=Trash').first()).toBeVisible();
		await expect(page.locator('text=3 MB').first()).toBeVisible();
		await expect(page.locator('text=Proporsi pemakaian').first()).toBeVisible();
		await expect(page.getByText('30%', { exact: true })).toBeVisible();

		// 5. Test Limited Quota Mode
		// Login as limited user (has 1 GB quota)
		await loginUser(page, limitedUsername, limitedPassword);

		// 100 MB video + 50 MB image + 25 MB document + 25 MB trash = 200 MB of 1 GB.
		await uploadFile(page, 'regular-presentation.mp4', 100 * 1024 * 1024);
		await uploadFile(page, 'regular-photo.png', 50 * 1024 * 1024);
		await uploadFile(page, 'regular-report.pdf', 25 * 1024 * 1024);
		const regularTrash = await uploadFile(page, 'regular-trash.pdf', 25 * 1024 * 1024);
		const deleteRegularTrash = await page.request.delete(
			`/api/v1/drive/my-storage/${regularTrash.id}`
		);
		expect(deleteRegularTrash.ok()).toBeTruthy();

		const limitedUsageRes = await page.request.get('/api/v1/drive/my-storage/category-usage');
		expect(limitedUsageRes.ok()).toBeTruthy();
		const limitedUsage = (await limitedUsageRes.json()) as {
			total_bytes: number;
			categories: { id: string; bytes: number }[];
		};
		expect(limitedUsage.total_bytes).toBe(200 * 1024 * 1024);
		const limitedCategoryBytes = Object.fromEntries(
			limitedUsage.categories.map((category) => [category.id, category.bytes])
		);
		expect(limitedCategoryBytes['video']).toBe(100 * 1024 * 1024);
		expect(limitedCategoryBytes['image']).toBe(50 * 1024 * 1024);
		expect(limitedCategoryBytes['document']).toBe(25 * 1024 * 1024);
		expect(limitedCategoryBytes['trash']).toBe(25 * 1024 * 1024);

		await page.goto('/drive/storage-stats');

		// 200 MB / 1 GB = 19.53125% of the track.
		await expect(page.locator('text=dari 1 GB telah digunakan')).toBeVisible();
		const regTrack = page.locator(
			'div[role="img"][aria-label="Pemakaian kuota penyimpanan My Storage"]'
		);
		const regFilled = regTrack.locator('div.flex.h-full');
		await expect(regFilled).toBeVisible();
		const regStyle = await regFilled.getAttribute('style');
		expect(regStyle).toMatch(/width:\s*19\.\d+%/);
		const regTrackBox = await regTrack.boundingBox();
		const regFilledBox = await regFilled.boundingBox();
		expect(regTrackBox).not.toBeNull();
		expect(regFilledBox).not.toBeNull();
		expect(regFilledBox!.width / regTrackBox!.width).toBeCloseTo(200 / 1024, 3);

		const limitedSegments = [
			{ name: 'Video', quotaShare: 100 / 1024, barShare: 0.5 },
			{ name: 'Gambar', quotaShare: 50 / 1024, barShare: 0.25 },
			{ name: 'Dokumen', quotaShare: 25 / 1024, barShare: 0.125 },
			{ name: 'Trash', quotaShare: 25 / 1024, barShare: 0.125 }
		];
		for (const segment of limitedSegments) {
			const button = regTrack.locator(`button[aria-label^="${segment.name}:"]`);
			await expect(button).toBeVisible();
			const box = await button.boundingBox();
			expect(box).not.toBeNull();
			expect(box!.width / regFilledBox!.width).toBeCloseTo(segment.barShare, 1);
			await button.hover();
			await expect(page.locator('text=Proporsi kuota').first()).toBeVisible();
			const expectedPct = (Math.round(segment.quotaShare * 1000) / 10).toString().replace('.', ',');
			const popover = page
				.locator('[data-slot="popover-content"]')
				.filter({ hasText: segment.name });
			await expect(popover.getByText(`${expectedPct}%`, { exact: true })).toBeVisible();
		}
	});
});
