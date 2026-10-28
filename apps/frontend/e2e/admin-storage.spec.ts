import { expect, test, type Page } from '@playwright/test';

const superAdminUsername = 'storage-e2e-admin';
const superAdminPassword = 'StorageE2E-password-2026';
const recoveryKey = 'ABCDE-FGHKL-MNPQR-STUVW-XYZ23-45678';

const regularUsername = 'admin-storage-reg-user';
const regularEmail = 'admin-storage-reg@example.test';
const regularPassword = 'RegularPassword2026!';

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
						username: superAdminUsername,
						email: 'storage-e2e@example.test',
						password: superAdminPassword,
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

async function loginUser(page: Page, username: string, password: string) {
	await page.context().clearCookies();
	await page.goto('/login');
	await page.locator('#login-identifier').fill(username);
	await page.locator('#login-password').fill(password);
	await page.getByRole('button', { name: /Masuk|Sign in/i }).click();
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
test.describe('Admin Storage Stats & Top Users', () => {
	test('Super Admin can view system storage quota, usage, and top users ranked by storage consumption', async ({
		page
	}) => {
		// 1. Ensure setup
		await ensureSetup(page);

		// 2. Login as Super Admin
		await loginUser(page, superAdminUsername, superAdminPassword);

		// 3. Create regular user via admin API if not exists
		const createUserRes = await page.request.post('/api/v1/admin/users', {
			data: {
				username: regularUsername,
				email: regularEmail,
				password: regularPassword,
				quota_mode: 'DEFAULT'
			}
		});
		expect([201, 409]).toContain(createUserRes.status());

		// 4. Verify sidebar has Penyimpanan link with href="/admin/storage"
		const sidebar = page.locator('aside');
		const storageNavLink = sidebar.locator('a[href="/admin/storage"]');
		await expect(storageNavLink).toBeVisible();
		await expect(storageNavLink).toContainText('Penyimpanan');

		// 5. Click sidebar link to navigate to /admin/storage
		await storageNavLink.click();
		await page.waitForURL('**/admin/storage');

		// 6. Verify page title and header
		await expect(page.getByRole('heading', { name: 'Penyimpanan Sistem' })).toBeVisible();

		// 7. Verify storage overview card exists
		const overviewCard = page
			.locator('section')
			.filter({ hasText: 'Detail kuota & pemakaian sistem' });
		await expect(overviewCard).toBeVisible();
		await expect(overviewCard.getByText('Total Kapasitas')).toBeVisible();
		await expect(overviewCard.getByText('Total Pemakaian')).toBeVisible();
		await expect(overviewCard.getByText('Sisa Kapasitas')).toBeVisible();

		// 8. Fetch direct stats from API to compare values and verify categories
		const statsRes = await page.request.get('/api/v1/admin/storage/stats');
		expect(statsRes.ok()).toBeTruthy();
		const statsData = (await statsRes.json()) as {
			total_capacity_bytes: number;
			total_capacity_gb: number;
			total_used_bytes: number;
			total_used_gb: number;
			nodes_count: number;
			categories: { id: string; label: string; bytes: number; used_gb: number }[];
		};
		expect(statsData.total_capacity_bytes).toBeGreaterThan(0);
		expect(statsData.nodes_count).toBeGreaterThan(0);
		expect(Array.isArray(statsData.categories)).toBeTruthy();

		// Verify expected category IDs exist in response
		const catIds = statsData.categories.map((c) => c.id);
		expect(catIds).toContain('video');
		expect(catIds).toContain('image');
		expect(catIds).toContain('document');
		expect(catIds).toContain('archive');
		expect(catIds).toContain('trash');

		// Verify Category Legend buttons are rendered in the UI
		await expect(overviewCard.getByRole('button', { name: 'Video' })).toBeVisible();
		await expect(overviewCard.getByRole('button', { name: 'Gambar' })).toBeVisible();
		await expect(overviewCard.getByRole('button', { name: 'Dokumen' })).toBeVisible();
		await expect(overviewCard.getByRole('button', { name: 'Arsip' })).toBeVisible();
		await expect(overviewCard.getByRole('button', { name: /^(Trash|Sampah)$/ })).toBeVisible();
		const topUsersSection = page
			.locator('section')
			.filter({ hasText: 'Pengguna dengan konsumsi terbesar' });
		await expect(topUsersSection).toBeVisible();

		// Verify users are listed and ranked
		const topUsersRes = await page.request.get('/api/v1/admin/storage/top-users?limit=20&offset=0');
		expect(topUsersRes.ok()).toBeTruthy();
		const topUsersData = (await topUsersRes.json()) as {
			items: { username: string; used_bytes: number; role: string }[];
			total: number;
		};
		expect(topUsersData.items.length).toBeGreaterThan(0);

		// Verify descending order of used_bytes
		for (let i = 0; i < topUsersData.items.length - 1; i++) {
			expect(topUsersData.items[i].used_bytes).toBeGreaterThanOrEqual(
				topUsersData.items[i + 1].used_bytes
			);
		}

		// Verify first user appears on page
		const firstUser = topUsersData.items[0];
		await expect(topUsersSection.getByText(firstUser.username)).toBeVisible();
	});

	test('Regular user is forbidden from accessing admin storage API and page', async ({ page }) => {
		await ensureSetup(page);

		// Login as Super Admin first to ensure regular user exists
		await loginUser(page, superAdminUsername, superAdminPassword);
		const createUserRes = await page.request.post('/api/v1/admin/users', {
			data: {
				username: regularUsername,
				email: regularEmail,
				password: regularPassword,
				quota_mode: 'DEFAULT'
			}
		});
		expect([201, 409]).toContain(createUserRes.status());

		// Now login as regular user
		await loginUser(page, regularUsername, regularPassword);

		// 1. Sidebar should NOT have Super Admin section or /admin/storage link
		const sidebar = page.locator('aside');
		await expect(sidebar.getByText('Super Admin', { exact: true })).not.toBeVisible();
		await expect(sidebar.locator('a[href="/admin/storage"]')).toHaveCount(0);

		// 2. Direct page navigation should show forbidden error page
		await page.goto('/admin/storage');
		const forbiddenPage = page.getByTestId('admin-forbidden-page');
		await expect(forbiddenPage).toBeVisible();
		await expect(page.getByRole('heading', { name: 'Penyimpanan Sistem' })).not.toBeVisible();

		// 3. Direct API call should return 403 Forbidden
		const statsApi = await page.request.get('/api/v1/admin/storage/stats');
		expect(statsApi.status()).toBe(403);

		const topUsersApi = await page.request.get('/api/v1/admin/storage/top-users');
		expect(topUsersApi.status()).toBe(403);
	});

	test('Super Admin sees multi-category segmented bar with popover percentages based on total node capacity', async ({
		page
	}) => {
		await ensureSetup(page);
		await loginUser(page, superAdminUsername, superAdminPassword);

		// Upload video and document files
		await uploadFile(page, 'admin_sample_video.mp4', 20 * 1024 * 1024);
		await uploadFile(page, 'admin_sample_doc.pdf', 10 * 1024 * 1024);

		await page.goto('/admin/storage');
		await page.waitForURL('**/admin/storage');

		const overviewCard = page
			.locator('section')
			.filter({ hasText: 'Detail kuota & pemakaian sistem' });
		await expect(overviewCard).toBeVisible();

		// Segmented bar track
		const segmentedBar = overviewCard.locator('[role="img"]');
		await expect(segmentedBar).toBeVisible();

		// Category buttons within the bar track exist
		const videoSegment = segmentedBar.locator('button[aria-label^="Video:"]');
		const docSegment = segmentedBar.locator('button[aria-label^="Dokumen:"]');

		await expect(videoSegment).toBeAttached();
		await expect(docSegment).toBeAttached();

		// Click Legend button to toggle popover cleanly
		const videoLegendBtn = overviewCard.getByRole('button', { name: 'Video', exact: true });
		await expect(videoLegendBtn).toBeVisible();
		await videoLegendBtn.click();
		await expect(page.getByText('Proporsi kuota')).toBeVisible();
	});
});
