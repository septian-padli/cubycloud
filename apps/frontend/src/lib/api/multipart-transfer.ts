/**
 * Direct presigned multipart transport (browser → provider UploadPart).
 *
 * The server owns the durable intent (24h), part planning, quota/capacity
 * enforcement, ListParts reconciliation and the final Complete call. This
 * module only moves raw file bytes: it asks the server for short-lived part
 * URLs, PUTs each part straight to the provider, and checkpoints intent/part
 * state through caller-supplied callbacks (the caller decides what to persist).
 *
 * Snapshot contract (what the caller may safely keep in IndexedDB): upload ids,
 * filename, size, part planning, part ETags and statuses. Provider secrets and
 * expiring part URLs are never part of the snapshot — URLs are minted per call
 * here and discarded after use.
 */

const MULTI_PART_CONCURRENCY = 3;
const MAX_PART_ATTEMPTS = 3;
const RETRY_DELAYS_MS = [500, 1500, 4500];
const DEFAULT_URL_BATCH_SIZE = 32;

export type MultipartScope = 'my-storage' | 'organization';

export interface MultipartPartState {
	part_number: number;
	size_bytes: number;
	etag?: string;
	status: 'pending' | 'completed' | 'failed';
}

export interface MultipartSessionSnapshot {
	scope: MultipartScope;
	base: string;
	upload_id: string;
	filename: string;
	size_bytes: number;
	part_size_bytes: number;
	part_count: number;
	parent_id: string | null;
	unique_name: boolean;
	created_at: string;
	expires_at?: string;
	status: 'active' | 'completed';
	final_item_id?: string;
	parts: MultipartPartState[];
}

export interface MultipartItem {
	id: string;
	name: string;
	kind: 'file' | 'folder';
	size_bytes: number;
	parent_id: string | null;
	modified_at: string;
}

export interface MultipartTransferCallbacks {
	/** Persist the snapshot whenever intent/part state changes. */
	checkpoint: (snapshot: MultipartSessionSnapshot) => void | Promise<void>;
	onPhase?: (phase: 'uploading' | 'committing') => void | Promise<void>;
	/** Cumulative uploaded bytes across all parts. */
	progress?: (loadedBytes: number, totalBytes: number) => void;
}

export interface MultipartTransferOptions {
	signal?: AbortSignal;
	/** Injectable part transport for tests; the default XHR does not carry app cookies. */
	partTransport?: MultipartPartTransport;
	urlBatchSize?: number;
	concurrency?: number;
}

export type MultipartPartTransport = (
	part: MultipartPartState,
	url: string,
	blob: Blob,
	post: (percent: number, partNumber: number) => void,
	signal?: AbortSignal
) => Promise<string>;

export class MultipartError extends Error {
	code: string;
	status?: number;
	missingParts?: number[];

	constructor(message: string, code: string, status?: number, missingParts?: number[]) {
		super(message);
		this.name = 'MultipartError';
		this.code = code;
		this.status = status;
		this.missingParts = missingParts;
	}
}

/** Expired or otherwise unrecoverable: never silently restart; rebuild explicitly. */
export class MultipartSessionGoneError extends MultipartError {
	constructor(code: string, status: number) {
		super(
			code === 'multipart_session_expired'
				? 'Sesi upload kedaluwarsa. Mulai ulang upload berkas ini.'
				: 'Sesi upload tidak dapat dilanjutkan.',
			code,
			status
		);
	}
}

function multipartErrorMessage(body: { error?: string } | undefined, status: number): string {
	if (body?.error === 'USER_QUOTA_EXCEEDED') return 'Kuota penyimpanan penuh. Hubungi admin.';
	if (body?.error === 'STORAGE_POOL_FULL') return 'Penyimpanan server penuh.';
	if (body?.error === 'multipart_session_expired')
		return 'Sesi upload kedaluwarsa. Mulai ulang upload berkas ini.';
	if (body?.error === 'multipart_unrecoverable')
		return 'Sesi upload tidak dapat dilanjutkan. Mulai ulang upload berkas ini.';
	if (body?.error === 'multipart_parts_incomplete')
		return 'Sebagian bagian berkas belum terunggah.';
	return body?.error || `HTTP ${status}`;
}

async function multipartRequest<T>(path: string, init?: RequestInit): Promise<T> {
	const response = await fetch(path, { credentials: 'include', ...init });
	const text = await response.text();
	const body = text ? (JSON.parse(text) as { error?: string; parts?: PresignedPart[] }) : undefined;
	if (!response.ok) {
		const code = body?.error;
		if (code === 'multipart_session_expired' || code === 'multipart_unrecoverable') {
			throw new MultipartSessionGoneError(code, response.status);
		}
		if (code === 'multipart_parts_incomplete') {
			const missing =
				((body as unknown as { parts?: unknown })?.parts as number[] | undefined) ?? [];
			throw new MultipartError(
				multipartErrorMessage(body, response.status),
				code,
				response.status,
				missing
			);
		}
		throw new MultipartError(
			multipartErrorMessage(body, response.status),
			code ?? 'request_failed',
			response.status
		);
	}
	return body as T;
}

interface PresignedPart {
	part_number: number;
	url: string;
	expires_at: string;
}

function cloneSession(session: MultipartSessionSnapshot): MultipartSessionSnapshot {
	return { ...session, parts: session.parts.map((part) => ({ ...part })) };
}

function isRetryableStatus(status: number | undefined): boolean {
	return status === undefined || status === 0 || status >= 500 || status === 429;
}

function delay(ms: number, signal?: AbortSignal): Promise<void> {
	return new Promise((resolve, reject) => {
		const timer = setTimeout(resolve, ms);
		signal?.addEventListener(
			'abort',
			() => {
				clearTimeout(timer);
				reject(signal.reason ?? new DOMException('Aborted', 'AbortError'));
			},
			{ once: true }
		);
	});
}

export function defaultPartTransport(
	part: MultipartPartState,
	url: string,
	blob: Blob,
	post: (percent: number, partNumber: number) => void,
	signal?: AbortSignal
): Promise<string> {
	const { promise, resolve, reject } = Promise.withResolvers<string>();
	const xhr = new XMLHttpRequest();
	xhr.open('PUT', url);
	xhr.upload.onprogress = (event) => {
		if (event.lengthComputable)
			post(Math.round((event.loaded / event.total) * 100), part.part_number);
	};
	xhr.onabort = () => reject(signal?.reason ?? new DOMException('Aborted', 'AbortError'));
	xhr.onerror = () => reject(new MultipartError('Koneksi ke cloud storage gagal', 'network', 0));
	xhr.onload = () => {
		const etag = xhr.getResponseHeader('ETag');
		if (xhr.status >= 200 && xhr.status < 300 && etag) {
			post(100, part.part_number);
			resolve(etag);
		} else if (xhr.status >= 200 && xhr.status < 300) {
			reject(
				new MultipartError(
					`Respons upload part ${part.part_number} tanpa ETag`,
					'missing_etag',
					xhr.status
				)
			);
		} else {
			reject(
				new MultipartError(
					`Upload part ${part.part_number} gagal (HTTP ${xhr.status})`,
					'part_transport',
					xhr.status
				)
			);
		}
	};
	signal?.addEventListener('abort', () => xhr.abort(), { once: true });
	xhr.send(blob);
	return promise;
}

async function presignParts(
	session: MultipartSessionSnapshot,
	partNumbers: number[],
	urlBatchSize: number
): Promise<Map<number, string>> {
	const urls = new Map<number, string>();
	for (let start = 0; start < partNumbers.length; start += urlBatchSize) {
		const batch = partNumbers.slice(start, start + urlBatchSize);
		const body = await multipartRequest<{ parts: PresignedPart[] }>(
			`${session.base}/multipart/${encodeURIComponent(session.upload_id)}/parts/urls`,
			{
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ part_numbers: batch })
			}
		);
		for (const entry of body.parts ?? []) urls.set(entry.part_number, entry.url);
		for (const number of batch) {
			if (!urls.has(number)) {
				throw new MultipartError(
					`Server tidak mengirim URL untuk part ${number}`,
					'presign_missing',
					200
				);
			}
		}
	}
	return urls;
}

/**
 * PUT one raw slice with bounded retries; a 403 only means the short-lived URL
 * lapsed, so the URL is renewed once before it counts as a failure.
 */
async function uploadOnePart(
	file: Blob,
	working: MultipartSessionSnapshot,
	part: MultipartPartState,
	url: string,
	partTransport: MultipartPartTransport,
	signal: AbortSignal | undefined,
	urlBatchSize: number,
	onPartBytes: (deltaBytes: number) => void
): Promise<void> {
	const offset = (part.part_number - 1) * working.part_size_bytes;
	const blob = file.slice(offset, offset + part.size_bytes);
	let activeURL = url;
	let renewed = false;
	for (let attempt = 0; ; attempt++) {
		const uploaded = { bytes: 0 };
		try {
			const etag = await partTransport(
				part,
				activeURL,
				blob,
				(percent) => {
					const reached = Math.round((percent / 100) * part.size_bytes);
					if (reached > uploaded.bytes) {
						onPartBytes(reached - uploaded.bytes);
						uploaded.bytes = reached;
					}
				},
				signal
			);
			if (uploaded.bytes < part.size_bytes) onPartBytes(part.size_bytes - uploaded.bytes);
			part.status = 'completed';
			part.etag = etag;
			return;
		} catch (error: unknown) {
			const failure = error as MultipartError;
			if (signal?.aborted) {
				onPartBytes(-uploaded.bytes);
				throw error;
			}
			if (failure.status === 403 && !renewed) {
				renewed = true;
				const renewedURLs = await presignParts(working, [part.part_number], urlBatchSize);
				const fresh = renewedURLs.get(part.part_number);
				if (fresh) {
					activeURL = fresh;
					attempt--;
					continue;
				}
			}
			if (!isRetryableStatus(failure.status) || attempt >= MAX_PART_ATTEMPTS) {
				onPartBytes(-uploaded.bytes);
				part.status = 'failed';
				delete part.etag;
				throw error;
			}
			await delay(RETRY_DELAYS_MS[Math.min(attempt, RETRY_DELAYS_MS.length - 1)], signal);
		}
	}
}

export async function transferMultipartSession(
	file: Blob,
	session: MultipartSessionSnapshot,
	callbacks: MultipartTransferCallbacks,
	options: MultipartTransferOptions = {}
): Promise<{ session: MultipartSessionSnapshot; item: MultipartItem }> {
	const partTransport = options.partTransport ?? defaultPartTransport;
	const signal = options.signal;
	const urlBatchSize = options.urlBatchSize ?? DEFAULT_URL_BATCH_SIZE;
	const concurrency = options.concurrency ?? MULTI_PART_CONCURRENCY;

	const working = cloneSession(session);
	let loaded = working.parts
		.filter((part) => part.status === 'completed')
		.reduce((sum, part) => sum + part.size_bytes, 0);
	const onPartBytes = (deltaBytes: number) => {
		loaded += deltaBytes;
		callbacks.progress?.(loaded, working.size_bytes);
	};
	callbacks.progress?.(loaded, working.size_bytes);

	for (let reconcile = 1; ; reconcile++) {
		if (signal?.aborted) throw signal.reason ?? new DOMException('Aborted', 'AbortError');
		const pending = working.parts
			.filter((part) => part.status !== 'completed')
			.sort((a, b) => a.part_number - b.part_number);
		if (pending.length > 0) {
			await callbacks.onPhase?.('uploading');
			const urls = await presignParts(
				working,
				pending.map((part) => part.part_number),
				urlBatchSize
			);
			const queue = [...pending];
			const workers: Promise<void>[] = [];
			for (let slot = 0; slot < concurrency && queue.length > 0; slot++) {
				workers.push(
					(async () => {
						while (queue.length > 0) {
							if (signal?.aborted) throw signal.reason ?? new DOMException('Aborted', 'AbortError');
							const part = queue.shift()!;
							const url = urls.get(part.part_number);
							if (!url) {
								part.status = 'failed';
								delete part.etag;
								throw new MultipartError(
									`Tidak ada URL untuk part ${part.part_number}`,
									'presign_missing',
									200
								);
							}
							await uploadOnePart(
								file,
								working,
								part,
								url,
								partTransport,
								signal,
								urlBatchSize,
								onPartBytes
							);
							await callbacks.checkpoint(cloneSession(working));
						}
					})()
				);
			}
			const outcomes = await Promise.allSettled(workers);
			const rejected = outcomes.find((entry) => entry.status === 'rejected');
			if (rejected && rejected.status === 'rejected') {
				throw rejected.reason;
			}
		}
		try {
			await callbacks.onPhase?.('committing');
			const item = await multipartRequest<MultipartItem>(
				`${working.base}/multipart/${encodeURIComponent(working.upload_id)}/complete`,
				{ method: 'POST' }
			);
			working.status = 'completed';
			working.final_item_id = item?.id;
			await callbacks.checkpoint(cloneSession(working));
			return { session: working, item };
		} catch (error: unknown) {
			if (signal?.aborted) throw signal.reason ?? new DOMException('Aborted', 'AbortError');
			if (
				error instanceof MultipartError &&
				error.code === 'multipart_parts_incomplete' &&
				reconcile < 3
			) {
				// The server reconciled provider truth and listed missing
				// parts: resume exactly those (never a silent full restart).
				for (const partNumber of error.missingParts ?? []) {
					const part = working.parts.find((entry) => entry.part_number === partNumber);
					if (part) {
						if (part.status === 'completed') onPartBytes(-part.size_bytes);
						part.status = 'pending';
						delete part.etag;
					}
				}
				await callbacks.checkpoint(cloneSession(working));
				continue;
			}
			throw error;
		}
	}
}

/** Explicitly terminate a durable session server-side (provider parts are dropped). */
export async function cancelMultipartSession(session: MultipartSessionSnapshot): Promise<void> {
	await fetch(`${session.base}/multipart/${encodeURIComponent(session.upload_id)}`, {
		method: 'DELETE',
		credentials: 'include'
	});
}
