import { afterEach, expect, test } from 'bun:test';
import {
	cancelMultipartSession,
	transferMultipartSession,
	type MultipartSessionSnapshot,
	type MultipartTransferCallbacks
} from './multipart-transfer.js';

const originalFetch = globalThis.fetch;

afterEach(() => {
	globalThis.fetch = originalFetch;
});

function snapshotFor(
	parts: { n: number; status: 'pending' | 'completed'; etag?: string }[]
): MultipartSessionSnapshot {
	return {
		scope: 'my-storage',
		base: '/api/v1/drive/my-storage',
		upload_id: 'mp1',
		filename: 'big.bin',
		size_bytes: parts.length * 10,
		part_size_bytes: 10,
		part_count: parts.length,
		parent_id: null,
		unique_name: true,
		created_at: '2026-10-21T00:00:00Z',
		status: 'active',
		parts: parts.map((p) => ({ part_number: p.n, size_bytes: 10, status: p.status, etag: p.etag }))
	};
}

interface Harness {
	presignedCalls: number[][];
	attemptedParts: number[];
	maxConcurrent: number;
	checkpoints: MultipartSessionSnapshot[];
	completeCalled: number;
}

/** fetch mock covering the three multipart endpoints with per-test knobs. */
function watchFetch(
	harness: Harness,
	options: {
		missingParts: number[] | null;
		completeStatus: number[];
		expired?: boolean;
		urlPrefix?: string;
	}
): void {
	let completionRound = 0;
	globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
		const url = String(input instanceof Request ? input.url : input);
		if (url.includes('/parts/urls')) {
			if (options.expired) {
				return new Response(JSON.stringify({ error: 'multipart_session_expired' }), {
					status: 410
				});
			}
			const requested = (JSON.parse(String(init?.body)) as { part_numbers: number[] }).part_numbers;
			harness.presignedCalls.push(requested);
			const body = {
				parts: requested.map((n) => ({
					part_number: n,
					url: `${options.urlPrefix ?? 'https://provider'}/part/${n}`,
					expires_at: 'x'
				}))
			};
			return new Response(JSON.stringify(body), { status: 200 });
		}
		if (url.includes('/complete')) {
			harness.completeCalled++;
			const status = options.completeStatus[completionRound++] ?? 201;
			if (status >= 400) {
				return new Response(
					JSON.stringify({ error: 'multipart_parts_incomplete', parts: options.missingParts }),
					{ status }
				);
			}
			return new Response(
				JSON.stringify({
					id: 'item-1',
					name: 'big.bin',
					kind: 'file',
					size_bytes: 40,
					parent_id: null,
					modified_at: '2026-10-21T01:00:00Z'
				}),
				{ status: 201 }
			);
		}
		harness.completeCalled = -999;
		return new Response('{}', { status: 404 });
	}) as unknown as typeof fetch;
}

function makeCallbacks(harness: Harness): MultipartTransferCallbacks {
	return {
		checkpoint: (snapshot) => {
			expect(snapshot.parts.every((part) => !Object.hasOwn(part, 'url'))).toBe(true);
			harness.checkpoints.push(snapshot);
		}
	};
}

function trackingTransport(harness: Harness, onPartStarted?: (n: number) => void) {
	return async (
		part: { part_number: number; size_bytes: number },
		url: string
	): Promise<string> => {
		if (!url.includes('/part/'))
			throw new Error(`transport must receive the presigned URL, got ${url}`);
		harness.attemptedParts.push(part.part_number);
		onPartStarted?.(part.part_number);
		return `e-${part.part_number}`;
	};
}

test('multipart transport uploads only absent parts, checkpoints etags, and completes server-side', async () => {
	const harness: Harness = {
		presignedCalls: [],
		attemptedParts: [],
		maxConcurrent: 0,
		checkpoints: [],
		completeCalled: 0
	};
	watchFetch(harness, { missingParts: null, completeStatus: [201] });
	let active = 0;
	const transport = async (part: { part_number: number }, url: string): Promise<string> => {
		active++;
		harness.maxConcurrent = Math.max(harness.maxConcurrent, active);
		await new Promise((resolve) => setTimeout(resolve, 5));
		const etag = `e-${part.part_number}`;
		active--;
		void url;
		harness.attemptedParts.push(part.part_number);
		return etag;
	};

	const file = new Blob([new Uint8Array(40).fill(1)]);
	const session = snapshotFor([
		{ n: 1, status: 'completed', etag: 'e-1' },
		{ n: 2, status: 'pending' },
		{ n: 3, status: 'pending' },
		{ n: 4, status: 'pending' }
	]);
	let lastProgress = -1;
	const { session: settled, item } = await transferMultipartSession(
		file,
		session,
		{
			checkpoint: (snapshot) => {
				expect(snapshot.parts.every((part) => !Object.hasOwn(part, 'url'))).toBe(true);
				harness.checkpoints.push(snapshot);
			},
			progress: (loaded, total) => {
				expect(loaded).toBeLessThanOrEqual(total);
				lastProgress = loaded;
			}
		},
		{ partTransport: transport }
	);

	expect(harness.presignedCalls).toEqual([[2, 3, 4]]);
	expect(harness.attemptedParts).toEqual([2, 3, 4]);
	expect(item.id).toBe('item-1');
	expect(settled.status).toBe('completed');
	expect(settled.final_item_id).toBe('item-1');
	expect(settled.parts.map((p) => ({ n: p.part_number, s: p.status, e: p.etag }))).toEqual([
		{ n: 1, s: 'completed', e: 'e-1' },
		{ n: 2, s: 'completed', e: 'e-2' },
		{ n: 3, s: 'completed', e: 'e-3' },
		{ n: 4, s: 'completed', e: 'e-4' }
	]);
	expect(lastProgress).toBe(40);
	expect(settled.parts.every((part) => !Object.hasOwn(part, 'url'))).toBe(true);
});

test('parts upload with bounded concurrency of three', async () => {
	const harness: Harness = {
		presignedCalls: [],
		attemptedParts: [],
		maxConcurrent: 0,
		checkpoints: [],
		completeCalled: 0
	};
	watchFetch(harness, { missingParts: null, completeStatus: [201] });
	let active = 0;
	const transport = async (part: { part_number: number }, url: string): Promise<string> => {
		if (!url.includes('/part/'))
			throw new Error(`transport must receive the presigned URL, got ${url}`);
		harness.attemptedParts.push(part.part_number);
		active++;
		if (active > harness.maxConcurrent) harness.maxConcurrent = active;
		await Promise.resolve();
		active--;
		return `e-${part.part_number}`;
	};
	const session = snapshotFor(
		Array.from({ length: 10 }, (_, i) => ({ n: i + 1, status: 'pending' as const }))
	);
	await transferMultipartSession(
		new Blob([new Uint8Array(100).fill(2)]),
		session,
		makeCallbacks(harness),
		{ partTransport: transport }
	);
	expect(harness.maxConcurrent).toBe(3);
	expect(harness.attemptedParts.length).toBe(10);
});

test('parts_incomplete reconciliation re-requests only the missing part', async () => {
	const harness: Harness = {
		presignedCalls: [],
		attemptedParts: [],
		maxConcurrent: 0,
		checkpoints: [],
		completeCalled: 0
	};
	watchFetch(harness, { missingParts: [2], completeStatus: [409, 201] });
	const transport = trackingTransport(harness);
	const session = snapshotFor([
		{ n: 1, status: 'pending' },
		{ n: 2, status: 'pending' },
		{ n: 3, status: 'pending' },
		{ n: 4, status: 'pending' }
	]);
	const { item } = await transferMultipartSession(
		new Blob([new Uint8Array(40).fill(3)]),
		session,
		makeCallbacks(harness),
		{ partTransport: transport }
	);
	expect(item.id).toBe('item-1');
	expect(harness.presignedCalls).toEqual([[1, 2, 3, 4], [2]]);
	expect(harness.completeCalled).toBe(2);
});

test('expired session fails explicitly and never restarts the upload', async () => {
	const harness: Harness = {
		presignedCalls: [],
		attemptedParts: [],
		maxConcurrent: 0,
		checkpoints: [],
		completeCalled: 0
	};
	watchFetch(harness, { missingParts: null, completeStatus: [201], expired: true });
	const session = snapshotFor([
		{ n: 1, status: 'pending' },
		{ n: 2, status: 'pending' }
	]);
	let error: { code?: string } | undefined;
	await transferMultipartSession(
		new Blob([new Uint8Array(20).fill(4)]),
		session,
		makeCallbacks(harness),
		{ partTransport: trackingTransport(harness) }
	).catch((thrown) => {
		error = thrown;
	});
	expect(error?.code).toBe('multipart_session_expired');
	expect(harness.attemptedParts).toEqual([]);
	expect(harness.completeCalled).toBe(0);
});

test('an aborted signal stops further parts without checkpointing a completed snapshot', async () => {
	const harness: Harness = {
		presignedCalls: [],
		attemptedParts: [],
		maxConcurrent: 0,
		checkpoints: [],
		completeCalled: 0
	};
	watchFetch(harness, { missingParts: null, completeStatus: [201] });
	const controller = new AbortController();
	const transport = async (): Promise<string> => {
		harness.attemptedParts.push(harness.attemptedParts.length);
		controller.abort();
		return 'e-1';
	};
	const session = snapshotFor([
		{ n: 1, status: 'pending' },
		{ n: 2, status: 'pending' }
	]);
	let error: unknown = null;
	try {
		await transferMultipartSession(
			new Blob([new Uint8Array(20).fill(5)]),
			session,
			makeCallbacks(harness),
			{ partTransport: transport, signal: controller.signal }
		);
	} catch (thrown) {
		error = thrown;
	}
	expect(error).not.toBeNull();
	expect(harness.completeCalled).toBe(0);
});

test('cancelMultipartSession issues the explicit DELETE', async () => {
	let method = '';
	let path = '';
	globalThis.fetch = (async (input: RequestInfo | URL, init?: RequestInit) => {
		path = String(input instanceof Request ? input.url : input);
		method = init?.method ?? 'GET';
		return new Response(null, { status: 204 });
	}) as unknown as typeof fetch;
	await cancelMultipartSession(snapshotFor([{ n: 1, status: 'pending' }]));
	expect(method).toBe('DELETE');
	expect(path).toBe('/api/v1/drive/my-storage/multipart/mp1');
});
test('an expired part URL is renewed once without restarting successful parts', async () => {
	const harness: Harness = {
		presignedCalls: [],
		attemptedParts: [],
		maxConcurrent: 0,
		checkpoints: [],
		completeCalled: 0
	};
	watchFetch(harness, { missingParts: null, completeStatus: [201] });
	let expireFirstPartURL = true;
	const transport = async (part: { part_number: number }): Promise<string> => {
		harness.attemptedParts.push(part.part_number);
		if (part.part_number === 1 && expireFirstPartURL) {
			expireFirstPartURL = false;
			throw Object.assign(new Error('expired presigned URL'), { status: 403 });
		}
		return `e-${part.part_number}`;
	};

	const { item } = await transferMultipartSession(
		new Blob([new Uint8Array(20).fill(6)]),
		snapshotFor([
			{ n: 1, status: 'pending' },
			{ n: 2, status: 'pending' }
		]),
		makeCallbacks(harness),
		{ partTransport: transport }
	);

	expect(item.id).toBe('item-1');
	expect(harness.presignedCalls).toEqual([[1, 2], [1]]);
	expect(harness.attemptedParts.filter((part) => part === 1)).toHaveLength(2);
	expect(harness.attemptedParts.filter((part) => part === 2)).toHaveLength(1);
});
