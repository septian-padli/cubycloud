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
	await page.getByLabel('Username atau Email').fill(username);
	await page.getByRole('textbox', { name: 'Password' }).fill(password);
	await page.getByRole('button', { name: /Masuk|Sign in/ }).click();
	await page.waitForURL('**/drive/home');
}

test('Admin settings allows selecting placement strategy with confirmation dialog on save', async ({ page }) => {
	await ensureAdminLoggedIn(page);

	// 1. Navigate to /admin/settings
	await page.goto('/admin/settings');
	await expect(page.getByRole('heading', { name: 'Admin Settings' })).toBeVisible();
	await expect(page.getByText('Pengaturan sistem')).toBeVisible();

	const strategySelect = page.getByTestId('select-placement-strategy');
	await expect(strategySelect).toBeVisible();

	const saveBtn = page.getByTestId('btn-save-settings');
	await expect(saveBtn).toBeDisabled();

	// 2. Open dropdown and verify the 3 options exist
	await strategySelect.click();
	await expect(page.getByRole('option', { name: 'Fill First' })).toBeVisible();
	await expect(page.getByRole('option', { name: 'Most Free Space' })).toBeVisible();
	await expect(page.getByRole('option', { name: 'Round Robin' })).toBeVisible();

	// 3. Select 'Most Free Space'
	await page.getByRole('option', { name: 'Most Free Space' }).click();
	await expect(strategySelect).toContainText('Most Free Space');
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
	await expect(page.getByTestId('select-placement-strategy')).toContainText('Fill First');

	// 6. Select 'Most Free Space' again and confirm save
	await page.getByTestId('select-placement-strategy').click();
	await page.getByRole('option', { name: 'Most Free Space' }).click();
	await expect(saveBtn).toBeEnabled();

	await saveBtn.click();
	await expect(confirmModal).toBeVisible();
	await confirmModal.getByRole('button', { name: 'Simpan' }).click();
	await expect(confirmModal).not.toBeVisible();

	await expect(page.getByText('Pengaturan berhasil disimpan')).toBeVisible();
	await expect(saveBtn).toBeDisabled();

	// 7. Reload page and verify persisted value
	await page.reload();
	await expect(page.getByTestId('select-placement-strategy')).toContainText('Most Free Space');

	// 8. Select 'Round Robin' and confirm save
	await page.getByTestId('select-placement-strategy').click();
	await page.getByRole('option', { name: 'Round Robin' }).click();
	await expect(saveBtn).toBeEnabled();

	await saveBtn.click();
	await expect(confirmModal).toBeVisible();
	await confirmModal.getByRole('button', { name: 'Simpan' }).click();
	await expect(confirmModal).not.toBeVisible();

	await expect(page.getByText('Pengaturan berhasil disimpan')).toBeVisible();
	await expect(saveBtn).toBeDisabled();

	// 9. Reload page and verify 'Round Robin' persisted
	await page.reload();
	await expect(page.getByTestId('select-placement-strategy')).toContainText('Round Robin');
});
