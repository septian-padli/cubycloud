import { expect, test, type Page } from '@playwright/test';

const superAdminUsername = 'share-e2e-admin';
const superAdminPassword = 'ShareE2E-password-2026';
const recoveryKey = 'ABCDE-FGHKL-MNPQR-STUVW-XYZ23-45678';

const userAlice = {
	username: 'alice-share-user',
	email: 'alice-share@example.test',
	password: 'AliceSharePassword2026!'
};

const userBob = {
	username: 'bob-share-user',
	email: 'bob-share@example.test',
	password: 'BobSharePassword2026!'
};

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
						email: 'share-e2e@example.test',
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
	await page.getByLabel('Username atau Email').fill(username);
	await page.getByRole('textbox', { name: 'Password' }).fill(password);
	await page.getByRole('button', { name: 'Masuk' }).click();
	await page.waitForURL('**/drive/home');
}

test('File and folder sharing end-to-end: direct share to registered user and public links with password and revocation', async ({
	page,
	browser
}) => {
	// 1. Pastikan setup awal selesai dan login superadmin
	await ensureSetup(page);
	await loginUser(page, superAdminUsername, superAdminPassword);

	// 2. Buat user Alice dan Bob via API admin
	for (const user of [userAlice, userBob]) {
		const res = await page.request.post('/api/v1/admin/users', {
			data: {
				username: user.username,
				email: user.email,
				password: user.password,
				quota_mode: 'DEFAULT'
			}
		});
		expect([201, 409]).toContain(res.status());
	}

	// 3. Login sebagai Alice dan buat folder baru di My Storage
	await loginUser(page, userAlice.username, userAlice.password);
	await page.goto('/drive/my-storage');
	await page.waitForLoadState('networkidle');

	const folderName = `Collab-${Date.now()}`;
	await page.getByRole('button', { name: 'Tambah' }).click();
	await page.getByRole('menuitem', { name: 'Folder Baru' }).click();
	await page.getByLabel('Nama folder').fill(folderName);
	await page.getByRole('button', { name: 'Buat' }).click();

	const folderItem = page.getByRole('option').filter({ hasText: folderName });
	await expect(folderItem).toBeVisible({ timeout: 10000 });

	// 4. Buka Share Sheet untuk folder tersebut
	await folderItem.getByRole('button', { name: 'Aksi berkas' }).click();
	await page.getByRole('menuitem', { name: 'Bagikan' }).click();

	// Sheet Bagikan terbuka
	const sheet = page.getByRole('dialog');
	await expect(sheet.getByRole('heading', { name: 'Bagikan' })).toBeVisible();

	// 5. Bagikan ke user Bob dengan izin Editor
	const userSearchInput = sheet.getByLabel('Cari pengguna CubyCloud');
	await userSearchInput.fill(userBob.username);
	const suggestion = sheet.getByRole('button', { name: new RegExp(userBob.username, 'i') }).first();
	await expect(suggestion).toBeVisible({ timeout: 5000 });
	await suggestion.click();

	// Ubah izin Bob menjadi Ubah (Editor) lewat dropdown
	await sheet.getByRole('button', { name: new RegExp(`Pilih hak akses ${userBob.username}`, 'i') }).click();
	await page.getByRole('option', { name: 'Ubah', exact: true }).click();

	// Klik Simpan
	await sheet.getByRole('button', { name: 'Simpan' }).click();
	await expect(sheet).not.toBeVisible();

	// 6. Buka kembali sheet Bagikan untuk membuat Tautan Berbagi
	await folderItem.getByRole('button', { name: 'Aksi berkas' }).click();
	await page.getByRole('menuitem', { name: 'Bagikan' }).click();
	await expect(sheet.getByRole('heading', { name: 'Bagikan' })).toBeVisible();

	// Pindah ke tab Tautan
	await sheet.getByRole('button', { name: 'Tautan' }).click();

	// Aktifkan proteksi kata sandi
	const passwordSwitch = sheet.getByLabel('Aktifkan kata sandi');
	await passwordSwitch.click();
	const sharePasswordInput = sheet.locator('#share-password');
	await expect(sharePasswordInput).toBeVisible();
	await sharePasswordInput.fill('kunci-rahasia-2026');

	// Buat tautan
	await sheet.getByRole('button', { name: 'Buat tautan' }).click();

	// Tunggu URL tautan muncul
	const shareUrlInput = sheet.locator('#share-url');
	await expect(shareUrlInput).toBeVisible({ timeout: 8000 });
	const shareUrlValue = await shareUrlInput.inputValue();
	expect(shareUrlValue).toContain('/s/');
	const shareToken = shareUrlValue.split('/s/')[1]?.split('?')[0];
	expect(shareToken).toBeTruthy();

	// Verifikasi daftar tautan aktif menampilkan tautan tersebut
	await expect(sheet.getByText('Tautan Aktif')).toBeVisible();
	await expect(sheet.getByText('Viewer').first()).toBeVisible();

	// 7. Pengujian Akses Guest Publik (dengan Password Protection)
	const guestContext = await browser.newContext();
	const guestPage = await guestContext.newPage();
	await guestPage.goto(`/s/${shareToken}`);

	// Halaman Password Challenge harus muncul
	await expect(guestPage.getByText('Tautan dilindungi').first()).toBeVisible();
	const pwdInput = guestPage.locator('#share-gate-password');
	await pwdInput.fill('password-salah');
	await guestPage.getByRole('button', { name: 'Buka tautan' }).click();
	// Error muncul
	await expect(guestPage.getByText(/tidak sesuai|salah/i)).toBeVisible();

	// Masukkan password yang benar
	await pwdInput.fill('kunci-rahasia-2026');
	await guestPage.getByRole('button', { name: 'Buka tautan' }).click();

	// Folder terbuka dan menampilkan nama folder
	await expect(guestPage.getByText(folderName).first()).toBeVisible({ timeout: 8000 });
	await guestContext.close();

	// 8. Pengujian User Bob: Buka halaman "Dibagikan pada saya" (`/drive/shared`)
	await loginUser(page, userBob.username, userBob.password);
	await page.goto('/drive/shared');
	await page.waitForLoadState('networkidle');

	// Folder harus muncul di daftar Dibagikan pada saya
	await expect(page.getByText(folderName).first()).toBeVisible({ timeout: 10000 });
	await expect(page.getByText('Editor').first()).toBeVisible();

	// Buka folder bersama via double click
	await page.getByText(folderName).first().dblclick();

	// Breadcrumb atau tombol Folder baru muncul
	const newSubfolderBtn = page.getByRole('button', { name: /Folder baru/i });
	await expect(newSubfolderBtn).toBeVisible({ timeout: 8000 });

	// Bob membuat subfolder di dalam folder bersama (karena izin Editor)
	await newSubfolderBtn.click();
	const subInput = page.getByLabel('Nama folder').or(page.getByPlaceholder('Nama folder'));
	await subInput.fill('SubBob');
	await page.getByRole('button', { name: 'Buat' }).click();
	await expect(page.getByText('SubBob').first()).toBeVisible({ timeout: 10000 });

	// 9. Alice mencabut / menghapus tautan berbagi
	await loginUser(page, userAlice.username, userAlice.password);
	await page.goto('/drive/my-storage');
	await page.waitForLoadState('networkidle');

	const aliceFolderItem = page.getByRole('option').filter({ hasText: folderName });
	await aliceFolderItem.getByRole('button', { name: 'Aksi berkas' }).click();
	await page.getByRole('menuitem', { name: 'Bagikan' }).click();
	await expect(sheet.getByRole('heading', { name: 'Bagikan' })).toBeVisible();
	await sheet.getByRole('button', { name: 'Tautan' }).click();

	// Klik tombol hapus tautan
	const deleteLinkBtn = sheet.getByRole('button', { name: 'Cabut tautan' }).first();
	await expect(deleteLinkBtn).toBeVisible({ timeout: 15000 });
	await deleteLinkBtn.click();
	await expect(page.getByText('Tautan berhasil dicabut')).toBeVisible({ timeout: 10000 });

	// 10. Verifikasi tautan yang dicabut sekarang tidak bisa dibuka (404 / tidak ditemukan)
	const checkContext = await browser.newContext();
	const checkPage = await checkContext.newPage();
	await checkPage.goto(`/s/${shareToken}`);
	await expect(checkPage.getByText(/tautan tidak ditemukan/i).or(checkPage.getByText(/404/i))).toBeVisible({ timeout: 5000 });
	await checkContext.close();
});
