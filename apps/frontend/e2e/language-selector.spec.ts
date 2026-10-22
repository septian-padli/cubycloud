import { expect, test, type Page } from '@playwright/test';

test.use({ locale: 'id-ID' });

const adminUsername = 'storage-e2e-admin';
const adminPassword = 'StorageE2E-password-2026';
const recoveryKey = 'ABCDE-FGHKL-MNPQR-STUVW-XYZ23-45678';
const regularUsername = 'language-e2e-user';
const regularEmail = 'language-e2e-user@example.test';
const regularPassword = 'LanguageE2E-password-2026!';

async function ensureSetup(page: Page) {
	const status = await page.request.get('/api/v1/setup/status');
	expect(status.ok()).toBeTruthy();
	const json = (await status.json()) as { is_setup_completed: boolean };
	if (json.is_setup_completed) return;

	const scan = await page.request.get('/api/v1/setup/scan-nodes');
	expect(scan.ok()).toBeTruthy();
	const { nodes } = (await scan.json()) as { nodes: { path: string; capacity_bytes: number }[] };
	const setup = await page.request.post('/api/v1/setup/complete', {
		data: {
			admin: {
				username: adminUsername,
				email: 'storage-e2e@example.test',
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

async function submitLogin(
	page: Page,
	username: string,
	password: string,
	buttonLabel: string
) {
	await page.locator('#login-identifier').fill(username);
	await page.locator('#login-password').fill(password);
	await page.getByRole('button', { name: buttonLabel, exact: true }).click();
	await page.waitForURL('**/drive/home');
}

async function loginUser(page: Page, username: string, password: string) {
	await page.context().clearCookies();
	await page.goto('/login');
	await submitLogin(page, username, password, 'Masuk');
}

async function chooseLanguage(page: Page, currentLabel: string, language: 'en' | 'id') {
	await page.getByRole('button', { name: currentLabel, exact: true }).click();
	await page.getByRole('option', { name: language, exact: true }).click();
	await expect(page.locator('html')).toHaveAttribute('lang', language);
}

test('language selection defaults from browser language and persists per account', async ({ page }) => {
	await ensureSetup(page);
	await page.goto('/login');
	await expect(page.locator('html')).toHaveAttribute('lang', 'id');
	await chooseLanguage(page, 'Bahasa', 'en');
	await submitLogin(page, adminUsername, adminPassword, 'Sign in');
	await expect(page.locator('html')).toHaveAttribute('lang', 'en');
	await expect(page.locator('aside').getByRole('link', { name: 'Home', exact: true })).toBeVisible();
	const createUser = await page.request.post('/api/v1/admin/users', {
		data: {
			username: regularUsername,
			email: regularEmail,
			password: regularPassword,
			quota_mode: 'DEFAULT'
		}
	});
	expect([201, 409]).toContain(createUser.status());

	await page.getByRole('button', { name: 'Account menu' }).click();
	await page.getByRole('menuitem', { name: 'Log out' }).click();
	await page.waitForURL('**/login');
	await expect(page.locator('html')).toHaveAttribute('lang', 'id');

	await loginUser(page, regularUsername, regularPassword);
	await expect(page.locator('html')).toHaveAttribute('lang', 'id');
	await expect(page.locator('aside').getByRole('link', { name: 'Beranda', exact: true })).toBeVisible();
	await chooseLanguage(page, 'Bahasa', 'en');
	await expect(page.locator('aside').getByRole('link', { name: 'Home', exact: true })).toBeVisible();
	await chooseLanguage(page, 'Language', 'id');
	await expect(page.locator('aside').getByRole('link', { name: 'Beranda', exact: true })).toBeVisible();

	const savedPreferences = await page.evaluate(() =>
		Object.entries(localStorage)
			.filter(([key]) => key.startsWith('cubycloud-language:'))
			.map(([, value]) => value)
			.sort()
	);
	expect(savedPreferences).toEqual(['en', 'id']);

	await loginUser(page, adminUsername, adminPassword);
	await expect(page.locator('html')).toHaveAttribute('lang', 'en');
	await expect(page.locator('aside').getByRole('link', { name: 'Home', exact: true })).toBeVisible();
});

test('login errors follow the selected language', async ({ page }) => {
	await ensureSetup(page);
	await page.goto('/login');
	await page.locator('#login-identifier').fill('missing-user');
	await page.locator('#login-password').fill('invalid-password');
	await page.getByRole('button', { name: 'Masuk', exact: true }).click();
	await expect(page.locator('form').getByText('Username atau password salah.')).toBeVisible();

	await chooseLanguage(page, 'Bahasa', 'en');
	await page.locator('#login-identifier').fill('missing-user');
	await page.locator('#login-password').fill('invalid-password');
	await page.getByRole('button', { name: 'Sign in', exact: true }).click();
	await expect(page.locator('form').getByText('Invalid username or password.')).toBeVisible();
});

test('documented authenticated routes follow the selected language', async ({ page }) => {
	await ensureSetup(page);
	await page.goto('/login');
	await submitLogin(page, adminUsername, adminPassword, 'Masuk');

	const meResponse = await page.request.get('/api/v1/auth/me');
	expect(meResponse.ok()).toBeTruthy();
	const me = (await meResponse.json()) as { id: string };
	const routes = [
		{ path: '/drive/home', en: 'Home', id: 'Beranda' },
		{ path: '/drive/search', en: 'File Search', id: 'Pencarian Berkas' },
		{ path: '/drive/my-storage', en: 'My Storage', id: 'My Storage' },
		{ path: '/drive/organization', en: 'Organization', id: 'Organisasi' },
		{ path: '/drive/shared', en: 'Shared with me', id: 'Dibagikan pada saya' },
		{ path: '/drive/starred', en: 'Starred', id: 'Berbintang' },
		{ path: '/drive/trash', en: 'Trash', id: 'Sampah' },
		{ path: '/drive/storage-stats', en: 'Storage', id: 'Penyimpanan' },
		{ path: `/drive/profile/${me.id}`, en: 'Account information', id: 'Informasi akun' },
		{ path: '/admin/storage', en: 'System Storage', id: 'Penyimpanan Sistem' },
		{ path: '/admin/settings', en: 'Admin Settings', id: 'Pengaturan Admin' },
		{ path: '/admin/users', en: 'Users & Quotas', id: 'Pengguna & Kuota' }
	];

	await chooseLanguage(page, 'Bahasa', 'en');
	for (const route of routes) {
		await page.goto(route.path);
		await expect(page.locator('html')).toHaveAttribute('lang', 'en');
		await expect(page.getByRole('heading', { name: route.en, exact: true })).toBeVisible();
	}

	await chooseLanguage(page, 'Language', 'id');
	for (const route of routes) {
		await page.goto(route.path);
		await expect(page.locator('html')).toHaveAttribute('lang', 'id');
		await expect(page.getByRole('heading', { name: route.id, exact: true })).toBeVisible();
	}
});

test.describe('browser language fallback', () => {
	test.use({ locale: 'fr-FR' });

	test('non-Indonesian browser languages default to English', async ({ page }) => {
		await ensureSetup(page);
		await page.goto('/login');
		await expect(page.locator('html')).toHaveAttribute('lang', 'en');
		await expect(page.getByRole('button', { name: 'Language' })).toHaveText('en');
	});
});
