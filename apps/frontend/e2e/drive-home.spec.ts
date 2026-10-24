import { test, expect, type Page } from '@playwright/test';

const superAdminUsername = 'storage-e2e-admin';
const superAdminPassword = 'StorageE2E-password-2026';
const recoveryKey = 'ABCDE-FGHKL-MNPQR-STUVW-XYZ23-45678';

const userHomeA = {
	username: 'alice-home',
	email: 'alice-home@example.test',
	password: 'AliceHomePassword2026!'
};

const userHomeB = {
	username: 'bob-home',
	email: 'bob-home@example.test',
	password: 'BobHomePassword2026!'
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
	await page.locator('#login-identifier').fill(username);
	await page.locator('#login-password').fill(password);
	await page.getByRole('button', { name: /Masuk|Sign in/i }).click();
	await page.waitForURL('**/drive/home');
}

async function uploadFileBrowser(page: Page, baseUrl: string, fileName: string, content = 'isi berkas contoh') {
	const url = `${baseUrl}${baseUrl.includes('?') ? '&' : '?'}filename=${encodeURIComponent(fileName)}`;
	return page.evaluate(
		async ({ url, fileName, content }) => {
			const formData = new FormData();
			const blob = new Blob([content], { type: 'text/plain' });
			formData.append('file', blob, fileName);
			const res = await fetch(url, {
				method: 'POST',
				body: formData,
				credentials: 'include'
			});
			return { ok: res.ok, status: res.status, json: await res.json().catch(() => null) };
		},
		{ url, fileName, content }
	);
}

async function apiRequestBrowser(page: Page, method: string, url: string, body?: unknown) {
	return page.evaluate(
		async ({ method, url, body }) => {
			const res = await fetch(url, {
				method,
				headers: body ? { 'Content-Type': 'application/json' } : undefined,
				body: body ? JSON.stringify(body) : undefined,
				credentials: 'include'
			});
			return { ok: res.ok, status: res.status, json: await res.json().catch(() => null) };
		},
		{ method, url, body }
	);
}

test.describe('Drive Home Page (/drive/home)', () => {
	test.beforeEach(async ({ page }) => {
		await ensureSetup(page);
		await loginUser(page, superAdminUsername, superAdminPassword);

		// Ensure test users exist
		for (const user of [userHomeA, userHomeB]) {
			await apiRequestBrowser(page, 'POST', '/api/v1/admin/users', {
				username: user.username,
				email: user.email,
				password: user.password,
				quota_mode: 'DEFAULT'
			});
		}
	});

	test('shows empty states for new user with no activity or accessed files', async ({ page }) => {
		await loginUser(page, userHomeB.username, userHomeB.password);
		await page.goto('/drive/home');

		await expect(page.getByRole('heading', { name: 'Beranda' })).toBeVisible();
		await expect(page.getByTestId('quick-access-empty')).toBeVisible();
		await expect(page.getByTestId('recent-activities-empty')).toBeVisible();
	});

	test('Quick Access displays up to 5 recently opened items in newest-first order and supports opening', async ({
		page
	}) => {
		await loginUser(page, userHomeA.username, userHomeA.password);

		// 1. Upload 6 files into My Storage via authenticated browser session
		const fileNames = ['item-1.txt', 'item-2.txt', 'item-3.txt', 'item-4.txt', 'item-5.txt', 'item-6.txt'];
		const fileIDs: string[] = [];

		for (const name of fileNames) {
			const res = await uploadFileBrowser(page, '/api/v1/drive/my-storage/upload', name, `Isi berkas ${name}`);
			expect(res.ok).toBeTruthy();
			fileIDs.push(res.json.id);
		}

		// 2. Record access sequentially from item-1 to item-6
		for (let i = 0; i < fileIDs.length; i++) {
			const touchRes = await apiRequestBrowser(page, 'POST', '/api/v1/drive/access', {
				item_id: fileIDs[i],
				source: 'my-storage'
			});
			expect(touchRes.ok).toBeTruthy();
		}

		// 3. Visit Home
		await page.goto('/drive/home');
		await expect(page.getByRole('heading', { name: 'Beranda' })).toBeVisible();

		// Quick access must have exactly 5 items (item-6 down to item-2, item-1 excluded because cap is 5)
		const quickAccess = page.getByRole('listbox', { name: 'Akses cepat' });
		await expect(quickAccess.getByText('item-6.txt')).toBeVisible();
		await expect(quickAccess.getByText('item-5.txt')).toBeVisible();
		await expect(quickAccess.getByText('item-4.txt')).toBeVisible();
		await expect(quickAccess.getByText('item-3.txt')).toBeVisible();
		await expect(quickAccess.getByText('item-2.txt')).toBeVisible();
		await expect(quickAccess.getByText('item-1.txt')).toHaveCount(0);

		// 4. Click item-6.txt in Quick Access -> should navigate to preview URL
		await quickAccess.getByText('item-6.txt').click();
		await page.waitForURL((url) => url.pathname.includes('/drive/my-storage') && url.searchParams.has('preview'));
	});

	test('Recent Activities displays 5 latest upload/edit actions with deduplication and user isolation in Organization', async ({
		page
	}) => {
		// Alice logs in and performs actions
		await loginUser(page, userHomeA.username, userHomeA.password);

		// Action 1: Upload a doc
		const up1 = await uploadFileBrowser(page, '/api/v1/drive/my-storage/upload', 'dokumen-awal.txt', 'draft awal');
		expect(up1.ok).toBeTruthy();
		const doc1 = up1.json;

		// Action 2: Rename doc1 -> Action: edit (should deduplicate doc1 in activities)
		const ren1 = await apiRequestBrowser(page, 'PATCH', `/api/v1/drive/my-storage/${doc1.id}`, {
			name: 'dokumen-revisi.txt'
		});
		expect(ren1.ok).toBeTruthy();

		// Action 3: Create folder
		const f1 = await apiRequestBrowser(page, 'POST', '/api/v1/drive/my-storage/folders', {
			name: 'Folder Arsip',
			parent_id: null
		});
		expect(f1.ok).toBeTruthy();
		const folder1 = f1.json;

		// Action 4: Move doc1 into folder -> Action: edit (should keep doc1 deduplicated with latest action)
		const mov1 = await apiRequestBrowser(page, 'PATCH', `/api/v1/drive/items/${doc1.id}/move`, {
			target_parent_id: folder1.id
		});
		expect(mov1.ok).toBeTruthy();

		// Action 5: Upload an image in My Storage
		const img1 = await uploadFileBrowser(page, '/api/v1/drive/my-storage/upload', 'foto-liburan.jpg', 'image bytes');
		expect(img1.ok).toBeTruthy();

		// Action 6: Upload in Organization by Alice
		const orgAlice = await uploadFileBrowser(
			page,
			'/api/v1/drive/organization/upload',
			'alice-org-doc.pdf',
			'%PDF-1.4 test'
		);
		expect(orgAlice.ok).toBeTruthy();

		// Now Bob logs in and uploads in Organization
		await loginUser(page, userHomeB.username, userHomeB.password);
		const orgBob = await uploadFileBrowser(
			page,
			'/api/v1/drive/organization/upload',
			'bob-secret-plan.pdf',
			'%PDF-1.4 bob secret'
		);
		expect(orgBob.ok).toBeTruthy();

		// Switch back to Alice and view Home
		await loginUser(page, userHomeA.username, userHomeA.password);
		await page.goto('/drive/home');

		// 1. Verify Bob's organization activity does NOT leak into Alice's home
		await expect(page.getByText('bob-secret-plan.pdf')).toHaveCount(0);

		// 2. Alice's own organization upload is visible
		await expect(page.getByText('alice-org-doc.pdf')).toBeVisible();

		// 3. Verify deduplication: dokumen-revisi.txt should only be listed once
		await expect(page.getByText('dokumen-revisi.txt')).toHaveCount(1);
		await expect(page.getByText('dokumen-awal.txt')).toHaveCount(0);

		// 4. Verify Folder Arsip is visible
		await expect(page.getByText('Folder Arsip')).toBeVisible();
	});
});
