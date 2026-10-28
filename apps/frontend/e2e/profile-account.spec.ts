import { expect, test, type Locator, type Page } from '@playwright/test';

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
			const { nodes } = (await scan.json()) as {
				nodes: { path: string; capacity_bytes: number }[];
			};
			const setup = await page.request.post('/api/v1/setup/complete', {
				data: {
					admin: {
						username: adminUsername,
						email: 'user-e2e@example.test',
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

	await page.context().clearCookies();
	await page.goto('/login');
	await page.locator('#login-identifier').fill(adminUsername);
	await page.locator('#login-password').fill(adminPassword);
	await page.getByRole('button', { name: /Masuk|Sign in/i }).click();
	await page.waitForURL('**/drive/home');
}

async function expectTabColors(tab: Locator, selected: boolean) {
	const expected = await tab.evaluate(() => {
		const tokenColor = (token: string) => {
			const probe = document.createElement('span');
			probe.style.color = `var(${token})`;
			document.body.append(probe);
			const color = getComputedStyle(probe).color;
			probe.remove();
			return color;
		};
		return { primary: tokenColor('--primary'), muted: tokenColor('--text-muted') };
	});

	await expect
		.poll(async () =>
			tab.evaluate((element) => {
				const style = getComputedStyle(element);
				return {
					color: style.color,
					borderBottomColor: style.borderBottomColor,
					borderBottomWidth: style.borderBottomWidth
				};
			})
		)
		.toEqual({
			color: selected ? expected.primary : expected.muted,
			borderBottomColor: selected ? expected.primary : 'rgba(0, 0, 0, 0)',
			borderBottomWidth: '2px'
		});
}

test('Account profile edits persist, other-user profiles are read-only, and password changes revoke sessions', async ({
	page
}) => {
	await ensureAdminLoggedIn(page);
	const stamp = Date.now();
	const username = `profile-user-${stamp}`;
	const originalPassword = 'ProfileUser2026!';
	const newPassword = 'ProfileUserChanged2026!';
	const originalEmail = `${username}@example.test`;
	const newEmail = `updated-${username}@example.test`;
	let createdUser: { id: string } | undefined;

	try {
		const create = await page.request.post('/api/v1/admin/users', {
			data: {
				username,
				email: originalEmail,
				password: originalPassword,
				role: 'regular',
				quota_mode: 'CUSTOM',
				quota_gb: 2.5
			}
		});
		expect(create.status()).toBe(201);
		const created = (await create.json()) as { id: string };
		createdUser = { id: created.id };

		const adminMeResponse = await page.request.get('/api/v1/auth/me');
		expect(adminMeResponse.ok()).toBeTruthy();
		const adminMe = (await adminMeResponse.json()) as { id: string };

		await page.goto(`/drive/profile/${created.id}`);
		await expect(page.getByRole('heading', { name: username, exact: true })).toBeVisible();
		const effectiveQuotaSection = page.locator('div').filter({ hasText: /^Kuota efektif/ });
		await expect(effectiveQuotaSection).toBeVisible();
		await expect(effectiveQuotaSection.getByText(/2[.,]5 GiB/)).toBeVisible();
		await expect(page.getByText(/^(User|Pengguna)$/i)).toBeVisible();
		await expect(page.getByRole('button', { name: 'Edit Profil' })).toBeDisabled();

		await page.context().clearCookies();
		await page.goto('/login');
		await page.locator('#login-identifier').fill(username);
		await page.locator('#login-password').fill(originalPassword);
		await page.getByRole('button', { name: /Masuk|Sign in/i }).click();
		await page.waitForURL('**/drive/home');
		await page.goto(`/drive/profile/${created.id}`);
		await expect(page.getByRole('button', { name: 'Edit Profil' })).toBeEnabled();

		await page.getByRole('button', { name: 'Edit Profil' }).click();
		const profileDialog = page.getByRole('dialog', { name: 'Pengaturan Akun' });
		await expect(profileDialog).toBeVisible();
		await profileDialog.getByLabel('Nama tampilan').fill('Profile Editor');
		const profileTabs = profileDialog.getByRole('tablist');
		const profileTab = profileTabs.getByRole('tab', { name: 'Profil' });
		const passwordTab = profileTabs.getByRole('tab', { name: 'Kata Sandi' });
		await expect(profileTabs).toBeVisible();
		await expect(profileTab).toHaveAttribute('aria-selected', 'true');
		await expect(passwordTab).toHaveAttribute('aria-selected', 'false');
		await expectTabColors(profileTab, true);
		await expectTabColors(passwordTab, false);
		await expect(profileDialog.getByLabel('Nama tampilan')).toBeVisible();
		await expect(profileDialog.getByLabel('Kata sandi saat ini')).not.toBeVisible();
		await passwordTab.click();
		await expect(passwordTab).toHaveAttribute('aria-selected', 'true');
		await expect(profileTab).toHaveAttribute('aria-selected', 'false');
		await expectTabColors(passwordTab, true);
		await expectTabColors(profileTab, false);
		await expect(profileDialog.getByLabel('Kata sandi saat ini')).toBeVisible();
		await profileTab.click();
		await expect(profileTab).toHaveAttribute('aria-selected', 'true');
		await expect(profileDialog.getByLabel('Nama tampilan')).toBeVisible();
		await profileDialog.getByLabel('Email').fill(newEmail);
		await profileDialog.getByLabel('Kata sandi saat ini (diperlukan)').fill('WrongPassword2026!');
		await profileDialog.getByRole('button', { name: 'Simpan' }).click();
		await expect(profileDialog).toBeVisible();
		await profileDialog.getByLabel('Kata sandi saat ini (diperlukan)').fill(originalPassword);
		await profileDialog.getByRole('button', { name: 'Simpan' }).click();
		await expect(profileDialog).not.toBeVisible();
		await expect(page.getByRole('heading', { name: 'Profile Editor' })).toBeVisible();
		await expect(page.getByText(newEmail, { exact: true }).last()).toBeVisible();

		const ownProfile = await page.request.get(`/api/v1/auth/profile/${created.id}`);
		expect(ownProfile.ok()).toBeTruthy();
		const updatedProfile = (await ownProfile.json()) as { display_name: string; email: string };
		expect(updatedProfile).toMatchObject({ display_name: 'Profile Editor', email: newEmail });
		const forbiddenProfile = await page.request.get(`/api/v1/auth/profile/${adminMe.id}`);
		expect(forbiddenProfile.status()).toBe(403);

		await page.reload();
		await expect(page.getByRole('heading', { name: 'Profile Editor' })).toBeVisible();
		await expect(page.getByText(newEmail, { exact: true }).last()).toBeVisible();

		await page.getByRole('button', { name: 'Edit Profil' }).click();

		const changePasswordDialog = page.getByRole('dialog', { name: 'Pengaturan Akun' });
		const changePasswordTabs = changePasswordDialog.getByRole('tablist');
		const changeProfileTab = changePasswordTabs.getByRole('tab', { name: 'Profil' });
		const changePasswordTab = changePasswordTabs.getByRole('tab', { name: 'Kata Sandi' });
		await expect(changePasswordTab).toHaveAttribute('aria-selected', 'false');
		await changePasswordTab.click();
		await expect(changePasswordTab).toHaveAttribute('aria-selected', 'true');
		await expect(changeProfileTab).toHaveAttribute('aria-selected', 'false');
		await expectTabColors(changePasswordTab, true);
		await expectTabColors(changeProfileTab, false);
		await expect(changePasswordDialog.getByLabel('Kata sandi saat ini')).toBeVisible();
		await page.getByLabel('Kata sandi saat ini').fill(originalPassword);
		await page.getByLabel('Kata sandi baru', { exact: true }).fill(newPassword);
		await page.getByLabel('Konfirmasi kata sandi baru', { exact: true }).fill(newPassword);
		await page.getByRole('button', { name: 'Ubah Kata Sandi' }).click();
		await page.waitForURL('**/login');

		const staleSession = await page.request.get('/api/v1/auth/me');
		expect(staleSession.status()).toBe(401);
		const oldPasswordLogin = await page.request.post('/api/v1/auth/login', {
			data: { identifier: username, password: originalPassword }
		});
		expect(oldPasswordLogin.status()).toBe(401);
		const newPasswordLogin = await page.request.post('/api/v1/auth/login', {
			data: { identifier: username, password: newPassword }
		});
		expect(newPasswordLogin.ok()).toBeTruthy();
		await page.goto(`/drive/profile/${created.id}`);
		await expect(page.getByRole('heading', { name: 'Profile Editor' })).toBeVisible();
	} finally {
		if (createdUser) {
			await page.context().clearCookies();
			const adminLogin = await page.request.post('/api/v1/auth/login', {
				data: { identifier: adminUsername, password: adminPassword }
			});
			if (adminLogin.ok()) {
				const usersResponse = await page.request.get('/api/v1/admin/users');
				expect(usersResponse.ok()).toBeTruthy();
				const { users } = (await usersResponse.json()) as {
					users: { id: string; email: string }[];
				};
				const targetUser = users.find((user) => user.id === createdUser.id);
				if (targetUser) {
					const removed = await page.request.delete(`/api/v1/admin/users/${createdUser.id}`, {
						data: { email: targetUser.email }
					});
					expect(removed.ok(), `${removed.status()}: ${await removed.text()}`).toBeTruthy();
				}
			}
		}
	}
});
