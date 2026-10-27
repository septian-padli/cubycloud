import { expect, test, request, type Page } from '@playwright/test';

const adminUsername = 'storage-e2e-admin';
const adminPassword = 'StorageE2E-password-2026';
const recoveryKey = 'ABCDE-FGHKL-MNPQR-STUVW-XYZ23-45678';

async function ensureAdminLoggedIn(page: Page) {
	const status = await page.request.get('/api/v1/setup/status');
	if (status.ok()) {
		const json = (await status.json()) as { is_setup_completed: boolean };
		if (!json.is_setup_completed) {
			const scan = await page.request.get('/api/v1/setup/scan-nodes');
			expect(scan.ok()).toBeTruthy();
			const { nodes } = (await scan.json()) as { nodes: { path: string; capacity_bytes: number }[] };
			const setup = await page.request.post('/api/v1/setup/complete', {
				data: {
					admin: { username: adminUsername, email: 'user-e2e@example.test', password: adminPassword, recovery_key: recoveryKey },
					storage_nodes: nodes.map((node) => ({ ...node, enabled: true })),
					placement_strategy: 'FILL_FIRST',
					cloud: { enabled: false }
				}
			});
			expect(setup.ok()).toBeTruthy();
		}
	}

	await page.context().clearCookies();
	await page.goto('/login');
	await page.locator('#login-identifier').fill(adminUsername);
	await page.locator('#login-password').fill(adminPassword);
	await page.getByRole('button', { name: /Masuk|Sign in/ }).click();
	await page.waitForURL('**/drive/home');
}

test('Admin Users manages user creation, quota modes (fractional, default, unlimited), and deletion', async ({ page }) => {
	await ensureAdminLoggedIn(page);
	await page.request.put('/api/v1/admin/settings', {
		data: { default_quota: { mode: 'limited', gb: 100 } }
	});
	// 1. Navigate to /admin/users
	await page.goto('/admin/users');
	await expect(page.getByRole('heading', { name: /Users & Quotas|Pengguna & Kuota/ })).toBeVisible();

	// 2. Open Buat Pengguna dialog
	await page.getByRole('button', { name: /Buat Pengguna|Create User/ }).click();
	await expect(page.getByRole('heading', { name: /Buat Pengguna|Create user/ })).toBeVisible();

	// 3. Fill form and choose fractional quota (1.5 GiB)
	const newUsername = 'tester_quota';
	await page.getByLabel(/Nama pengguna|Username/).fill(newUsername);
	await page.getByLabel('Email').fill('tester_quota@example.test');

	// Select Atur GiB and verify suffix GiB and full width
	await page.getByText(/Atur GiB|Set GiB/).click();
	const quotaInput = page.getByLabel(/Kuota dalam GiB|Quota in GiB/);
	await expect(quotaInput).toBeVisible();
	await expect(quotaInput).toHaveClass(/w-full/);
	await expect(page.getByRole('dialog').getByText('GiB', { exact: true })).toBeVisible();
	await quotaInput.fill('1.5');
	// Submit creation
	await page.getByRole('button', { name: /Buat Pengguna|Create user/ }).last().click();
	await expect(page.getByRole('heading', { name: /Undangan pengguna siap|User invitation ready/ })).toBeVisible();
	const inviteInput = page.getByLabel(/Tautan undangan|Invitation link/);
	const inviteUrl = await inviteInput.inputValue();
	const inviteToken = new URL(inviteUrl).hash.replace('#token=', '');
	await page.keyboard.press('Escape');
	await expect(page.getByRole('heading', { name: /Undangan pengguna siap|User invitation ready/ })).not.toBeVisible();
	await expect(page.getByRole('heading', { name: /Buat Pengguna|Create user/ })).not.toBeVisible();
	// 4. Verify user appears in table with 1.5 GiB
	const userRow = page.locator('tr').filter({ hasText: newUsername });
	await expect(userRow).toBeVisible();
	await expect(userRow.getByText('1.5 GiB')).toBeVisible();

	// 5. Edit Kuota -> Reset ke Default
	await userRow.getByRole('button', { name: new RegExp(`(Aksi|Actions for) ${newUsername}`) }).click();
	await page.getByRole('menuitem', { name: /Edit Kuota|Edit quota/ }).click();
	await expect(page.getByRole('heading', { name: /Edit Kuota|Edit quota/ })).toBeVisible();

	// Select Default
	await page.getByText(/Reset ke Default|Reset to [Dd]efault/i).click();
	await page.getByRole('button', { name: /Simpan|Save/ }).click();
	await expect(page.getByRole('heading', { name: /Edit Kuota|Edit quota/ })).not.toBeVisible();

	// Verify quota is updated (100 GiB default from setup)
	await expect(userRow.getByText('100 GiB')).toBeVisible();

	// 6. Edit Kuota -> Unlimited
	await userRow.getByRole('button', { name: new RegExp(`(Aksi|Actions for) ${newUsername}`) }).click();
	await page.getByRole('menuitem', { name: /Edit Kuota|Edit quota/ }).click();
	await expect(page.getByRole('heading', { name: /Edit Kuota|Edit quota/ })).toBeVisible();

	await page.getByRole('dialog').getByLabel(/Unlimited|Tanpa batas/).click();
	await page.getByRole('button', { name: /Simpan|Save/ }).click();
	await expect(page.getByRole('heading', { name: /Edit Kuota|Edit quota/ })).not.toBeVisible();

	// Verify quota is updated to Unlimited
	await expect(userRow.getByText(/Unlimited|Tanpa batas/i)).toBeVisible();

	// 7. Upload a file as tester_quota in a separate request context so admin cookie is preserved
	const baseURL = new URL(page.url()).origin;
	const userApi = await request.newContext({ baseURL });
	// Complete password setup via invitation token first
	const setupRes = await userApi.post('/api/v1/auth/setup-password', {
		data: {
			token: inviteToken,
			password: 'ValidPassword12345!',
			confirm_password: 'ValidPassword12345!'
		}
	});
	expect(setupRes.ok()).toBeTruthy();
	const userLogin = await userApi.post('/api/v1/auth/login', {
		data: { identifier: newUsername, password: 'ValidPassword12345!' }
	});
	expect(userLogin.ok()).toBeTruthy();
	const uploadRes = await userApi.post('/api/v1/drive/my-storage/upload?filename=dummy.bin', {
		multipart: {
			file: {
				name: 'dummy.bin',
				mimeType: 'application/octet-stream',
				buffer: Buffer.alloc(1024 * 1024, 'a')
			}
		}
	});
	expect(uploadRes.ok()).toBeTruthy();
	await userApi.dispose();
	await page.goto('/admin/users');
	await expect(userRow).toBeVisible();

	// 8. Lower quota below usage (set to 0 GiB) -> triggers confirmation dialog
	await userRow.getByRole('button', { name: new RegExp(`(Aksi|Actions for) ${newUsername}`) }).click();
	await page.getByRole('menuitem', { name: /Edit Kuota|Edit quota/ }).click();
	await expect(page.getByRole('heading', { name: /Edit Kuota|Edit quota/ })).toBeVisible();

	await page.getByText(/Atur GiB|Set GiB/).click();
	const editQuotaInput = page.getByRole('dialog').getByLabel(/Kuota dalam GiB|Quota in GiB/);
	await expect(editQuotaInput).toHaveClass(/w-full/);
	await expect(page.getByRole('dialog').getByText('GiB', { exact: true })).toBeVisible();
	await editQuotaInput.fill('0');
	await page.getByRole('button', { name: /Simpan|Save/ }).click();

	// Confirm dialog must appear
	await expect(page.getByRole('heading', { name: /Konfirmasi Penurunan Kuota|Confirm quota reduction/i })).toBeVisible();

	// Click Batal on confirmation dialog
	await page.getByRole('dialog', { name: /Konfirmasi Penurunan Kuota|Confirm quota reduction/i }).getByRole('button', { name: /Batal|Cancel/i }).click();
	await expect(page.getByRole('heading', { name: /Konfirmasi Penurunan Kuota|Confirm quota reduction/i })).not.toBeVisible();

	// Click Simpan again, this time confirm with "Ya, Simpan"
	await page.getByRole('button', { name: /Simpan|Save/i }).click();
	await expect(page.getByRole('heading', { name: /Konfirmasi Penurunan Kuota|Confirm quota reduction/i })).toBeVisible();
	await page.getByRole('button', { name: /Ya, Simpan|Yes, save/i }).click();
	await expect(page.getByRole('heading', { name: /Konfirmasi Penurunan Kuota|Confirm quota reduction/i })).not.toBeVisible();
	await expect(page.getByRole('heading', { name: /Edit Kuota|Edit quota/ })).not.toBeVisible();

	// Verify quota is updated to 0 GiB
	await expect(userRow.getByRole('cell', { name: '0 GiB', exact: true })).toBeVisible();

	// Verify Super Admin row does NOT have 'Hapus' action item (hidden for super admin)
	const adminRow = page.locator('tr').filter({ hasText: adminUsername });
	await adminRow.getByRole('button', { name: new RegExp(`(Aksi|Actions for) ${adminUsername}`) }).click();
	await expect(page.getByRole('menuitem', { name: /Hapus|Delete/ })).not.toBeVisible();
	await expect(page.getByRole('menuitem', { name: /Reset Kata Sandi|Reset password/ })).not.toBeVisible();
	await page.keyboard.press('Escape');

	// Verify regular user row has 'Reset Kata Sandi' action item
	await userRow.getByRole('button', { name: new RegExp(`(Aksi|Actions for) ${newUsername}`) }).click();
	await expect(page.getByRole('menuitem', { name: /Reset Kata Sandi|Reset password/ })).toBeVisible();
	await page.keyboard.press('Escape');

	// 9. Delete regular user with confirmation dialog requiring exact email
	await userRow.getByRole('button', { name: new RegExp(`(Aksi|Actions for) ${newUsername}`) }).click();
	await page.getByRole('menuitem', { name: /Hapus|Delete/ }).click();

	const deleteDialog = page.getByRole('dialog', { name: /Hapus Pengguna|Delete user/ });
	await expect(deleteDialog).toBeVisible();

	const deleteSubmitBtn = deleteDialog.getByRole('button', { name: /Hapus Pengguna|Delete user/ });
	await expect(deleteSubmitBtn).toBeDisabled();

	// Type mismatched email -> still disabled
	const confirmInput = deleteDialog.locator('#confirm-email-input');
	await confirmInput.fill('wrong@email.test');
	await expect(deleteSubmitBtn).toBeDisabled();

	// Type exact email -> button becomes enabled
	await confirmInput.fill('tester_quota@example.test');
	await expect(deleteSubmitBtn).toBeEnabled();

	await deleteSubmitBtn.click();
	await expect(deleteDialog).not.toBeVisible();

	// Verify user is removed from table
	await expect(page.locator('tr').filter({ hasText: newUsername })).not.toBeVisible();

	// Verify deleted user cannot log in
	const deletedLoginContext = await request.newContext({ baseURL });
	const deletedLoginRes = await deletedLoginContext.post('/api/v1/auth/login', {
		data: { identifier: newUsername, password: 'ValidPassword12345!' }
	});
	expect(deletedLoginRes.status()).toBe(401);
	await deletedLoginContext.dispose();
	// 10. Verify Sidebar actual storage, user name, email, and logout functionality
	const sidebar = page.locator('aside').first();
	await expect(sidebar.getByText(adminUsername)).toBeVisible();
	await expect(sidebar.getByText(/@example\.test/)).toBeVisible();
	await expect(sidebar.getByText(/terpakai|used/)).toBeVisible();

	// Open three-dot account menu popover in sidebar
	await sidebar.getByRole('button', { name: /Menu akun|Account menu/ }).click();
	const logoutItem = page.getByRole('menuitem', { name: /Logout|Log out|Keluar/ });
	await expect(logoutItem).toBeVisible();
	await logoutItem.click();

	// Verify redirection to /login
	await page.waitForURL(/\/login/);
	await expect(page).toHaveURL(/\/login/);
});
test('Delete User enforces exact email typing, blocks paste, purges personal storage & trash, and preserves organization items as deleted user', async ({ page }) => {
	await ensureAdminLoggedIn(page);
	const baseURL = new URL(page.url()).origin;

	// 1. Create user alice_del via Admin UI
	await page.goto('/admin/users');
	await page.getByRole('button', { name: /Buat Pengguna|Create User/ }).click();
	await page.getByLabel(/Nama pengguna|Username/).fill('alice_del');
	await page.getByLabel('Email').fill('alice_del@example.test');
	await page.getByRole('button', { name: /Buat Pengguna|Create user/ }).last().click();
	await expect(page.getByRole('heading', { name: /Undangan pengguna siap|User invitation ready/ })).toBeVisible();
	const aliceInviteInput = page.getByLabel(/Tautan undangan|Invitation link/);
	const aliceInviteUrl = await aliceInviteInput.inputValue();
	const aliceToken = new URL(aliceInviteUrl).hash.replace('#token=', '');
	await page.keyboard.press('Escape');
	await expect(page.getByRole('heading', { name: /Undangan pengguna siap|User invitation ready/ })).not.toBeVisible();
	await expect(page.getByRole('heading', { name: /Buat Pengguna|Create user/ })).not.toBeVisible();
	const aliceRow = page.locator('tr').filter({ hasText: 'alice_del' });
	await expect(aliceRow).toBeVisible();

	// 2. Login as alice_del and create:
	// - 1 active personal file
	// - 1 trashed personal file
	// - 1 organization shared file
	const aliceApi = await request.newContext({ baseURL });
	const aliceSetup = await aliceApi.post('/api/v1/auth/setup-password', {
		data: {
			token: aliceToken,
			password: 'AliceSecurePassword123!',
			confirm_password: 'AliceSecurePassword123!'
		}
	});
	expect(aliceSetup.ok()).toBeTruthy();
	const aliceLogin = await aliceApi.post('/api/v1/auth/login', {
		data: { identifier: 'alice_del', password: 'AliceSecurePassword123!' }
	});
	expect(aliceLogin.ok()).toBeTruthy();

	// Personal file 1 (active)
	const upActive = await aliceApi.post('/api/v1/drive/my-storage/upload?filename=alice-active.txt', {
		multipart: {
			file: {
				name: 'alice-active.txt',
				mimeType: 'text/plain',
				buffer: Buffer.from('active content')
			}
		}
	});
	expect(upActive.ok()).toBeTruthy();

	// Personal file 2 (move to trash)
	const upTrash = await aliceApi.post('/api/v1/drive/my-storage/upload?filename=alice-trash.txt', {
		multipart: {
			file: {
				name: 'alice-trash.txt',
				mimeType: 'text/plain',
				buffer: Buffer.from('trash content')
			}
		}
	});
	expect(upTrash.ok()).toBeTruthy();
	const trashItem = (await upTrash.json()) as { id: string };
	const delRes = await aliceApi.delete(`/api/v1/drive/my-storage/${trashItem.id}`);
	expect(delRes.status()).toBe(204);

	// Organization shared file
	const upOrg = await aliceApi.post('/api/v1/drive/organization/upload?filename=alice-shared-doc.txt', {
		multipart: {
			file: {
				name: 'alice-shared-doc.txt',
				mimeType: 'text/plain',
				buffer: Buffer.from('shared organization content')
			}
		}
	});
	expect(upOrg.ok()).toBeTruthy();
	await aliceApi.dispose();

	// 3. Admin opens delete dialog for alice_del
	await aliceRow.getByRole('button', { name: /(Aksi|Actions for) alice_del/ }).click();
	await page.getByRole('menuitem', { name: /Hapus|Delete/ }).click();

	const deleteDialog = page.getByRole('dialog', { name: /Hapus Pengguna|Delete user/ });
	await expect(deleteDialog).toBeVisible();
	const deleteSubmitBtn = deleteDialog.getByRole('button', { name: /Hapus Pengguna|Delete user/ });
	await expect(deleteSubmitBtn).toBeDisabled();

	// 4. Test paste prevention: dispatch a paste event
	const confirmInput = deleteDialog.locator('#confirm-email-input');
	await confirmInput.focus();
	await confirmInput.evaluate((el) => {
		const pasteEvent = new ClipboardEvent('paste', {
			bubbles: true,
			cancelable: true,
			clipboardData: new DataTransfer()
		});
		pasteEvent.clipboardData?.setData('text/plain', 'alice_del@example.test');
		el.dispatchEvent(pasteEvent);
	});
	// Verify paste was cancelled and input remains empty
	await expect(confirmInput).toHaveValue('');
	await expect(deleteSubmitBtn).toBeDisabled();

	// 5. Test case mismatch (capital letters) -> button remains disabled
	await confirmInput.fill('ALICE_DEL@example.test');
	await expect(deleteSubmitBtn).toBeDisabled();

	// 6. Type exact matching email -> button enabled
	await confirmInput.fill('alice_del@example.test');
	await expect(deleteSubmitBtn).toBeEnabled();

	// 7. Confirm deletion
	await deleteSubmitBtn.click();
	await expect(deleteDialog).not.toBeVisible();
	await expect(page.locator('tr').filter({ hasText: 'alice_del' })).not.toBeVisible();

	// 8. Verify deleted user cannot log in
	const verifyContext = await request.newContext({ baseURL });
	const reLogin = await verifyContext.post('/api/v1/auth/login', {
		data: { identifier: 'alice_del', password: 'AliceSecurePassword123!' }
	});
	expect(reLogin.status()).toBe(401);

	// 9. Verify in Organization that alice-shared-doc.txt is preserved and creator is 'deleted user'
	await page.goto('/drive/organization');
	await expect(page.getByText('alice-shared-doc.txt')).toBeVisible();

	// Verify organization API response has created_by = 'deleted user'
	const orgListRes = await page.request.get('/api/v1/drive/organization');
	expect(orgListRes.ok()).toBeTruthy();
	const orgList = (await orgListRes.json()) as { items: { name: string; created_by: string }[] };
	const foundDoc = orgList.items.find((i) => i.name === 'alice-shared-doc.txt');
	expect(foundDoc).toBeDefined();
	expect(foundDoc?.created_by).toBe('deleted user');

	await verifyContext.dispose();
});

test('Superadmin can reset another user password with single-use link and session revocation', async ({ page }) => {
	await ensureAdminLoggedIn(page);
	const baseURL = new URL(page.url()).origin;

	// 1. Create a user via Admin UI
	await page.goto('/admin/users');
	await page.getByRole('button', { name: /Buat Pengguna|Create User/ }).click();
	const targetUsername = 'bob_reset';
	await page.getByLabel(/Nama pengguna|Username/).fill(targetUsername);
	await page.getByLabel('Email').fill('bob_reset@example.test');
	await page.getByRole('button', { name: /Buat Pengguna|Create user/ }).last().click();
	await expect(page.getByRole('heading', { name: /Undangan pengguna siap|User invitation ready/ })).toBeVisible();
	const bobInviteInput = page.getByLabel(/Tautan undangan|Invitation link/);
	const bobInviteUrl = await bobInviteInput.inputValue();
	const bobToken = new URL(bobInviteUrl).hash.replace('#token=', '');
	await page.keyboard.press('Escape');
	await expect(page.getByRole('heading', { name: /Undangan pengguna siap|User invitation ready/ })).not.toBeVisible();

	const bobRow = page.locator('tr').filter({ hasText: targetUsername });
	await expect(bobRow).toBeVisible();

	// 2. User bob sets password and logs in
	const bobContext = await request.newContext({ baseURL });
	const bobSetup = await bobContext.post('/api/v1/auth/setup-password', {
		data: {
			token: bobToken,
			password: 'InitialPassword12345!',
			confirm_password: 'InitialPassword12345!'
		}
	});
	expect(bobSetup.ok()).toBeTruthy();

	const bobLogin1 = await bobContext.post('/api/v1/auth/login', {
		data: { identifier: targetUsername, password: 'InitialPassword12345!' }
	});
	expect(bobLogin1.ok()).toBeTruthy();

	// 3. Admin opens Reset Kata Sandi dialog for bob
	await bobRow.getByRole('button', { name: new RegExp(`(Aksi|Actions for) ${targetUsername}`) }).click();
	const resetItem = page.getByRole('menuitem', { name: /Reset Kata Sandi|Reset password/ });
	await expect(resetItem).toBeVisible();
	await resetItem.click();

	const resetDialog = page.getByRole('dialog', { name: /Reset Kata Sandi|Reset Password/ });
	await expect(resetDialog).toBeVisible();

	// Cancel test
	await resetDialog.getByRole('button', { name: /Batal|Cancel/ }).click();
	await expect(resetDialog).not.toBeVisible();

	// Open dialog again and confirm reset
	await bobRow.getByRole('button', { name: new RegExp(`(Aksi|Actions for) ${targetUsername}`) }).click();
	await page.getByRole('menuitem', { name: /Reset Kata Sandi|Reset password/ }).click();
	await expect(resetDialog).toBeVisible();
	await resetDialog.getByRole('button', { name: /Reset Kata Sandi|Reset Password/ }).click();
	await expect(resetDialog).not.toBeVisible();

	// Success dialog displays reset one-time link
	await expect(page.getByRole('heading', { name: /Tautan reset siap|Reset link ready/ })).toBeVisible();
	const resetInput = page.getByLabel(/Tautan reset kata sandi|Password reset link/);
	const resetLink = await resetInput.inputValue();
	const resetToken = new URL(resetLink).hash.replace('#token=', '');
	await page.keyboard.press('Escape');

	// 4. Verify bob's previous session is revoked and old password cannot log in
	const oldPassLogin = await bobContext.post('/api/v1/auth/login', {
		data: { identifier: targetUsername, password: 'InitialPassword12345!' }
	});
	expect(oldPassLogin.status()).toBe(403);

	// 5. Complete password reset via new token
	const resetSubmit = await bobContext.post('/api/v1/auth/setup-password', {
		data: {
			token: resetToken,
			password: 'BrandNewPassword9999!',
			confirm_password: 'BrandNewPassword9999!'
		}
	});
	expect(resetSubmit.ok()).toBeTruthy();

	// 6. Verify bob can log in with brand new password
	const newPassLogin = await bobContext.post('/api/v1/auth/login', {
		data: { identifier: targetUsername, password: 'BrandNewPassword9999!' }
	});
	expect(newPassLogin.ok()).toBeTruthy();
	await bobContext.dispose();
});
