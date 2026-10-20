import { expect, test, type Page } from '@playwright/test';

const superAdminUsername = 'storage-e2e-admin';
const superAdminPassword = 'StorageE2E-password-2026';
const recoveryKey = 'ABCDE-FGHKL-MNPQR-STUVW-XYZ23-45678';

const userAlice = {
	username: 'alice-org-user',
	email: 'alice-org@example.test',
	password: 'AliceOrgPassword2026!'
};

const userBob = {
	username: 'bob-org-user',
	email: 'bob-org@example.test',
	password: 'BobOrgPassword2026!'
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
	await page.getByLabel('Username atau Email').fill(username);
	await page.getByRole('textbox', { name: 'Password' }).fill(password);
	await page.getByRole('button', { name: 'Masuk' }).click();
	await page.waitForURL('**/drive/home');
}

test('Organization shared drive provides multi-user collaboration and isolates personal My Storage with no node badges', async ({
	page
}) => {
	// 1. Setup superadmin if needed
	await ensureSetup(page);
	await loginUser(page, superAdminUsername, superAdminPassword);

	// 2. Create Alice and Bob user accounts
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

	// 3. Login as Alice
	await loginUser(page, userAlice.username, userAlice.password);

	// 3a. Check absence of storage node labels in My Storage
	await page.goto('/drive/my-storage');
	await expect(page.getByRole('heading', { name: 'My Storage' })).toBeVisible();
	await expect(page.getByText('SSD-1')).toHaveCount(0);
	await expect(page.getByText('HDD-2')).toHaveCount(0);
	await expect(page.getByText('R2')).toHaveCount(0);

	// 3b. Check absence of storage node labels in Organization
	await page.goto('/drive/organization');
	await expect(page.getByRole('heading', { name: 'Organization' })).toBeVisible();
	await expect(page.getByText('SSD-1')).toHaveCount(0);
	await expect(page.getByText('HDD-2')).toHaveCount(0);
	await expect(page.getByText('R2')).toHaveCount(0);

	// 4. Alice creates folder "Proyek Bersama Tim" in Organization
	await page.getByRole('button', { name: 'Tambah' }).click();
	await page.getByRole('menuitem', { name: 'Folder Baru' }).click();
	await expect(page.getByRole('heading', { name: 'Folder Baru' })).toBeVisible();
	await page.getByLabel('Nama folder').fill('Proyek Bersama Tim');
	await page.getByRole('button', { name: 'Buat' }).click();
	await expect(page.getByText('Proyek Bersama Tim', { exact: true })).toBeVisible();

	// 5. Alice uploads a shared file "shared-spec.txt" into Organization
	const fileInput = page.locator('input[type="file"]').first();
	await fileInput.setInputFiles({
		name: 'shared-spec.txt',
		mimeType: 'text/plain',
		buffer: Buffer.from('Spesifikasi bersama tim CubyCloud dibuat oleh Alice.')
	});
	await expect(page.getByText('shared-spec.txt', { exact: true })).toBeVisible();

	// 6. Alice goes to My Storage to verify isolation
	await page.goto('/drive/my-storage');
	await expect(page.getByRole('heading', { name: 'My Storage' })).toBeVisible();
	// Organization items MUST NOT leak into personal My Storage
	await expect(page.getByText('Proyek Bersama Tim')).toHaveCount(0);
	await expect(page.getByText('shared-spec.txt')).toHaveCount(0);

	// Alice creates a private folder in My Storage
	await page.getByRole('button', { name: 'Tambah' }).click();
	await page.getByRole('menuitem', { name: 'Folder Baru' }).click();
	await expect(page.getByRole('heading', { name: 'Folder Baru' })).toBeVisible();
	await page.getByLabel('Nama folder').fill('Alice Private Vault');
	await page.getByRole('button', { name: 'Buat' }).click();
	await expect(page.getByText('Alice Private Vault', { exact: true })).toBeVisible();

	// 7. Logout Alice, Login as Bob
	await loginUser(page, userBob.username, userBob.password);

	// 7a. Bob goes to My Storage: Alice's private vault must NOT appear in Bob's My Storage
	await page.goto('/drive/my-storage');
	await expect(page.getByRole('heading', { name: 'My Storage' })).toBeVisible();
	await expect(page.getByText('Alice Private Vault')).toHaveCount(0);
	await expect(page.getByText('Proyek Bersama Tim')).toHaveCount(0);
	await expect(page.getByText('SSD-1')).toHaveCount(0);

	// Bob creates his own private folder in My Storage
	await page.getByRole('button', { name: 'Tambah' }).click();
	await page.getByRole('menuitem', { name: 'Folder Baru' }).click();
	await expect(page.getByRole('heading', { name: 'Folder Baru' })).toBeVisible();
	await page.getByLabel('Nama folder').fill('Bob Personal Folder');
	await page.getByRole('button', { name: 'Buat' }).click();
	await expect(page.getByText('Bob Personal Folder', { exact: true })).toBeVisible();

	// 7b. Bob goes to Organization: Bob MUST see the shared folder and file created by Alice!
	await page.goto('/drive/organization');
	await expect(page.getByRole('heading', { name: 'Organization' })).toBeVisible();
	await expect(page.getByText('Proyek Bersama Tim', { exact: true })).toBeVisible();
	await expect(page.getByText('shared-spec.txt', { exact: true })).toBeVisible();

	// 8. Bob renames "shared-spec.txt" to "shared-spec-revised.txt" in Organization
	const fileRow = page.getByRole('option').filter({ hasText: 'shared-spec.txt' }).first();
	await fileRow.getByRole('button', { name: 'Aksi berkas' }).click();
	await page.getByRole('menuitem', { name: 'Ganti nama' }).click();
	await expect(page.getByRole('heading', { name: 'Ganti nama' })).toBeVisible();
	await page.getByRole('textbox', { name: 'Nama' }).fill('shared-spec-revised.txt');
	await page.getByRole('button', { name: 'Simpan' }).click();
	await expect(page.getByText('shared-spec-revised.txt', { exact: true })).toBeVisible();

	// 9. Bob logs out and Alice logs back in to verify shared state in Organization
	await loginUser(page, userAlice.username, userAlice.password);
	await page.goto('/drive/organization');
	await expect(page.getByRole('heading', { name: 'Organization' })).toBeVisible();
	// Alice sees the file renamed by Bob
	await expect(page.getByText('shared-spec-revised.txt', { exact: true })).toBeVisible();

	// Alice checks My Storage: Bob's personal folder MUST NOT be in Alice's My Storage
	await page.goto('/drive/my-storage');
	await expect(page.getByRole('heading', { name: 'My Storage' })).toBeVisible();
	await expect(page.getByText('Alice Private Vault', { exact: true })).toBeVisible();
	await expect(page.getByText('Bob Personal Folder')).toHaveCount(0);
});
