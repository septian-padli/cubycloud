import { afterEach, expect, test } from 'bun:test';
import { myStorageAPI } from './my-storage.js';

const originalFetch = globalThis.fetch;
const originalXHR = globalThis.XMLHttpRequest;
const chunkSize = 2 * 1024 * 1024;

afterEach(() => {
	globalThis.fetch = originalFetch;
	globalThis.XMLHttpRequest = originalXHR;
});

function json(body: unknown, status = 200, headers?: HeadersInit): Response {
	return new Response(JSON.stringify(body), { status, headers });
}

test('S3 PUT succeeds but metadata sync fails: retain the upload intent for retry', async () => {
	const methods: string[] = [];
	globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
		const url = String(input);
		methods.push(`${init?.method ?? 'GET'} ${url}`);
		if (url.endsWith('/upload-intent') && init?.method === 'POST') {
			return json({ placement: 's3', upload_id: 'intent-1', url: 'https://s3.example.test/object' });
		}
		if (url.endsWith('/sync-cloud')) return json({ error: 'temporary failure' }, 503);
		return json({}, 204);
	}) as typeof fetch;
	class SuccessfulPut {
		upload = { onprogress: null };
		status = 200;
		onload = () => {};
		onerror = () => {};
		open() {}
		setRequestHeader() {}
		send() { queueMicrotask(() => this.onload()); }
	}
	globalThis.XMLHttpRequest = SuccessfulPut as unknown as typeof XMLHttpRequest;

	await expect(myStorageAPI.upload(new File(['cloud'], 'cloud.txt'), null, () => {})).rejects.toMatchObject({ status: 503 });
	expect(methods).toEqual([
		'POST /api/v1/drive/my-storage/upload-intent',
		'POST /api/v1/drive/sync-cloud'
	]);
});

for (const [label, remoteOffset] of [['remaining chunk', chunkSize], ['terminal offset', chunkSize + 3]] as const) {
	test(`TUS HEAD recovery continues at ${label} and returns finalized item`, async () => {
		const patches: { offset: number; size: number }[] = [];
		let headCount = 0;
		globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
			const url = String(input);
			if (url.includes('/upload-intent')) return json({ placement: 'local' });
			if (url.includes('/upload-tus') && init?.method === 'POST') {
				return new Response(null, { status: 201, headers: { Location: '/api/v1/drive/my-storage/upload-tus/tus-1' } });
			}
			if (init?.method === 'HEAD') {
				headCount++;
				return new Response(null, { status: 200, headers: { 'Upload-Offset': String(remoteOffset) } });
			}
			throw new Error(`Unexpected request: ${init?.method} ${url}`);
		}) as typeof fetch;
		class Patch {
			upload = { onprogress: null };
			status = 0;
			responseText = '';
			onload = () => {};
			onerror = () => {};
			private requestHeaders: Record<string, string> = {};
			private responseHeaders: Record<string, string> = {};
			open() {}
			setRequestHeader(name: string, value: string) { this.requestHeaders[name] = value; }
			getResponseHeader(name: string) { return this.responseHeaders[name] ?? null; }
			send(chunk: Blob) {
				patches.push({ offset: Number(this.requestHeaders['Upload-Offset']), size: chunk.size });
				this.status = patches.length === 1 ? 409 : 204;
				if (this.status === 204) {
					this.responseHeaders['Upload-Offset'] = String(chunkSize + 3);
					this.responseHeaders['Upload-Item'] = btoa(JSON.stringify({ id: 'completed', name: 'big.bin' }));
				}
				queueMicrotask(() => this.onload());
			}
		}
		globalThis.XMLHttpRequest = Patch as unknown as typeof XMLHttpRequest;

		const item = await myStorageAPI.upload(new File([new Uint8Array(chunkSize + 3)], 'big.bin'), null, () => {});
		expect(item.id).toBe('completed');
		expect(headCount).toBe(1);
		expect(patches).toEqual([
			{ offset: 0, size: chunkSize },
			{ offset: remoteOffset, size: chunkSize + 3 - remoteOffset }
		]);
	});
}

test('TUS recovery when remoteOffset equals client offset retries chunk and succeeds', async () => {
	const patches: { offset: number; size: number }[] = [];
	let headCount = 0;
	globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
		const url = String(input);
		if (url.includes('/upload-intent')) return json({ placement: 'local' });
		if (url.includes('/upload-tus') && init?.method === 'POST') {
			return new Response(null, { status: 201, headers: { Location: '/api/v1/drive/my-storage/upload-tus/tus-retry' } });
		}
		if (init?.method === 'HEAD') {
			headCount++;
			return new Response(null, { status: 200, headers: { 'Upload-Offset': '0' } });
		}
		throw new Error(`Unexpected request: ${init?.method} ${url}`);
	}) as typeof fetch;

	class RetryPatch {
		upload = { onprogress: null };
		status = 0;
		responseText = '';
		onload = () => {};
		onerror = () => {};
		private requestHeaders: Record<string, string> = {};
		private responseHeaders: Record<string, string> = {};
		open() {}
		setRequestHeader(name: string, value: string) { this.requestHeaders[name] = value; }
		getResponseHeader(name: string) { return this.responseHeaders[name] ?? null; }
		send(chunk: Blob) {
			patches.push({ offset: Number(this.requestHeaders['Upload-Offset']), size: chunk.size });
			this.status = patches.length === 1 ? 500 : 204;
			if (this.status === 204) {
				this.responseHeaders['Upload-Offset'] = String(chunkSize);
				this.responseHeaders['Upload-Item'] = btoa(JSON.stringify({ id: 'recovered', name: 'chunk.bin' }));
			}
			queueMicrotask(() => this.onload());
		}
	}
	globalThis.XMLHttpRequest = RetryPatch as unknown as typeof XMLHttpRequest;

	const item = await myStorageAPI.upload(new File([new Uint8Array(chunkSize)], 'chunk.bin'), null, () => {});
	expect(item.id).toBe('recovered');
	expect(headCount).toBe(1);
	expect(patches).toEqual([
		{ offset: 0, size: chunkSize },
		{ offset: 0, size: chunkSize }
	]);
});

test('TUS recovery on final retry: network error on last PATCH response still checks HEAD and recovers', async () => {
	const patches: { offset: number; size: number }[] = [];
	let headCount = 0;
	globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
		const url = String(input);
		if (url.includes('/upload-intent')) return json({ placement: 'local' });
		if (url.includes('/upload-tus') && init?.method === 'POST') {
			return new Response(null, { status: 201, headers: { Location: '/api/v1/drive/my-storage/upload-tus/tus-last-retry' } });
		}
		if (init?.method === 'HEAD') {
			headCount++;
			// HEAD returns terminal offset proving server received bytes despite network drop
			return new Response(null, { status: 200, headers: { 'Upload-Offset': String(chunkSize + 3) } });
		}
		throw new Error(`Unexpected request: ${init?.method} ${url}`);
	}) as typeof fetch;

	class LastAttemptPatch {
		upload = { onprogress: null };
		status = 0;
		responseText = '';
		onload = () => {};
		onerror = () => {};
		private requestHeaders: Record<string, string> = {};
		private responseHeaders: Record<string, string> = {};
		open() {}
		setRequestHeader(name: string, value: string) { this.requestHeaders[name] = value; }
		getResponseHeader(name: string) { return this.responseHeaders[name] ?? null; }
		send(chunk: Blob) {
			patches.push({ offset: Number(this.requestHeaders['Upload-Offset']), size: chunk.size });
			if (patches.length < 3) {
				this.status = 500;
				queueMicrotask(() => this.onload());
			} else if (patches.length === 3) {
				// Attempt 3: server writes chunk, but client receives network error
				queueMicrotask(() => this.onerror());
			} else {
				// Finalizer PATCH with 0 bytes after HEAD recovered terminal offset
				this.status = 204;
				this.responseHeaders['Upload-Offset'] = String(chunkSize + 3);
				this.responseHeaders['Upload-Item'] = btoa(JSON.stringify({ id: 'finalized-after-head', name: 'last.bin' }));
				queueMicrotask(() => this.onload());
			}
		}
	}
	globalThis.XMLHttpRequest = LastAttemptPatch as unknown as typeof XMLHttpRequest;

	const item = await myStorageAPI.upload(new File([new Uint8Array(chunkSize + 3)], 'last.bin'), null, () => {});
	expect(item.id).toBe('finalized-after-head');
	expect(headCount).toBeGreaterThanOrEqual(1);
	expect(patches[patches.length - 1]).toEqual({ offset: chunkSize + 3, size: 0 });
});
