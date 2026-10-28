import { test, expect } from '@playwright/test';

test.describe('Setup Cloud Storage S3 Connection Validation', () => {
	test('tests S3 connection with 4 distinct conditions: valid, unreachable, invalid credentials, invalid bucket', async ({
		page
	}) => {
		// 1. Check setup status first
		const statusRes = await page.request.get('/api/v1/setup/status');
		expect(statusRes.ok()).toBeTruthy();
		const status = (await statusRes.json()) as { is_setup_completed: boolean };

		if (status.is_setup_completed) {
			// Backend test-cloud endpoint rejection when setup already completed
			const directRes = await page.request.post('/api/v1/setup/test-cloud', {
				data: {
					enabled: true,
					provider: 'S3_COMPATIBLE',
					node_name: 'test-node',
					endpoint: 'http://127.0.0.1:19000',
					bucket: 'cubycloud-dev',
					region: 'us-east-1',
					access_key: 'cubycloud-app',
					secret_key: 'cubycloud-app-local-only-2026'
				}
			});
			expect(directRes.status()).toBe(403);
			return;
		}

		// Navigate to /setup
		await page.goto('/setup');
		await expect(page.getByRole('heading', { level: 1 })).toBeVisible();

		// Step 1: Admin info
		await page.getByLabel(/Nama pengguna|Username/i).fill('superadmin');
		await page.getByLabel(/Email/i).fill('admin@example.test');
		await page.getByLabel(/^Kata sandi$|^Password$/i).fill('SuperSecretPassword12345!');
		await page
			.getByLabel(/Konfirmasi kata sandi|Confirm password/i)
			.fill('SuperSecretPassword12345!');
		await page.getByRole('button', { name: /Lanjut|Next/i }).click();
		// Step 2: Storage nodes (select default local node)
		await expect(
			page.getByText(/Penyimpanan Lokal \(Opsional\)|Local Storage \(Optional\)/i)
		).toBeVisible();
		await page.getByRole('button', { name: /Lanjut|Next/i }).click();

		// Step 3: Cloud Storage
		await expect(page.getByText(/Cloud \(Opsional\)|Cloud \(Optional\)/i)).toBeVisible();
		const enableCloudSwitch = page.getByRole('switch', {
			name: /Aktifkan node cloud|Enable cloud node/i
		});
		if (!(await enableCloudSwitch.isChecked())) {
			await enableCloudSwitch.click();
		}
		// Fill fields
		const nodeNameInput = page.locator('#cloud-node-name');
		const endpointInput = page.locator('#cloud-endpoint');
		const bucketInput = page.locator('#cloud-bucket');
		const regionInput = page.locator('#cloud-region');
		const accessKeyInput = page.locator('#cloud-access');
		const secretKeyInput = page.locator('#cloud-secret');
		const testButton = page.getByRole('button', { name: /Uji koneksi|Test connection/i });

		await nodeNameInput.fill('minio-dev-test');
		await regionInput.fill('us-east-1');

		// --- Condition 1: Unreachable endpoint ---
		await endpointInput.fill('http://127.0.0.1:19999');
		await bucketInput.fill('cubycloud-dev');
		await accessKeyInput.fill('cubycloud-app');
		await secretKeyInput.fill('cubycloud-app-local-only-2026');

		await testButton.click();
		await expect(page.getByText(/Koneksi gagal|Connection failed/i)).toBeVisible();

		// --- Condition 2: Invalid credentials ---
		await endpointInput.fill('http://127.0.0.1:19000');
		await bucketInput.fill('cubycloud-dev');
		await accessKeyInput.fill('invalid-user');
		await secretKeyInput.fill('wrong-secret-key-12345');

		await testButton.click();
		await expect(page.getByText(/Koneksi gagal|Connection failed/i)).toBeVisible();

		// --- Condition 3: Invalid bucket ---
		await endpointInput.fill('http://127.0.0.1:19000');
		await bucketInput.fill('non-existent-bucket-999');
		await accessKeyInput.fill('cubycloud-app');
		await secretKeyInput.fill('cubycloud-app-local-only-2026');

		await testButton.click();
		await expect(page.getByText(/Koneksi gagal|Connection failed/i)).toBeVisible();

		// --- Condition 4: Valid MinIO connection ---
		await endpointInput.fill('http://127.0.0.1:19000');
		await bucketInput.fill('cubycloud-dev');
		await accessKeyInput.fill('cubycloud-app');
		await secretKeyInput.fill('cubycloud-app-local-only-2026');

		await testButton.click();
		await expect(page.getByText(/Koneksi berhasil|Connection successful/i)).toBeVisible();
		await expect(page.getByText(/terverifikasi|verified/i)).toBeVisible();
	});
});
