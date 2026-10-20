import { afterEach, expect, test } from 'bun:test';
import {
	createTusTransfer,
	decodeTusItem,
	encodeTusMetadata,
	type TusProgress,
	type TusSessionSnapshot,
	type TusRunResult
} from './tus-transfer.js';

const originalFetch = globalThis.fetch;
const originalXHR = globalThis.XMLHttpRequest;

afterEach(() => {
	globalThis.fetch = originalFetch;
	globalThis.XMLHttpRequest = originalXHR;
});

interface ScriptedResponse {
	status: number;
	headers?: Record<string, string>;
	// Leave the request in flight (for pause tests).
	pending?: boolean;
}

class MockXHR {
	static queue: ScriptedResponse[] = [];
	static sent: Array<{ url: string; headers: Record<string, string> }> = [];

	withCredentials = false;
	status = 0;
	responseText = '';
	responseHeaders: Record<string, string> = {};
	upload = {
		onprogress: null as
			((event: { lengthComputable: boolean; loaded: number; total: number }) => void) | null
	};
	onload: (() => void) | null = null;
	onerror: (() => void) | null = null;
	onabort: (() => void) | null = null;
	private _url = '';
	private _headers: Record<string, string> = {};

	open(_method: string, url: string) {
		this._url = url;
	}
	setRequestHeader(name: string, value: string) {
		this._headers[name] = value;
	}
	getResponseHeader(name: string) {
		return this.responseHeaders[name] ?? null;
	}
	send(body: Blob | string | null) {
		MockXHR.sent.push({ url: this._url, headers: { ...this._headers } });
		const response = MockXHR.queue.shift();
		if (!response) {
			queueMicrotask(() => this.onerror?.());
			return;
		}
		if (response.pending) return; // keep the request in flight
		this.status = response.status;
		this.responseText = '';
		this.responseHeaders = response.headers ?? {};
		queueMicrotask(() => this.onload?.());
	}
	abort() {
		this.onabort?.();
	}
}

function makeFile(content: string, name = 'u.txt'): File {
	return new File([content], name, { type: 'text/plain' });
}

function createdResponse(location: string): Response {
	return new Response(null, { status: 201, headers: { Location: location } });
}

test('successful transfer: creation headers, patch offsets, phases, final item', async () => {
	const fetchUrls: string[] = [];
	globalThis.fetch = (async (url: string | URL) => {
		fetchUrls.push(String(url));
		return createdResponse('/api/v1/drive/organization/upload-tus/TUSID1');
	}) as unknown as typeof fetch;

	MockXHR.queue = [
		{ status: 204, headers: { 'Upload-Offset': '4' } },
		{ status: 204, headers: { 'Upload-Offset': '10', 'Upload-Item-Id': 'item-1' } }
	];
	MockXHR.sent = [];
	globalThis.XMLHttpRequest = MockXHR as unknown as typeof XMLHttpRequest;

	const progress: TusProgress[] = [];
	const phases: string[] = [];
	const snapshots: TusSessionSnapshot[] = [];

	const session = createTusTransfer({
		file: makeFile('abcdefghij'),
		scope: '/api/v1/drive/organization',
		parentId: 'folder-9',
		onProgress: (p) => progress.push(p),
		onPhase: (p) => {
			phases.push(p);
		},
		onSession: (s) => {
			snapshots.push(s);
		}
	});

	const result: TusRunResult = await session.run();
	expect(result.ok).toBe(true);
	expect(result.ok === true && result.item.id).toBe('item-1');
	expect(fetchUrls).toHaveLength(1);
	expect(fetchUrls[0]).toContain('/api/v1/drive/organization/upload-tus?');
	expect(encodeTusMetadata({ parent_id: 'folder-9' })).not.toBe('');
	expect(MockXHR.sent).toHaveLength(2);
	expect(MockXHR.sent[0].headers['Upload-Offset']).toBe('0');
	expect(MockXHR.sent[0].url).toContain('upload-tus/TUSID1');
	expect(MockXHR.sent[1].headers['Upload-Offset']).toBe('4');
	expect(snapshots[0].uploadUrl).toContain('upload-tus/TUSID1');
	// Persistable: snapshot must survive a JSON round trip.
	const persisted = JSON.parse(JSON.stringify(snapshots.at(-1))) as TusSessionSnapshot;
	expect(persisted.state).toBe('done');
	expect(phases).toContain('uploading');
	expect(phases).toContain('committing');
	expect(progress.at(-1)?.percent).toBe(100);
	expect(session.snapshot().state).toBe('done');
});

test('network interruption recovers the server offset via HEAD and resumes from it', async () => {
	globalThis.fetch = (async (_url: string | URL, init?: RequestInit) => {
		if (init?.method === 'HEAD') {
			return new Response(null, { status: 200, headers: { 'Upload-Offset': '4' } });
		}
		return createdResponse('/api/v1/drive/my-storage/upload-tus/TUSID2');
	}) as unknown as typeof fetch;

	MockXHR.queue = [
		{ status: 0 },
		{ status: 204, headers: { 'Upload-Offset': '10', 'Upload-Item-Id': 'item-2' } }
	];
	MockXHR.sent = [];
	globalThis.XMLHttpRequest = MockXHR as unknown as typeof XMLHttpRequest;

	const session = createTusTransfer({
		file: makeFile('abcdefghij'),
		scope: '/api/v1/drive/my-storage'
	});
	const result = await session.run();
	expect(result.ok).toBe(true);
	expect(result.ok === true && result.item.id).toBe('item-2');
	expect(MockXHR.sent[0].headers['Upload-Offset']).toBe('0');
	expect(MockXHR.sent[1].headers['Upload-Offset']).toBe('4');
});

test('lost final response: HEAD reports a finished upload, helper adopts the recorded item', async () => {
	globalThis.fetch = (async (_url: string | URL, init?: RequestInit) => {
		if (init?.method === 'HEAD') {
			return new Response(null, {
				status: 200,
				headers: { 'Upload-Offset': '10', 'Upload-Item-Id': 'stored-item' }
			});
		}
		return createdResponse('/api/v1/drive/my-storage/upload-tus/TUSID3');
	}) as unknown as typeof fetch;

	MockXHR.queue = [{ status: 0 }];
	MockXHR.sent = [];
	globalThis.XMLHttpRequest = MockXHR as unknown as typeof XMLHttpRequest;

	const session = createTusTransfer({
		file: makeFile('abcdefghij'),
		scope: '/api/v1/drive/my-storage'
	});
	const result = await session.run();
	expect(result.ok).toBe(true);
	expect(result.ok === true && result.item.id).toBe('stored-item');
	expect(MockXHR.sent).toHaveLength(1); // no duplicate final chunk
	expect(session.snapshot().state).toBe('done');
});

test('saved snapshot resume: verifies server offset via HEAD, skips creation', async () => {
	const saved: TusSessionSnapshot = {
		scope: '/api/v1/drive/my-storage',
		filename: 'u.txt',
		size: 10,
		parentId: null,
		uploadUrl: '/api/v1/drive/my-storage/upload-tus/TUSID9',
		offset: 4,
		itemId: null,
		state: 'paused',
		error: null,
		createdAt: '2026-01-01T00:00:00.000Z',
		updatedAt: '2026-01-01T00:00:00.000Z'
	};

	const fetchCalls: Array<{ url: string; method?: string }> = [];
	globalThis.fetch = (async (url: string | URL, init?: RequestInit) => {
		fetchCalls.push({ url: String(url), method: init?.method });
		if (init?.method === 'HEAD') {
			return new Response(null, {
				status: 200,
				headers: { 'Upload-Offset': '10', 'Upload-Item-Id': 'stored-item-9' }
			});
		}
		throw new Error('no POST expected when resuming an existing session');
	}) as unknown as typeof fetch;

	globalThis.XMLHttpRequest = MockXHR as unknown as typeof XMLHttpRequest;
	MockXHR.queue = [];
	MockXHR.sent = [];

	const session = createTusTransfer({
		file: makeFile('abcdefghij'),
		scope: '/api/v1/drive/my-storage',
		resume: saved
	});
	const result = await session.run();
	expect(result.ok).toBe(true);
	expect(result.ok === true && result.item.id).toBe('stored-item-9');
	expect(fetchCalls.map((call) => call.method)).toEqual(['HEAD']);
	expect(session.snapshot().state).toBe('done');
	expect(session.snapshot().offset).toBe(10);
});

test('saved snapshot resume with absent server session fails deterministically', async () => {
	const saved: TusSessionSnapshot = {
		scope: '/api/v1/drive/my-storage',
		filename: 'u.txt',
		size: 10,
		parentId: null,
		uploadUrl: '/api/v1/drive/my-storage/upload-tus/GONE',
		offset: 0,
		itemId: null,
		state: 'paused',
		error: null,
		createdAt: '2026-01-01T00:00:00.000Z',
		updatedAt: '2026-01-01T00:00:00.000Z'
	};
	globalThis.fetch = (async () => new Response(null, { status: 404 })) as unknown as typeof fetch;
	globalThis.XMLHttpRequest = MockXHR as unknown as typeof XMLHttpRequest;
	MockXHR.queue = [];
	MockXHR.sent = [];

	const session = createTusTransfer({
		file: makeFile('abcdefghij'),
		scope: '/api/v1/drive/my-storage',
		resume: saved
	});
	const result = await session.run();
	expect(result.ok).toBe(false);
	expect(result.ok === false && result.reason).toBe('failed');
	expect(session.snapshot().state).toBe('failed');
});

test('pause via AbortSignal keeps the server session and returns a recoverable snapshot', async () => {
	let deleteCalled = false;
	globalThis.fetch = (async (url: string | URL, init?: RequestInit) => {
		if (String(url).includes('upload-tus')) {
			if (init?.method === 'DELETE') {
				deleteCalled = true;
				return new Response(null, { status: 204 });
			}
			return createdResponse('/api/v1/drive/my-storage/upload-tus/TUSID5');
		}
		throw new Error('unexpected fetch');
	}) as unknown as typeof fetch;
	MockXHR.queue = [{ status: 0, pending: true }]; // PATCH stays in flight
	MockXHR.sent = [];
	globalThis.XMLHttpRequest = MockXHR as unknown as typeof XMLHttpRequest;

	const controller = new AbortController();
	const session = createTusTransfer({
		file: makeFile('abcdefghij'),
		scope: '/api/v1/drive/my-storage',
		signal: controller.signal
	});
	const running = session.run();
	await Bun.sleep(20); // let the PATCH start
	controller.abort();
	const result = await running;
	expect(result.ok === false && result.reason).toBe('uncertain'); // final PATCH may have reached the server before pause
	const saved = result.ok === false ? result.snapshot : null;
	expect(saved?.state).toBe('paused');
	expect(saved?.uploadUrl).toContain('TUSID5');
	expect(deleteCalled).toBe(false); // signal pauses only; no server DELETE

	// Explicit cancel terminates the server session.
	const cancelled = await session.cancel();
	expect(cancelled).toBe(true);
	expect(session.snapshot().state).toBe('cancelled');
	expect(deleteCalled).toBe(true);
});

test('4xx rejections surface status and code, marking the session failed', async () => {
	globalThis.fetch = (async () =>
		new Response(JSON.stringify({ error: 'USER_QUOTA_EXCEEDED' }), {
			status: 413
		})) as unknown as typeof fetch;
	const session = createTusTransfer({
		file: makeFile('abc'),
		scope: '/api/v1/drive/my-storage'
	});
	const result = await session.run();
	expect(result.ok).toBe(false);
	expect(result.ok === false && result.reason).toBe('failed');
	expect(session.snapshot().state).toBe('failed');
	expect(session.snapshot().error).toBe('USER_QUOTA_EXCEEDED');
	expect(decodeTusItem(null)).toBeNull();
});
