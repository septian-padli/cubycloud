import { expect, test, type Page } from '@playwright/test';

const superAdminUsername = 'storage-e2e-admin';
const superAdminPassword = 'StorageE2E-password-2026';
const recoveryKey = 'ABCDE-FGHKL-MNPQR-STUVW-XYZ23-45678';

const regularUsername = 'rbac-regular-user';
const regularEmail = 'rbac-regular@example.test';
const regularPassword = 'RegularPassword2026!';

async function ensureSetup(page: Page) {
	const status = await page.request.get('/api/v1/setup/status');
	if (status.ok()) {
		const json = (await status.json()) as { is_setup_completed: boolean };
		if (!json.is_setup_completed) {
			const scan = await page.request.get('/api/v1/setup/scan-nodes');
			expect(scan.ok()).toBeTruthy();
			const { nodes } = (await scan.json()) as { nodes: { path: string; capacity_bytes: number }[] };
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

test('Role-based access control enforces Super Admin routes and prevents Regular User access', async ({
	page
}) => {
	// 1. Helper: pastikan setup sudah complete (jika belum, setup dengan superadmin storage-e2e-admin)
	await ensureSetup(page);

	// 2. Login sebagai superadmin
	await loginUser(page, superAdminUsername, superAdminPassword);

	// Verifikasi dapat membuka /admin/settings dengan sukses (heading 'Admin Settings' visible, tidak ada pesan 403)
	await page.goto('/admin/settings');
	await expect(page.getByRole('heading', { name: /Pengaturan Admin|Admin Settings/i })).toBeVisible();
	await expect(page.getByTestId('admin-forbidden-page')).not.toBeVisible();
	await expect(page.getByText('Akses ditolak')).not.toBeVisible();

	// Verifikasi sidebar memiliki menu Super Admin
	const superAdminSidebar = page.locator('aside');
	await expect(superAdminSidebar.getByText('Super Admin', { exact: true })).toBeVisible();
	await expect(superAdminSidebar.getByRole('link', { name: /Pengaturan Admin|Admin Settings/i })).toBeVisible();
	await expect(superAdminSidebar.getByRole('link', { name: /Pengguna & Kuota|Users & Quotas/i })).toBeVisible();

	// Verifikasi header nav TIDAK memiliki tombol toggle role demo
	const superAdminHeader = page.locator('header');
	await expect(superAdminHeader.getByRole('button', { name: /Super Admin|Regular/i })).toHaveCount(0);
	await expect(superAdminHeader.getByText('Super Admin', { exact: true })).toHaveCount(0);
	await expect(superAdminHeader.getByText('Regular', { exact: true })).toHaveCount(0);

	// Buat user regular baru bernama rbac-regular-user via API /api/v1/admin/users. Jika user sudah ada (409 conflict), lanjutkan.
	const createUserRes = await page.request.post('/api/v1/admin/users', {
		data: {
			username: regularUsername,
			email: regularEmail,
			password: regularPassword,
			quota_mode: 'DEFAULT'
		}
	});
	expect([201, 409]).toContain(createUserRes.status());

	// 3. Logout / clear cookies, lalu login sebagai rbac-regular-user
	await loginUser(page, regularUsername, regularPassword);

	// Verifikasi header nav TIDAK memiliki tombol toggle role demo
	const regularHeader = page.locator('header');
	await expect(regularHeader.getByRole('button', { name: /Super Admin|Regular/i })).toHaveCount(0);
	await expect(regularHeader.getByText('Super Admin', { exact: true })).toHaveCount(0);
	await expect(regularHeader.getByText('Regular', { exact: true })).toHaveCount(0);

	// Verifikasi sidebar TIDAK menampilkan menu 'Super Admin' maupun link ke /admin/settings, /admin/users
	const regularSidebar = page.locator('aside');
	await expect(regularSidebar.getByText('Super Admin', { exact: true })).not.toBeVisible();
	await expect(regularSidebar.getByRole('link', { name: /Pengaturan Admin|Admin Settings/i })).toHaveCount(0);
	await expect(regularSidebar.getByRole('link', { name: /Pengguna & Kuota|Users & Quotas/i })).toHaveCount(0);
	await expect(regularSidebar.locator('a[href^="/admin/"]')).toHaveCount(0);

	// Buka langsung /admin/settings
	await page.goto('/admin/settings');
	const forbiddenSettings = page.getByTestId('admin-forbidden-page');
	await expect(forbiddenSettings).toBeVisible();
	await expect(forbiddenSettings).toContainText(/403|Akses ditolak/i);

	// Verifikasi form 'Admin Settings' atau 'Pengaturan sistem' TIDAK tampil
	await expect(page.getByRole('heading', { name: /Pengaturan Admin|Admin Settings/i })).not.toBeVisible();
	await expect(page.getByText('Pengaturan sistem')).not.toBeVisible();
	await expect(page.getByRole('heading', { name: 'Node penyimpanan' })).not.toBeVisible();

	// Buka langsung /admin/users
	await page.goto('/admin/users');
	const forbiddenUsers = page.getByTestId('admin-forbidden-page');
	await expect(forbiddenUsers).toBeVisible();
	await expect(forbiddenUsers).toContainText(/Akses ditolak/i);

	// Verifikasi tabel user management TIDAK tampil
	await expect(page.getByRole('heading', { name: /Pengguna & Kuota|Users & Quotas/i })).not.toBeVisible();
	await expect(page.getByRole('button', { name: 'Buat Pengguna' })).not.toBeVisible();
	await expect(page.locator('table')).not.toBeVisible();

	// Lakukan request langsung via page.request.get('/api/v1/admin/settings')
	const settingsApiRes = await page.request.get('/api/v1/admin/settings');
	expect(settingsApiRes.status()).toBe(403);

	// Lakukan request langsung via page.request.get('/api/v1/admin/users')
	const usersApiRes = await page.request.get('/api/v1/admin/users');
	expect(usersApiRes.status()).toBe(403);
});
