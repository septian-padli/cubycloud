import { locale } from '$lib/locale.svelte.js';
/**
 * Standalone TUS transport for resumable uploads against a single scope
 * (`/api/v1/drive/my-storage` or `/api/v1/drive/organization`).
 *
 * Owns only the wire protocol (POST creation, HEAD offset recovery, PATCH
 * chunks, DELETE termination). The caller supplies the File handle plus an
 * optional previously persisted session snapshot, receives progress/phase/
 * session callbacks, and pauses or cancels via AbortSignal and explicit
 * methods. The exported snapshot is plain JSON (backend session identity only:
 * upload URL, offset, item id, state) so the caller may persist it — bytes and
 * File handles must stay with the caller.
 */

export type TusScope = '/api/v1/drive/my-storage' | '/api/v1/drive/organization';

export type TusTransferState =
	| 'creating'
	| 'verifying'
	| 'uploading'
	| 'committing'
	| 'done'
	| 'paused'
	| 'cancelled'
	| 'failed';

export type TusPhase = 'verifying' | 'uploading' | 'committing';

/** Serializable session identity: safe to persist (no File, no bytes, no credentials). */
export interface TusSessionSnapshot {
	scope: TusScope;
	filename: string;
	size: number;
	parentId: string | null;
	uploadUrl: string | null;
	offset: number;
	itemId: string | null;
	state: TusTransferState;
	error: string | null;
	createdAt: string;
	updatedAt: string;
}

export interface TusProgress {
	loaded: number;
	total: number;
	percent: number;
}

/** Finalized item as recorded by the backend (`Upload-Item` JSON). */
export interface TusStorageItem {
	id: string;
	name?: string;
	kind?: string;
	size_bytes?: number;
	[key: string]: unknown;
}

export type TusRunResult =
	| { ok: true; item: TusStorageItem; snapshot: TusSessionSnapshot }
	/** `paused`/`uncertain` snapshots are recoverable by calling run() again with `resume`. */
	| { ok: false; reason: 'paused' | 'uncertain' | 'failed'; snapshot: TusSessionSnapshot };

export interface TusTransferOptions {
	file: File;
	scope: TusScope;
	parentId?: string | null;
	/** Expected full-file digest for the TUS checksum extension (base64 or hex, 32 bytes). */
	checksum?: string;
	chunkSize?: number;
	maxRetries?: number;
	/**
	 * Aborting pauses the in-flight network operation WITHOUT deleting the
	 * server session; run() resolves with a recoverable snapshot. Use
	 * session.cancel() to terminate the server session explicitly.
	 */
	signal?: AbortSignal;
	onProgress?: (progress: TusProgress) => void;
	onPhase?: (phase: TusPhase) => void | Promise<void>;
	/** Alias of onSession emitted on every state change. */
	onSession?: (snapshot: TusSessionSnapshot) => void | Promise<void>;
	/** Resume from a previously persisted TuSessionSnapshot (same File). */
	resume?: Pick<
		TusSessionSnapshot,
		'scope' | 'filename' | 'size' | 'uploadUrl' | 'offset' | 'itemId' | 'state'
	> &
		Partial<TusSessionSnapshot>;
}

export const DEFAULT_TUS_CHUNK_SIZE = 2 * 1024 * 1024;
const TUS_VERSION = '1.0.0';

/** Thrown by run() only when the session resumption state is inconsistent with the provided File. */
export class TusResumeError extends Error {
	constructor(
		message = locale.language === 'id'
			? 'Snapshot sesi upload tidak cocok dengan berkas yang dipilih.'
			: 'The saved upload session does not match the selected file.'
	) {
		super(message);
		this.name = 'TusResumeError';
	}
}

export class TusPauseError extends Error {
	constructor(message = locale.language === 'id' ? 'Upload dijeda.' : 'Upload paused.') {
		super(message);
		this.name = 'TusPauseError';
	}
}

/** Encode TUS creation metadata (comma-separated base64 pairs). */
export function encodeTusMetadata(meta: Record<string, string | undefined | null>): string {
	return Object.entries(meta)
		.filter(([, value]) => value !== undefined && value !== null && value !== '')
		.map(([key, value]) => `${key} ${btoa(unescape(encodeURIComponent(value as string)))}`)
		.join(',');
}

/** Decode the Upload-Item header the backend sends on finalization. */
export function decodeTusItem(header: string | null): TusStorageItem | null {
	if (!header) return null;
	try {
		const json = decodeURIComponent(escape(atob(header)));
		return JSON.parse(json) as TusStorageItem;
	} catch {
		return null;
	}
}

interface RawResponse {
	status: number;
	ok: boolean;
	text: string;
	header(name: string): string | null;
}

export interface TusTransferSession {
	/** Runs the transfer, resuming from `resume` when provided. Never throws operational failures. */
	run(): Promise<TusRunResult>;
	/** Plain-JSON snapshot of the backend session identity (persist-safe). */
	snapshot(): TusSessionSnapshot;
	/** Pauses the current network operation; the server session is kept for later resume. */
	pause(): void;
	/** Terminates the server session (DELETE upload-tus/:id) and marks the upload cancelled. */
	cancel(): Promise<boolean>;
}

export function createTusTransfer(options: TusTransferOptions): TusTransferSession {
	const file = options.file;
	const scope = options.scope;
	const chunkSize = options.chunkSize ?? DEFAULT_TUS_CHUNK_SIZE;
	const maxRetries = options.maxRetries ?? 3;
	const resume = options.resume ?? null;

	// Internal signal: aborting pauses the current network operation only.
	const pauseController = new AbortController();
	const forwardAbort = () => pauseController.abort();
	if (options.signal) {
		if (options.signal.aborted) pauseController.abort();
		else options.signal.addEventListener('abort', forwardAbort, { once: true });
	}

	let snapshot: TusSessionSnapshot = {
		scope,
		filename: file.name,
		size: file.size,
		parentId: options.parentId ?? null,
		uploadUrl: resume?.uploadUrl ?? null,
		offset: resume?.offset ?? 0,
		itemId: resume?.itemId ?? null,
		state:
			resume?.itemId && resume?.state === 'done'
				? 'done'
				: resume?.uploadUrl
					? 'verifying'
					: 'creating',
		error: null,
		createdAt: resume?.createdAt ?? new Date().toISOString(),
		updatedAt: new Date().toISOString()
	};

	let callbackWrites = Promise.resolve();
	let callbackError: unknown;

	const scheduleCallback = (callback?: () => void | Promise<void>) => {
		if (!callback) return;
		callbackWrites = callbackWrites.then(callback).catch((error: unknown) => {
			callbackError ??= error;
		});
	};

	const flushCallbacks = async () => {
		await callbackWrites;
		if (callbackError) throw callbackError;
	};

	const emit = () => {
		snapshot.updatedAt = new Date().toISOString();
		const current = { ...snapshot };
		scheduleCallback(() => options.onSession?.(current));
	};

	const update = (patch: Partial<TusSessionSnapshot>) => {
		snapshot = { ...snapshot, ...patch };
		emit();
	};

	const setPhase = (phase: TusPhase) => {
		scheduleCallback(() => options.onPhase?.(phase));
		update({ state: phase });
	};

	const emitProgress = (offset: number, cap = true) => {
		const total = file.size;
		const percent =
			total === 0 ? 100 : Math.min(cap ? 99 : 100, Math.round((offset / total) * 100));
		options.onProgress?.({ loaded: offset, total, percent });
	};

	const pauseRequested = () => pauseController.signal.aborted;

	const requirementMet = () => {
		if (pauseRequested()) throw new TusPauseError();
	};

	const throttle = (attempt: number) =>
		new Promise<void>((resolve) => {
			setTimeout(resolve, 500 * attempt);
		});

	const encodeChecksum = (value?: string) => (value ? `sha256 ${value}` : '');

	async function createUpload(): Promise<string> {
		requirementMet();
		const qs = new URLSearchParams({ filename: file.name });
		const parentId = options.parentId ?? undefined;
		if (parentId) qs.set('parent_id', parentId);

		const response = await fetch(`${scope}/upload-tus?${qs.toString()}`, {
			method: 'POST',
			headers: {
				'Upload-Length': file.size.toString(),
				'Upload-Metadata': encodeTusMetadata({
					filename: file.name,
					parent_id: parentId,
					checksum: options.checksum
				}),
				'Tus-Resumable': TUS_VERSION
			},
			credentials: 'include'
		});
		if (!response.ok) {
			let message =
				locale.language === 'id'
					? `Gagal membuat sesi upload (HTTP ${response.status})`
					: `Failed to create upload session (HTTP ${response.status})`;
			try {
				const body = (await response.clone().json()) as { error?: unknown };
				if (typeof body.error === 'string' && body.error) message = body.error;
			} catch {
				// Keep the HTTP status message when the response has no JSON error code.
			}
			update({ error: message, state: 'failed' });
			throw new Error(message);
		}
		const location = response.headers.get('Location');
		if (!location) {
			const message =
				locale.language === 'id'
					? 'Header Location tidak ditemukan pada respons TUS'
					: 'Location header not found in TUS response';
			update({ error: message, state: 'failed' });
			throw new Error(message);
		}
		const uploadUrl =
			location.startsWith('http') || location.startsWith('/') ? location : `${scope}/${location}`;
		snapshot.uploadUrl = uploadUrl;
		update({ uploadUrl, state: 'uploading' });
		await flushCallbacks();
		return uploadUrl;
	}

	function patchChunk(uploadUrl: string, offset: number, chunk: Blob): Promise<RawResponse> {
		const { promise, resolve, reject } = Promise.withResolvers<RawResponse>();
		const xhr = new XMLHttpRequest();
		xhr.open('PATCH', uploadUrl);
		xhr.setRequestHeader('Upload-Offset', offset.toString());
		const checksumHeader = encodeChecksum(options.checksum);
		if (checksumHeader) xhr.setRequestHeader('Upload-Checksum', checksumHeader);
		xhr.setRequestHeader('Content-Type', 'application/offset+octet-stream');
		xhr.setRequestHeader('Tus-Resumable', TUS_VERSION);
		xhr.withCredentials = true;
		pauseController.signal.addEventListener('abort', () => xhr.abort(), { once: true });
		xhr.upload.onprogress = (event) => {
			if (event.lengthComputable) {
				options.onProgress?.({
					loaded: offset + event.loaded,
					total: file.size,
					percent:
						file.size === 0
							? 100
							: Math.min(99, Math.round(((offset + event.loaded) / file.size) * 100))
				});
			}
		};
		xhr.onload = () =>
			resolve({
				status: xhr.status,
				ok: xhr.status >= 200 && xhr.status < 300,
				text: xhr.responseText,
				header: (name: string) => xhr.getResponseHeader(name)
			});
		xhr.onerror = () =>
			reject(
				new Error(
					locale.language === 'id'
						? 'Koneksi ke server terputus'
						: 'Connection to the server was lost'
				)
			);
		xhr.onabort = () =>
			reject(
				new DOMException(
					locale.language === 'id' ? 'Upload dijeda.' : 'Upload paused.',
					'AbortError'
				)
			);
		xhr.send(chunk);
		return promise;
	}

	async function recoverOffset(
		uploadUrl: string
	): Promise<{ offset: number; item: TusStorageItem | null } | null> {
		const response = await fetch(uploadUrl, {
			method: 'HEAD',
			headers: { 'Tus-Resumable': TUS_VERSION },
			credentials: 'include'
		});
		if (!response.ok) return null;
		const header = response.headers.get('Upload-Offset');
		const remote = header === null ? NaN : Number(header);
		if (!Number.isInteger(remote) || remote < 0 || remote > file.size) return null;
		const item = decodeTusItem(response.headers.get('Upload-Item'));
		const idHeader = response.headers.get('Upload-Item-Id');
		return { offset: remote, item: item ?? (idHeader ? { id: idHeader } : null) };
	}

	function itemIdFrom(item: TusStorageItem | null, header: string | null): string | null {
		if (item && typeof item.id === 'string') return item.id;
		if (header) return header;
		return null;
	}

	function finish(offset: number, item: TusStorageItem | null, itemId: string | null) {
		const stored = item ?? { id: itemId ?? '' };
		update({ offset, state: 'done', itemId: itemId ?? stored.id });
		options.onProgress?.({ loaded: file.size, total: file.size, percent: 100 });
		return stored;
	}

	function markFailed(message: string): TusRunResult {
		update({ state: 'failed', error: message });
		return { ok: false, reason: 'failed', snapshot };
	}

	function markPaused(phaseBeforePause: TusPhase): TusRunResult {
		const uncertain = phaseBeforePause === 'committing';
		update({ state: 'paused', error: new TusPauseError().message });
		return { ok: false, reason: uncertain ? 'uncertain' : 'paused', snapshot };
	}

	async function run(): Promise<TusRunResult> {
		// A finished snapshot resumes to the recorded item immediately.
		if (snapshot.state === 'done' && snapshot.itemId) {
			return {
				ok: true,
				item: { id: snapshot.itemId },
				snapshot
			};
		}

		let committedItem: TusStorageItem | null = null;
		try {
			requirementMet();
			let uploadUrl: string;

			if (resume?.uploadUrl && file.size === resume.size) {
				// Trust the persisted session identity, then confirm against the
				// server via HEAD before trusting any offset.
				setPhase('verifying');
				uploadUrl = resume.uploadUrl;
				let confirmed = false;
				let attempts = 0;
				while (!confirmed) {
					requirementMet();
					attempts++;
					try {
						const recovery = await recoverOffset(uploadUrl);
						if (!recovery) {
							// Session gone (expired/cleanup/other owner): the caller
							// must start a fresh session; bytes were never persisted.
							update({
								error:
									locale.language === 'id'
										? 'Sesi upload tidak ditemukan di server'
										: 'Upload session not found on the server',
								state: 'failed'
							});
							return { ok: false, reason: 'failed', snapshot };
						}
						if (recovery.offset >= file.size) {
							// Already finalized; adopt the recorded item.
							committedItem = recovery.item;
							const id = itemIdFrom(committedItem, null);
							finish(recovery.offset, committedItem, id);
							return { ok: true, item: committedItem!, snapshot };
						}
						update({ offset: recovery.offset });
						confirmed = true;
					} catch {
						if (attempts >= maxRetries) {
							update({
								error:
									locale.language === 'id'
										? 'Gagal memverifikasi sesi upload'
										: 'Failed to verify upload session',
								state: 'failed'
							});
							return { ok: false, reason: 'failed', snapshot };
						}
						await throttle(attempts);
					}
				}
			} else {
				if (resume && file.size !== resume.size) throw new TusResumeError();
				if (resume?.uploadUrl) snapshot.uploadUrl = resume.uploadUrl;
				uploadUrl = await createUpload();
			}

			const total = file.size;
			let offset = snapshot.offset;
			// Zero-byte uploads still require one PATCH for the server to finalize.
			let finalized = total !== 0 && offset >= total;
			let finishedFromRecovery = false;
			let phase: TusPhase = finalized ? 'committing' : 'uploading';

			while (offset < total || !finalized) {
				requirementMet();
				const chunk = file.slice(offset, Math.min(offset + chunkSize, total));
				phase = offset + chunk.size >= total ? 'committing' : 'uploading';
				setPhase('uploading');
				emitProgress(offset);
				let attempts = 0;
				let success = false;
				let paused = false;
				while (!success) {
					requirementMet();
					await flushCallbacks();
					attempts++;
					try {
						const response = await patchChunk(uploadUrl, offset, chunk);
						if (response.ok) {
							let newOffset = offset + chunk.size;
							const header = response.header('Upload-Offset');
							if (header !== null) {
								const parsed = Number(header);
								if (Number.isInteger(parsed) && parsed >= 0 && parsed <= total) newOffset = parsed;
							}
							offset = newOffset;
							if (offset >= total) {
								setPhase('committing');
								let item = decodeTusItem(response.header('Upload-Item'));
								const quickId = itemIdFrom(item, response.header('Upload-Item-Id'));
								if (quickId) {
									committedItem = item ?? { id: quickId };
									finish(offset, committedItem, quickId);
								} else {
									update({ offset });
								}
								emitProgress(offset, false);
							} else {
								update({ offset });
							}
							finalized = offset >= total;
							success = true;
							break;
						}
						if (response.status >= 400 && response.status < 500 && response.status !== 409) {
							return markFailed(
								locale.language === 'id'
									? `Upload gagal (HTTP ${response.status})`
									: `Upload failed (HTTP ${response.status})`
							);
						}
						throw new Error(
							locale.language === 'id'
								? `Chunk tidak diterima (HTTP ${response.status})`
								: `Chunk was not accepted (HTTP ${response.status})`
						);
					} catch (err) {
						if (pauseRequested()) {
							paused = true;
							break;
						}
						// Network or 5xx/409: recover the authoritative server offset.
						let advanced = false;
						try {
							const recovery = await recoverOffset(uploadUrl);
							if (recovery) {
								const id = itemIdFrom(recovery.item, null);
								if (id) committedItem = recovery.item;
								if (recovery.offset !== offset) {
									offset = recovery.offset;
									emitProgress(offset);
									advanced = true;
								}
								if (recovery.offset >= total) {
									// The server already finalized the upload (lost final
									// response); adopt the recorded item without re-sending.
									update({ offset: recovery.offset });
									finishedFromRecovery = true;
									finalized = true;
									finish(recovery.offset, recovery.item, id);
									success = true;
									break;
								}
							}
						} catch {}
						if (!advanced) {
							if (attempts >= maxRetries) {
								// The commit state is unknown to the client: the server
								// may yet finalize; return a recoverable snapshot.
								update({
									state: 'paused',
									error: err instanceof Error ? err.message : String(err)
								});
								return {
									ok: false,
									reason: phase === 'committing' ? 'uncertain' : 'failed',
									snapshot
								};
							}
							await throttle(attempts);
						}
					}
				}
				if (paused) return markPaused(phase);
			}

			if (!finishedFromRecovery && !committedItem) {
				// No Upload-Item header ever arrived: recover via HEAD.
				setPhase('verifying');
				const recovery = await recoverOffset(uploadUrl!);
				if (recovery && recovery.offset >= total) {
					committedItem = recovery.item;
					const id = itemIdFrom(committedItem, null);
					finish(recovery.offset, committedItem, id);
				}
			}
			if (!committedItem) {
				return {
					ok: false,
					reason: 'uncertain',
					snapshot: {
						...snapshot,
						state: 'paused',
						error: 'Item ID tidak ditemukan pada respon TUS'
					}
				};
			}
			return { ok: true, item: committedItem, snapshot };
		} catch (err) {
			if (err instanceof TusResumeError) {
				update({ state: 'failed', error: err.message });
				return { ok: false, reason: 'failed', snapshot };
			}
			if (pauseRequested()) {
				return markPaused(scalePhase(snapshot.state));
			}
			const message = err instanceof Error ? err.message : String(err);
			update({ state: 'failed', error: message });
			return { ok: false, reason: 'failed', snapshot };
		} finally {
			try {
				await flushCallbacks();
			} finally {
				options.signal?.removeEventListener('abort', forwardAbort);
			}
		}
	}

	function scalePhase(state: TusTransferState): TusPhase {
		if (state === 'committing') return 'committing';
		if (state === 'verifying') return 'verifying';
		return 'uploading';
	}

	return {
		run,
		snapshot: () => snapshot,
		pause: () => pauseController.abort(),
		cancel: async () => {
			pauseController.abort();
			const uploadUrl = snapshot.uploadUrl;
			update({ state: 'cancelled' });
			if (!uploadUrl) return true;
			try {
				const response = await fetch(uploadUrl, {
					method: 'DELETE',
					headers: { 'Tus-Resumable': TUS_VERSION },
					credentials: 'include'
				});
				if (response.ok || response.status === 404) return true;
				return false;
			} catch {
				return false;
			}
		}
	};
}
