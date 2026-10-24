import { randomUUID } from 'node:crypto';
import { expect, test, type Page } from '@playwright/test';
import { existsSync, readFileSync, renameSync, rmSync, truncateSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { restartControlDir } from './restart-control.js';

const username = 'storage-e2e-admin';
const password = 'StorageE2E-password-2026';
const recoveryKey = 'ABCDE-FGHKL-MNPQR-STUVW-XYZ23-45678';
const restartDir = restartControlDir;
const restartRequestPath = join(restartDir, 'restart-request.json');
const restartResultPath = join(restartDir, 'restart-result.json');
let multipartTempPath: string | undefined;

test.afterEach(() => {
	if (multipartTempPath) rmSync(multipartTempPath, { force: true });
});

function writeJSONAtomically(path: string, value: unknown) {
	const temporaryPath = `${path}.${process.pid}.tmp`;
	writeFileSync(temporaryPath, JSON.stringify(value));
	renameSync(temporaryPath, path);
}

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
					admin: { username, email: 's3-admin@example.test', password, recovery_key: recoveryKey },
					storage_nodes: nodes.map((node) => ({ ...node, enabled: true })),
					placement_strategy: 'MOST_FREE_SPACE',
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

test('Super Admin configures MinIO S3 node via UI, uploads/downloads directly, persists across backend restart, and deletes', async ({
	page
}) => {
	test.setTimeout(180_000);
	expect(
		existsSync(restartResultPath),
		`Missing initial supervisor result at ${restartResultPath}`
	).toBeTruthy();
	await ensureAdminLoggedIn(page);

	// 1. Navigate to /admin/settings and open Add Storage Node modal
	await page.goto('/admin/settings');
	await expect(page.getByRole('heading', { name: /Pengaturan Admin|Admin Settings/i })).toBeVisible();
	await expect(page.getByRole('heading', { name: 'Node penyimpanan' })).toBeVisible();

	await page.getByTestId('btn-add-storage-node').click();
	const dialog = page.getByRole('dialog');
	await expect(dialog).toBeVisible();

	// 2. Switch to Cloud (S3) tab
	await dialog.getByRole('tab', { name: 'Cloud (S3)' }).click();

	// 3. Fill MinIO credentials from development compose
	const nodeName = `MinIO Dev ${Date.now()}`;
	await dialog.getByTestId('input-cloud-node-name').fill(nodeName);
	await dialog.locator('#cloud-endpoint').fill('http://127.0.0.1:19000');
	await dialog.locator('#cloud-bucket').fill('cubycloud-dev');
	await dialog.locator('#cloud-region').fill('us-east-1');
	await dialog.locator('#cloud-access-key').fill('cubycloud-app');
	await dialog.locator('#cloud-secret-key').fill('cubycloud-app-local-only-2026');
	await dialog.locator('#cloud-capacity').fill('500');

	// Submit add node
	const addCloudBtn = dialog.getByRole('button', { name: 'Tambah Node Cloud' });
	await expect(addCloudBtn).toBeEnabled();
	await addCloudBtn.click();

	// 4. Verify node appears in StorageNodesGrid with S3 badge and bucket info
	await expect(dialog).not.toBeVisible();
	const nodeCard = page.locator('div').filter({ hasText: nodeName }).first();
	await expect(nodeCard).toBeVisible();
	await expect(page.getByText('Bucket: cubycloud-dev').first()).toBeVisible();
	await expect(page.getByText('S3').first()).toBeVisible();

	// Isolate placement to MinIO so host disk capacity cannot affect the S3 upload.
	const storageNodesRes = await page.request.get('/api/v1/admin/storage-nodes');
	expect(storageNodesRes.ok()).toBeTruthy();
	const storageNodes = (await storageNodesRes.json()) as {
		nodes: { id: string; kind: string; enabled: boolean }[];
	};
	expect(storageNodes.nodes.some((node) => node.kind === 'S3' && node.enabled)).toBeTruthy();
	for (const node of storageNodes.nodes.filter((node) => node.kind !== 'S3' && node.enabled)) {
		const disableNodeRes = await page.request.put(`/api/v1/admin/storage-nodes/${node.id}/disable`);
		expect(disableNodeRes.ok()).toBeTruthy();
	}
	const enabledNodesRes = await page.request.get('/api/v1/admin/storage-nodes');
	expect(enabledNodesRes.ok()).toBeTruthy();
	const enabledNodes = (await enabledNodesRes.json()) as {
		nodes: { kind: string; enabled: boolean }[];
	};
	const nonS3EnabledNodes = enabledNodes.nodes.filter((node) => node.enabled && node.kind !== 'S3');
	expect(nonS3EnabledNodes).toHaveLength(0);

	// Verify persistence via reload
	await page.reload();
	await expect(page.getByText(nodeName).first()).toBeVisible();

	// 5. Use MOST_FREE_SPACE with MinIO as the only enabled storage node.
	const strategyRes = await page.request.put('/api/v1/admin/settings', {
		data: { placement_strategy: 'MOST_FREE_SPACE' }
	});
	expect(strategyRes.ok()).toBeTruthy();
	await page.goto('/drive/my-storage');
	await page.waitForURL('**/drive/my-storage');

	const testContent = `S3 Integration Test Payload ${Date.now()} - Verified End-to-End with MinIO!`;
	const filename = `s3-proof-${Date.now()}.txt`;

	// Exercise the real browser upload path, including the cross-origin S3 PUT and metadata sync.
	const intentResponsePromise = page.waitForResponse(
		(response) =>
			response.url().endsWith('/api/v1/drive/my-storage/multipart-intent') &&
			response.request().method() === 'POST'
	);
	const s3PutResponsePromise = page.waitForResponse(
		(response) =>
			response.url().startsWith('http://127.0.0.1:19000/cubycloud-dev/') &&
			response.request().method() === 'PUT',
		{ timeout: 15_000 }
	);
	const syncResponsePromise = page.waitForResponse(
		(response) =>
			response.url().endsWith('/api/v1/drive/my-storage/sync-cloud') &&
			response.request().method() === 'POST',
		{ timeout: 15_000 }
	);
	await page.getByTestId('upload-file-input').setInputFiles({
		name: filename,
		mimeType: 'text/plain',
		buffer: Buffer.from(testContent)
	});

	const intentRes = await intentResponsePromise;
	expect(intentRes.ok()).toBeTruthy();
	const intent = (await intentRes.json()) as {
		placement: string;
		mode: string;
		upload_id: string;
		url: string;
		method: string;
	};
	expect(intent.placement).toBe('s3');
	expect(intent.mode).toBe('single_put');
	expect(intent.url).toContain('127.0.0.1:19000/cubycloud-dev');
	expect(intent.upload_id).toBeTruthy();

	const s3PutRes = await s3PutResponsePromise;
	expect(s3PutRes.ok()).toBeTruthy();
	const syncRes = await syncResponsePromise;
	expect(syncRes.ok()).toBeTruthy();
	const createdItem = (await syncRes.json()) as { id: string; name: string };
	expect(createdItem.id).toBe(intent.upload_id);
	const multipartSize = 100 * 1024 * 1024;
	const multipartFilename = `cubycloud-s3-multipart-${randomUUID()}.bin`;
	const multipartPath = join(tmpdir(), multipartFilename);
	multipartTempPath = multipartPath;
	writeFileSync(multipartPath, '');
	truncateSync(multipartPath, multipartSize);

	const multipartIntentResponsePromise = page.waitForResponse(
		(response) =>
			response.url().endsWith('/api/v1/drive/my-storage/multipart-intent') &&
			response.request().method() === 'POST'
	);
	const multipartPartResponsePromise = page.waitForResponse(
		(response) => {
			const url = new URL(response.url());
			return (
				response.request().method() === 'PUT' &&
				url.origin === 'http://127.0.0.1:19000' &&
				url.searchParams.has('partNumber') &&
				url.searchParams.has('uploadId')
			);
		},
		{ timeout: 60_000 }
	);
	const multipartCompleteResponsePromise = page.waitForResponse(
		(response) =>
			response.url().includes('/api/v1/drive/my-storage/multipart/') &&
			response.url().endsWith('/complete') &&
			response.request().method() === 'POST',
		{ timeout: 60_000 }
	);
	await page.getByTestId('upload-file-input').setInputFiles(multipartPath);

	const multipartIntentResponse = await multipartIntentResponsePromise;
	expect(multipartIntentResponse.ok()).toBeTruthy();
	const multipartIntent = await multipartIntentResponse.json();
	expect(multipartIntent).toMatchObject({
		placement: 's3',
		mode: 'multipart',
		upload_id: expect.any(String),
		part_count: expect.any(Number)
	});
	expect(multipartIntent.part_count).toBeGreaterThan(1);
	const multipartPartResponse = await multipartPartResponsePromise;
	expect(multipartPartResponse.ok()).toBeTruthy();
	expect(multipartPartResponse.headers()['etag']).toBeTruthy();
	const multipartCompleteResponse = await multipartCompleteResponsePromise;
	expect(multipartCompleteResponse.status()).toBe(201);
	const multipartItem = await multipartCompleteResponse.json();
	expect(multipartItem).toMatchObject({
		id: expect.any(String),
		name: multipartFilename,
		size_bytes: multipartSize
	});
	await expect(page.getByRole('option').filter({ hasText: multipartFilename })).toBeVisible();

	// Exercise the same S3 multipart path through the Organization drive.
	await page.goto('/drive/organization');
	await expect(page.getByRole('heading', { name: /Organization|Organisasi/i })).toBeVisible();
	const orgIntentResponsePromise = page.waitForResponse(
		(response) =>
			response.url().endsWith('/api/v1/drive/organization/multipart-intent') &&
			response.request().method() === 'POST'
	);
	const orgPartResponsePromise = page.waitForResponse(
		(response) => {
			const url = new URL(response.url());
			return (
				response.request().method() === 'PUT' &&
				url.origin === 'http://127.0.0.1:19000' &&
				url.searchParams.has('partNumber') &&
				url.searchParams.has('uploadId')
			);
		},
		{ timeout: 60_000 }
	);
	const orgCompleteResponsePromise = page.waitForResponse(
		(response) =>
			response.url().includes('/api/v1/drive/organization/multipart/') &&
			response.url().endsWith('/complete') &&
			response.request().method() === 'POST',
		{ timeout: 60_000 }
	);
	await page.getByTestId('upload-file-input').setInputFiles(multipartPath);
	const orgIntentResponse = await orgIntentResponsePromise;
	expect(orgIntentResponse.ok()).toBeTruthy();
	const orgIntent = await orgIntentResponse.json();
	expect(orgIntent).toMatchObject({
		placement: 's3',
		mode: 'multipart',
		upload_id: expect.any(String),
		part_count: expect.any(Number)
	});
	expect(orgIntent.part_count).toBeGreaterThan(1);
	expect((await orgPartResponsePromise).ok()).toBeTruthy();
	const orgCompleteResponse = await orgCompleteResponsePromise;
	expect(orgCompleteResponse.status()).toBe(201);
	const orgItem = await orgCompleteResponse.json();
	expect(orgItem).toMatchObject({
		id: expect.any(String),
		name: multipartFilename,
		size_bytes: multipartSize
	});
	const orgListingResponse = await page.request.get('/api/v1/drive/organization');
	expect(orgListingResponse.ok()).toBeTruthy();
	const orgListing = await orgListingResponse.json();
	expect(orgListing).toMatchObject({
		items: expect.arrayContaining([expect.objectContaining({ id: orgItem.id })])
	});
	// Ask the Playwright-owned supervisor to stop its backend child and start a fresh process.
	await expect.poll(() => existsSync(restartResultPath), { timeout: 120_000 }).toBeTruthy();
	const previousBackend = JSON.parse(readFileSync(restartResultPath, 'utf8')) as {
		generation: number;
		pid: number;
	};
	const requestId = `restart-${Date.now()}`;
	writeJSONAtomically(restartRequestPath, { requestId });
	await expect
		.poll(
			() => {
				const result = JSON.parse(readFileSync(restartResultPath, 'utf8')) as {
					requestId: string | null;
					generation: number;
					pid: number;
				};
				return result.requestId === requestId ? result : null;
			},
			{ timeout: 120_000 }
		)
		.toMatchObject({
			requestId,
			generation: previousBackend.generation + 1
		});
	const restartedBackend = JSON.parse(readFileSync(restartResultPath, 'utf8')) as {
		requestId: string;
		generation: number;
		pid: number;
	};
	expect(restartedBackend.pid).not.toBe(previousBackend.pid);
	await expect
		.poll(
			async () => {
				const response = await page.request.get('/api/v1/setup/status');
				return response.ok();
			},
			{ timeout: 120_000 }
		)
		.toBeTruthy();

	// Reload persisted S3 node settings from the restarted process.
	await page.goto('/admin/settings', { waitUntil: 'domcontentloaded' }).catch(() => page.goto('/admin/settings'));
	await expect(page.getByText(nodeName).first()).toBeVisible();
	await expect(page.getByText('Bucket: cubycloud-dev').first()).toBeVisible();
	await expect(page.getByText('S3').first()).toBeVisible();

	// Reload My Storage through the fresh backend and verify its metadata and stored bytes.
	await page.goto('/drive/my-storage');
	const persistedItem = page.getByRole('option').filter({ hasText: filename });
	await expect(page.getByRole('option').filter({ hasText: multipartFilename })).toBeVisible();
	const persistedOrgListingResponse = await page.request.get('/api/v1/drive/organization');
	expect(persistedOrgListingResponse.ok()).toBeTruthy();
	const persistedOrgListing = await persistedOrgListingResponse.json();
	expect(persistedOrgListing).toMatchObject({
		items: expect.arrayContaining([expect.objectContaining({ id: orgItem.id })])
	});
	await expect(persistedItem).toBeVisible();
	const persistedDownloadUrlRes = await page.request.get(
		`/api/v1/drive/my-storage/${createdItem.id}/download-url`
	);
	expect(persistedDownloadUrlRes.ok()).toBeTruthy();
	const persistedDownloadData = (await persistedDownloadUrlRes.json()) as { url: string };
	const persistedDownloadRes = await page.request.get(persistedDownloadData.url);
	expect(persistedDownloadRes.ok()).toBeTruthy();
	expect(await persistedDownloadRes.body()).toEqual(Buffer.from(testContent));
	const persistedStreamRes = await page.request.get(
		`/api/v1/drive/my-storage/${createdItem.id}/download`
	);
	expect(persistedStreamRes.ok()).toBeTruthy();
	expect(await persistedStreamRes.body()).toEqual(Buffer.from(testContent));

	// Verify the completed upload appears in My Storage.
	await expect(page.getByRole('option').filter({ hasText: filename })).toBeVisible();

	// Verify both the signed direct download and backend stream path preserve the exact payload.
	const downloadUrlRes = await page.request.get(
		`/api/v1/drive/my-storage/${createdItem.id}/download-url`
	);
	expect(downloadUrlRes.ok()).toBeTruthy();
	const downloadData = (await downloadUrlRes.json()) as { url: string; filename: string };
	expect(downloadData.url).toContain('127.0.0.1:19000/cubycloud-dev');

	const downloadedContentRes = await page.request.get(downloadData.url);
	expect(downloadedContentRes.ok()).toBeTruthy();
	expect(await downloadedContentRes.text()).toBe(testContent);

	const streamDownloadRes = await page.request.get(
		`/api/v1/drive/my-storage/${createdItem.id}/download`
	);
	expect(streamDownloadRes.ok()).toBeTruthy();
	expect(await streamDownloadRes.text()).toBe(testContent);

	// Exercise the actual browser download action against the presigned S3 URL.
	const fileRow = page.getByRole('option').filter({ hasText: filename });
	await fileRow.getByRole('button', { name: 'Aksi berkas' }).click();
	const browserDownloadPromise = page.waitForEvent('download');
	await page.getByRole('menuitem', { name: 'Unduh' }).click();
	const browserDownload = await browserDownloadPromise;
	expect(browserDownload.suggestedFilename()).toBe(filename);
	const browserDownloadPath = await browserDownload.path();
	expect(readFileSync(browserDownloadPath, 'utf8')).toBe(testContent);

	// Soft-delete through the application UI, then purge from Trash.
	await fileRow.getByRole('button', { name: 'Aksi berkas' }).click();
	await page.getByRole('menuitem', { name: 'Hapus ke Trash' }).click();
	const deleteModal = page.getByTestId('confirm-dialog');
	await expect(deleteModal).toBeVisible();
	await deleteModal.getByRole('button', { name: 'Hapus ke Trash' }).click();
	await expect(fileRow).not.toBeVisible();
	await page.goto('/drive/trash');
	await expect(page.getByText(filename).first()).toBeVisible();

	const purgeRes = await page.request.delete(`/api/v1/drive/trash/${createdItem.id}`);
	expect(purgeRes.ok()).toBeTruthy();
	await page.reload();
	await expect(page.getByText(filename)).not.toBeVisible();
	const deleteMultipartRes = await page.request.delete(
		`/api/v1/drive/my-storage/${multipartItem.id}`
	);
	expect(deleteMultipartRes.ok()).toBeTruthy();
	const purgeMultipartRes = await page.request.delete(`/api/v1/drive/trash/${multipartItem.id}`);
	expect(purgeMultipartRes.ok()).toBeTruthy();
	const deleteOrgRes = await page.request.delete(`/api/v1/drive/organization/${orgItem.id}`);
	expect(deleteOrgRes.ok()).toBeTruthy();
	const purgeOrgRes = await page.request.delete(`/api/v1/drive/trash/${orgItem.id}`);
	expect(purgeOrgRes.ok()).toBeTruthy();

	// Reset placement strategy to default FILL_FIRST.
	const resetStrategyRes = await page.request.put('/api/v1/admin/settings', {
		data: { placement_strategy: 'FILL_FIRST' }
	});
	expect(resetStrategyRes.ok()).toBeTruthy();

	// Re-enable local storage nodes that were disabled for MinIO isolation.
	for (const node of storageNodes.nodes.filter((node) => node.kind !== 'S3' && node.enabled)) {
		const enableNodeRes = await page.request.put(`/api/v1/admin/storage-nodes/${node.id}`, {
			data: { enabled: true }
		});
		expect(enableNodeRes.ok()).toBeTruthy();
	}
});
