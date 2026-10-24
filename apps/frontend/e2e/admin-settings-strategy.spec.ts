import { expect, test, type Page } from '@playwright/test';

const username = 'storage-e2e-admin';
const password = 'StorageE2E-password-2026';
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
					admin: { username, email: 'strategy-e2e@example.test', password, recovery_key: recoveryKey },
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
	await page.locator('#login-identifier').fill(username);
	await page.locator('#login-password').fill(password);
	await page.getByRole('button', { name: /Masuk|Sign in/i }).click();
	await page.waitForURL('**/drive/home');
}

test('Admin settings allows selecting placement strategy with confirmation dialog on save', async ({ page }) => {
	await ensureAdminLoggedIn(page);

	// 1. Navigate to /admin/settings
	await page.goto('/admin/settings');
	await expect(page.getByRole('heading', { name: /Pengaturan Admin|Admin Settings/i })).toBeVisible();
	await expect(page.getByText(/Pengaturan sistem|System settings/i)).toBeVisible();

	const strategySelect = page.getByTestId('select-placement-strategy');
	await expect(strategySelect).toBeVisible();

	const saveBtn = page.getByTestId('btn-save-settings');
	await expect(saveBtn).toBeDisabled();

	// 2. Open dropdown and verify the 3 options exist
	await strategySelect.click();
	await expect(page.getByRole('option', { name: /Isi disk pertama|Fill First/i })).toBeVisible();
	await expect(page.getByRole('option', { name: /Ruang kosong terbanyak|Sisa kapasitas terbanyak|Most Free Space/i })).toBeVisible();
	await expect(page.getByRole('option', { name: /Bergiliran|Round Robin/i })).toBeVisible();

	// 3. Select 'Most Free Space'
	await page.getByRole('option', { name: /Ruang kosong terbanyak|Sisa kapasitas terbanyak|Most Free Space/i }).click();
	await expect(strategySelect).toContainText(/Ruang kosong terbanyak|Sisa kapasitas terbanyak|Most Free Space/i);
	await expect(saveBtn).toBeEnabled();

	// 4. Click Save -> Confirmation dialog must appear
	await saveBtn.click();
	const confirmModal = page.getByTestId('confirm-dialog');
	await expect(confirmModal).toBeVisible();
	await expect(confirmModal.getByText('Simpan Perubahan Pengaturan?')).toBeVisible();

	// 5. Test Cancel button on dialog -> closes dialog without saving
	await confirmModal.getByRole('button', { name: 'Batal' }).click();
	await expect(confirmModal).not.toBeVisible();
	await expect(saveBtn).toBeEnabled();

	// Verify not yet persisted by reloading
	await page.reload();
	await expect(page.getByTestId('select-placement-strategy')).toContainText(/Isi disk pertama|Fill First/i);

	// 6. Select 'Most Free Space' again and confirm save
	await page.getByTestId('select-placement-strategy').click();
	await page.getByRole('option', { name: /Ruang kosong terbanyak|Sisa kapasitas terbanyak|Most Free Space/i }).click();
	await expect(saveBtn).toBeEnabled();

	await saveBtn.click();
	await expect(confirmModal).toBeVisible();
	await confirmModal.getByRole('button', { name: 'Simpan' }).click();
	await expect(confirmModal).not.toBeVisible();

	await expect(page.getByText('Pengaturan berhasil disimpan')).toBeVisible();
	await expect(saveBtn).toBeDisabled();

	// 7. Reload page and verify persisted value
	await page.reload();
	await expect(page.getByTestId('select-placement-strategy')).toContainText(/Ruang kosong terbanyak|Sisa kapasitas terbanyak|Most Free Space/i);

	// 8. Select 'Round Robin' and confirm save
	await page.getByTestId('select-placement-strategy').click();
	await page.getByRole('option', { name: /Bergiliran|Round Robin/i }).click();
	await expect(saveBtn).toBeEnabled();

	await saveBtn.click();
	await expect(confirmModal).toBeVisible();
	await confirmModal.getByRole('button', { name: 'Simpan' }).click();
	await expect(confirmModal).not.toBeVisible();

	await expect(page.getByText('Pengaturan berhasil disimpan')).toBeVisible();
	await expect(saveBtn).toBeDisabled();

	// 9. Reload page and verify 'Round Robin' persisted
	await page.reload();
	await expect(page.getByTestId('select-placement-strategy')).toContainText(/Bergiliran|Round Robin/i);
});
