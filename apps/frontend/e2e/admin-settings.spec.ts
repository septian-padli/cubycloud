import { expect, test, type Locator, type Page } from '@playwright/test';

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
			const { nodes } = (await scan.json()) as {
				nodes: { path: string; capacity_bytes: number }[];
			};
			const setup = await page.request.post('/api/v1/setup/complete', {
				data: {
					admin: {
						username,
						email: 'storage-e2e@example.test',
						password,
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
	await page.locator('#login-identifier').fill(username);
	await page.locator('#login-password').fill(password);
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

test('Admin Settings manages settings, validation, unsaved changes warnings, and backup download', async ({
	page
}) => {
	await ensureAdminLoggedIn(page);
	// 1. Navigate to /admin/settings
	await page.goto('/admin/settings');
	await expect(
		page.getByRole('heading', { name: /Pengaturan Admin|Admin Settings/i })
	).toBeVisible();
	await expect(page.getByRole('heading', { name: 'Node penyimpanan' })).toBeVisible();
	await expect(page.getByText('Pengaturan sistem')).toBeVisible();

	// Storage nodes section check
	await expect(page.getByTestId('btn-add-storage-node')).toBeVisible();

	// Initially save button should be disabled because form is not dirty
	const saveBtn = page.getByTestId('btn-save-settings');
	await expect(saveBtn).toBeDisabled();

	// 2. Test Unlimited Switch and Validation
	const unlimitedSwitch = page.getByTestId('switch-unlimited-quota');
	const quotaInput = page.getByTestId('input-default-quota');
	await expect(quotaInput).toBeVisible();

	// Toggle unlimited on: quota input hidden
	await unlimitedSwitch.click();
	await expect(quotaInput).not.toBeVisible();

	// Toggle unlimited off: quota input visible again
	await unlimitedSwitch.click();
	await expect(quotaInput).toBeVisible();
	// Set invalid quota < 1
	await quotaInput.fill('0');
	await expect(page.getByTestId('error-quota')).toBeVisible();
	await expect(saveBtn).toBeDisabled();

	// Set valid quota
	await quotaInput.fill('35');
	await expect(page.getByTestId('error-quota')).not.toBeVisible();

	// Set invalid worker concurrency
	const concurrencyInput = page.getByTestId('input-worker-concurrency');
	await concurrencyInput.fill('0');
	await expect(page.getByTestId('error-concurrency')).toBeVisible();
	await expect(saveBtn).toBeDisabled();

	// Set valid worker concurrency
	await concurrencyInput.fill('3');
	await expect(page.getByTestId('error-concurrency')).not.toBeVisible();

	// Select audit retention
	await page.getByTestId('select-audit-retention').click();
	await page.getByRole('option', { name: '30 hari' }).click();

	// Form is dirty and valid, so Save button should be enabled
	await expect(saveBtn).toBeEnabled();

	// 3. Test Unsaved Changes Warning on Backup Download
	await page.getByTestId('btn-download-backup').click();
	const backupConfirmModal = page.getByTestId('confirm-dialog');
	await expect(backupConfirmModal).toBeVisible();
	await expect(backupConfirmModal.getByText('Perubahan Belum Disimpan')).toBeVisible();
	await backupConfirmModal.getByRole('button', { name: 'Batal' }).click();
	await expect(backupConfirmModal).not.toBeVisible();

	// 4. Test Global Save Settings
	await saveBtn.click();
	const saveConfirmModal = page.getByTestId('confirm-dialog');
	await expect(saveConfirmModal).toBeVisible();
	await saveConfirmModal.getByRole('button', { name: 'Simpan' }).click();
	await expect(page.getByText('Pengaturan berhasil disimpan')).toBeVisible();
	await expect(saveBtn).toBeDisabled();
	// Reload to verify persisted settings from backend
	await page.reload();
	await expect(page.getByTestId('input-default-quota')).toHaveValue('35');
	await expect(page.getByTestId('input-worker-concurrency')).toHaveValue('3');
	await expect(page.getByTestId('select-audit-retention')).toContainText('30 hari');
	await expect(saveBtn).toBeDisabled();

	// 5. Test Unsaved Changes Warning on Navigation
	await concurrencyInput.fill('4');
	await expect(saveBtn).toBeEnabled();

	// Attempt navigation to My Storage
	await page.getByRole('link', { name: 'My Storage' }).click();
	const leaveConfirmModal = page.getByTestId('confirm-dialog');
	await expect(leaveConfirmModal).toBeVisible();
	await expect(leaveConfirmModal.getByText('Tinggalkan Halaman?')).toBeVisible();

	// Cancel leave
	await leaveConfirmModal.getByRole('button', { name: 'Tetap di Sini' }).click();
	await expect(leaveConfirmModal).not.toBeVisible();
	expect(page.url()).toContain('/admin/settings');

	// Confirm leave
	await page.getByRole('link', { name: 'My Storage' }).click();
	await expect(leaveConfirmModal).toBeVisible();
	await leaveConfirmModal.getByRole('button', { name: 'Tinggalkan' }).click();
	await page.waitForURL('**/drive/my-storage');

	// 6. Test Database Backup Download directly when clean
	await page.goto('/admin/settings');
	await expect(page.getByTestId('btn-save-settings')).toBeDisabled();

	const downloadPromise = page.waitForEvent('download');
	await page.getByTestId('btn-download-backup').click();
	const download = await downloadPromise;

	// Reset default quota back to 100 GB for subsequent tests
	await page.request.put('/api/v1/admin/settings', {
		data: { default_quota: { mode: 'limited', gb: 100 }, worker_concurrency: 2 }
	});
	expect(download.suggestedFilename()).toContain('.db');
	await expect(
		page.getByText(
			/Backup (?:basis data|DB) berhasil diunduh|Database backup downloaded successfully/i
		)
	).toBeVisible();
});
test('storage node names can be edited and persist, while local creation requires a unique name', async ({
	page
}) => {
	await ensureAdminLoggedIn(page);
	await page.goto('/admin/settings');
	const nodesResponse = await page.request.get('/api/v1/admin/storage-nodes');
	expect(nodesResponse.ok()).toBeTruthy();
	const { nodes } = (await nodesResponse.json()) as {
		nodes: {
			id: string;
			name: string;
			path: string;
			used_bytes: number;
			capacity_cap_gb: number;
		}[];
	};
	expect(nodes.length).toBeGreaterThan(0);
	const firstNode = nodes[0]!;
	const localNode = nodes.find((node) => node.path.length > 0);
	if (!localNode) {
		test.skip();
		return;
	}
	const renamed = `${firstNode.name} E2E ${Date.now()}`;
	await page
		.getByRole('button', { name: /Ubah|Edit/i })
		.first()
		.click();
	const editName = page.getByTestId('input-edit-node-name');
	await editName.fill(`  ${renamed}  `);
	await page.getByRole('button', { name: 'Simpan Perubahan' }).click();
	await expect(page.getByText(renamed, { exact: true })).toBeVisible();
	await page.reload();
	await expect(page.getByText(renamed, { exact: true })).toBeVisible();

	await page
		.getByRole('button', { name: /Ubah|Edit/i })
		.first()
		.click();
	await page.getByTestId('input-edit-node-name').fill(firstNode.name);
	await page.getByRole('button', { name: 'Simpan Perubahan' }).click();
	await expect(page.getByText(firstNode.name, { exact: true })).toBeVisible();

	await page.getByTestId('btn-add-storage-node').click();
	const localName = page.getByTestId('input-local-node-name');
	const addLocal = page.getByRole('button', { name: 'Tambah Node Lokal' });
	await expect(addLocal).toBeDisabled();
	await page.getByLabel('Path Direktori').fill(localNode.path);
	await localName.fill(localNode.name);
	await expect(addLocal).toBeEnabled();
	const addNodeDialog = page.getByRole('dialog', { name: 'Tambah Node Penyimpanan' });
	const storageTabs = addNodeDialog.getByRole('tablist');
	const localTab = storageTabs.getByRole('tab', { name: 'Disk Lokal' });
	const cloudTab = storageTabs.getByRole('tab', { name: 'Cloud (S3)' });
	await expect(addNodeDialog).toBeVisible();
	await expect(localTab).toHaveAttribute('aria-selected', 'true');
	await expect(cloudTab).toHaveAttribute('aria-selected', 'false');
	await expectTabColors(localTab, true);
	await expectTabColors(cloudTab, false);
	await expect(localName).toBeVisible();
	await expect(addNodeDialog.getByLabel('Endpoint')).not.toBeVisible();
	await cloudTab.click();
	await expect(cloudTab).toHaveAttribute('aria-selected', 'true');
	await expect(localTab).toHaveAttribute('aria-selected', 'false');
	await expectTabColors(cloudTab, true);
	await expectTabColors(localTab, false);
	await expect(addNodeDialog.getByLabel('Endpoint')).toBeVisible();
	await expect(localName).not.toBeVisible();
	await localTab.click();
	await expect(localTab).toHaveAttribute('aria-selected', 'true');
	await expect(cloudTab).toHaveAttribute('aria-selected', 'false');
	await expect(localName).toBeVisible();
	await addLocal.click();
	await expect(page.getByTestId('error-local-node-name')).toContainText(
		/nama|name|already exists|sudah digunakan/i
	);
});

test('capacity warning follows per-node used bytes', async ({ page }) => {
	await ensureAdminLoggedIn(page);
	await page.goto('/admin/settings');
	const nodesResponse = await page.request.get('/api/v1/admin/storage-nodes');
	expect(nodesResponse.ok()).toBeTruthy();
	const { nodes } = (await nodesResponse.json()) as {
		nodes: { used_bytes: number }[];
	};
	expect(nodes.length).toBeGreaterThan(0);
	const usedBytes = nodes[0]!.used_bytes;
	test.skip(
		usedBytes <= 1024 ** 3,
		'Requires more than 1 GiB of node usage to lower capacity below usage.'
	);
	const belowUsageGb = Math.ceil(usedBytes / 1024 ** 3) - 1;

	await page
		.getByRole('button', { name: /Ubah|Edit/i })
		.first()
		.click();
	await page.getByTestId('input-edit-node-capacity').fill(String(belowUsageGb));
	await expect(page.getByTestId('warning-node-capacity')).toContainText('pemakaian node');
	await expect(page.getByRole('button', { name: 'Simpan Perubahan' })).toBeDisabled();
});
