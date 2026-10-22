import { locale } from '$lib/locale.svelte.js';
import { sha256File } from '$lib/api/incremental-sha256.js';
import {
	MultipartError,
	type MultipartSessionSnapshot,
	transferMultipartSession
} from '$lib/api/multipart-transfer.js';
import { createTusTransfer, type TusSessionSnapshot } from '$lib/api/tus-transfer.js';
import type {
	TransferCancelDriver,
	TransferDriver,
	TransferRecord,
	TransferSessionSnapshot
} from '$lib/transfer-queue.svelte.js';
import type { ExplorerVariant } from '$lib/types/drive.js';

interface APIErrorBody {
	error?: string;
}

interface MultipartIntentResponse extends APIErrorBody {
	placement: string;
	mode: string;
	upload_id?: string;
	url?: string;
	method?: string;
	expires_at?: string;
	size_bytes?: number;
	part_size_bytes?: number;
	part_count?: number;
	parts?: MultipartSessionSnapshot['parts'];
}

interface BackendMultipartState {
	upload_id: string;
	filename: string;
	size_bytes: number;
	part_size_bytes: number;
	part_count: number;
	status: string;
	expires_at?: string;
	final_item_id?: string;
	parts: MultipartSessionSnapshot['parts'];
}

interface SinglePutSnapshot {
	base: string;
	upload_id: string;
	filename: string;
	size_bytes: number;
	parent_id: string | null;
	unique_name: boolean;
	stage: 'uploading' | 'committing';
}

class TransferAPIError extends Error {
	constructor(
		message: string,
		readonly code?: string,
		readonly status?: number
	) {
		super(message);
		this.name = 'TransferAPIError';
	}
}

class FileMismatchError extends Error {
	readonly code = 'FILE_MISMATCH';

	constructor() {
		super(
			locale.language === 'id'
				? 'File yang dipilih tidak cocok dengan unggahan tersimpan. Pilih file asli yang benar.'
				: 'The selected file does not match the saved upload. Choose the correct original file.'
		);
		this.name = 'FileMismatchError';
	}
}

const tx = (id: string, en: string): string => (locale.language === 'id' ? id : en);

function apiErrorMessage(code: string | undefined, status: number): string {
	if (code === 'USER_QUOTA_EXCEEDED')
		return tx('Kuota penyimpanan penuh. Hubungi admin.', 'Storage quota is full. Contact an administrator.');
	if (code === 'STORAGE_POOL_FULL') return tx('Penyimpanan server penuh.', 'Server storage is full.');
	if (code === 'multipart_session_expired')
		return tx(
			'Sesi upload kedaluwarsa. Mulai ulang upload berkas ini.',
			'The upload session has expired. Restart this file upload.'
		);
	if (code === 'multipart_unrecoverable')
		return tx(
			'Sesi upload tidak dapat dilanjutkan. Mulai ulang upload berkas ini.',
			'This upload session cannot be resumed. Restart this file upload.'
		);
	return tx(`Permintaan upload gagal (HTTP ${status})`, `Upload request failed (HTTP ${status})`);
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
	const response = await fetch(path, { credentials: 'include', ...init });
	const text = await response.text();
	let body: (T & APIErrorBody) | undefined;
	if (text) {
		try {
			body = JSON.parse(text) as T & APIErrorBody;
		} catch {
			if (response.ok)
				throw new TransferAPIError(
					tx('Respons upload tidak valid', 'Invalid upload response'),
					undefined,
					response.status
				);
		}
	}
	if (!response.ok) {
		const code = body?.error;
		throw new TransferAPIError(apiErrorMessage(code, response.status), code, response.status);
	}
	return body as T;
}

function transferSession(
	transport: TransferSessionSnapshot['transport'],
	sessionId: string,
	fingerprint: string,
	snapshot: unknown,
	expiresAt?: number
): TransferSessionSnapshot {
	return { transport, sessionId, fingerprint, snapshot, expiresAt };
}

function tusSessionId(snapshot: TusSessionSnapshot): string {
	return snapshot.uploadUrl?.split('/').filter(Boolean).at(-1) ?? 'pending';
}

function tusSnapshot(session: TransferSessionSnapshot): TusSessionSnapshot {
	if (session.transport !== 'tus' || !session.snapshot || typeof session.snapshot !== 'object') {
		throw new TransferAPIError(
			tx('Sesi TUS tidak dapat dipulihkan.', 'The TUS session cannot be resumed.'),
			'UPLOAD_UNRECOVERABLE',
			410
		);
	}
	return session.snapshot as TusSessionSnapshot;
}

function multipartSnapshot(session: TransferSessionSnapshot): MultipartSessionSnapshot {
	if (
		session.transport !== 'multipart' ||
		!session.snapshot ||
		typeof session.snapshot !== 'object'
	) {
		throw new TransferAPIError(
			tx('Sesi multipart tidak dapat dipulihkan.', 'The multipart session cannot be resumed.'),
			'multipart_unrecoverable',
			410
		);
	}
	return session.snapshot as MultipartSessionSnapshot;
}

function singlePutSnapshot(session: TransferSessionSnapshot): SinglePutSnapshot {
	if (
		session.transport !== 'single-put' ||
		!session.snapshot ||
		typeof session.snapshot !== 'object'
	) {
		throw new TransferAPIError(
			tx('Sesi upload tidak dapat dipulihkan.', 'The upload session cannot be resumed.'),
			'UPLOAD_UNRECOVERABLE',
			410
		);
	}
	return session.snapshot as SinglePutSnapshot;
}

function throwTusFailure(snapshot: TusSessionSnapshot): never {
	const error = snapshot.error ?? '';
	if (/tidak ditemukan|not found|kedaluwarsa|expired/i.test(error)) {
		throw new TransferAPIError(
			tx('Sesi upload tidak ditemukan atau sudah kedaluwarsa.', 'The upload session was not found or has expired.'),
			'UPLOAD_SESSION_EXPIRED',
			410
		);
	}
	if (/checksum|digest|file.*tidak cocok/i.test(error)) throw new FileMismatchError();
	throw new TransferAPIError(tx('Upload TUS gagal', 'TUS upload failed'));
}

async function uploadTus(
	base: string,
	file: File,
	record: TransferRecord,
	fingerprint: string,
	checksum: string,
	options: Parameters<TransferDriver>[2]
): Promise<void> {
	const saved = record.session ? tusSnapshot(record.session) : undefined;
	if (
		saved &&
		(saved.scope !== base ||
			saved.filename !== file.name ||
			saved.size !== file.size ||
			saved.parentId !== record.parentId)
	) {
		throw new FileMismatchError();
	}
	const transfer = createTusTransfer({
		file,
		scope: base as TusSessionSnapshot['scope'],
		parentId: record.parentId,
		checksum,
		signal: options.signal,
		resume: saved,
		onProgress: ({ percent }) => options.onProgress(percent),
		onPhase: options.onPhase,
		onSession: (snapshot) =>
			options.onSession(
				transferSession('tus', tusSessionId(snapshot), fingerprint, snapshot, record.expiresAt)
			)
	});
	const result = await transfer.run();
	if (options.signal.aborted)
		throw options.signal.reason ?? new DOMException('Aborted', 'AbortError');
	if (!result.ok) {
		if (result.reason === 'paused' || result.reason === 'uncertain') {
			if (result.reason === 'uncertain') {
				const error = new TransferAPIError(
					tx(
						'Finalisasi upload belum terkonfirmasi',
						'Upload finalization has not been confirmed'
					),
					'NETWORK_ERROR',
					0
				);
				throw error;
			}
			throw new DOMException(tx('Upload dijeda', 'Upload paused'), 'AbortError');
		}
		throwTusFailure(result.snapshot);
	}
}

function asMultipartSnapshot(
	base: string,
	scope: 'my-storage' | 'organization',
	state: BackendMultipartState,
	record: TransferRecord,
	uniqueName: boolean,
	previous?: MultipartSessionSnapshot
): MultipartSessionSnapshot {
	return {
		scope,
		base,
		upload_id: state.upload_id,
		filename: state.filename,
		size_bytes: state.size_bytes,
		part_size_bytes: state.part_size_bytes,
		part_count: state.part_count,
		parent_id: record.parentId,
		unique_name: previous?.unique_name ?? uniqueName,
		created_at: previous?.created_at ?? new Date().toISOString(),
		expires_at: state.expires_at,
		status: state.status === 'completed' ? 'completed' : 'active',
		final_item_id: state.final_item_id,
		parts: state.parts ?? []
	};
}

async function uploadMultipart(
	base: string,
	scope: 'my-storage' | 'organization',
	file: File,
	record: TransferRecord,
	fingerprint: string,
	options: Parameters<TransferDriver>[2],
	previous?: TransferSessionSnapshot
): Promise<void> {
	const saved = previous ? multipartSnapshot(previous) : undefined;
	let session: MultipartSessionSnapshot;
	if (saved) {
		const state = await request<BackendMultipartState>(
			`${base}/multipart/${encodeURIComponent(saved.upload_id)}`
		);
		session = asMultipartSnapshot(base, scope, state, record, true, saved);
	} else {
		const intent = await request<MultipartIntentResponse>(`${base}/multipart-intent`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({
				filename: file.name,
				size: file.size,
				parent_id: record.parentId,
				unique_name: true
			})
		});
		if (intent.mode !== 'multipart' || !intent.upload_id) {
			if (scope === 'organization') {
				await uploadTus(base, file, record, fingerprint, '', options);
				return;
			}
			throw new TransferAPIError(
				tx('Respons cloud multipart tidak lengkap', 'Incomplete cloud multipart response')
			);
		}
		const state = await request<BackendMultipartState>(
			`${base}/multipart/${encodeURIComponent(intent.upload_id)}`
		);
		session = asMultipartSnapshot(base, scope, state, record, true);
	}

	if (session.status === 'completed') {
		if (!session.final_item_id)
			throw new TransferAPIError(
				tx('Sesi selesai tanpa ID berkas akhir.', 'The completed session has no final file ID.'),
				'multipart_unrecoverable',
				410
			);
		await request(`${base}/items/${encodeURIComponent(session.final_item_id)}`);
		options.onProgress(100);
		return;
	}
	if (session.expires_at && Date.parse(session.expires_at) <= Date.now()) {
		throw new TransferAPIError(
			tx(
				'Sesi upload kedaluwarsa. Mulai ulang upload berkas ini.',
				'The upload session has expired. Restart this file upload.'
			),
			'multipart_session_expired',
			410
		);
	}

	let checkpoint = Promise.resolve();
	const persist = (snapshot: MultipartSessionSnapshot): Promise<void> => {
		checkpoint = checkpoint.then(async () => {
			const next = { ...snapshot, parent_id: record.parentId, unique_name: session.unique_name };
			await options.onSession(
				transferSession(
					'multipart',
					snapshot.upload_id,
					fingerprint,
					next,
					snapshot.expires_at ? Date.parse(snapshot.expires_at) : undefined
				)
			);
			if (snapshot.status === 'active') {
				const changedParts = snapshot.parts
					.filter((part) => part.status !== 'pending')
					.map((part) => ({
						part_number: part.part_number,
						size_bytes: part.size_bytes,
						etag: part.etag,
						status: part.status
					}));
				if (changedParts.length > 0) {
					await request(`${base}/multipart/${encodeURIComponent(snapshot.upload_id)}/parts`, {
						method: 'PUT',
						headers: { 'Content-Type': 'application/json' },
						body: JSON.stringify({ parts: changedParts })
					});
				}
			}
		});
		return checkpoint;
	};
	await persist(session);
	await options.onPhase('uploading');
	try {
		await transferMultipartSession(
			file,
			session,
			{
				checkpoint: persist,
				onPhase: options.onPhase,
				progress: (loadedBytes, totalBytes) => {
					options.onProgress(totalBytes === 0 ? 100 : Math.floor((loadedBytes / totalBytes) * 99));
				}
			},
			{ signal: options.signal }
		);
		await checkpoint;
		options.onProgress(100);
		return;
	} catch (error) {
		await checkpoint;
		if (options.signal.aborted)
			throw options.signal.reason ?? new DOMException('Aborted', 'AbortError');
		if (error instanceof MultipartError) {
			throw new TransferAPIError(error.message, error.code, error.status);
		}
		throw error;
	}
}

function uploadSinglePut(
	base: string,
	file: File,
	intent: { upload_id: string; url: string; method?: string; expires_at?: string },
	fingerprint: string,
	record: TransferRecord,
	options: Parameters<TransferDriver>[2]
): Promise<void> {
	return new Promise((resolve, reject) => {
		const xhr = new XMLHttpRequest();
		xhr.open(intent.method ?? 'PUT', intent.url);
		xhr.upload.onprogress = (event) => {
			if (event.lengthComputable && event.total > 0) {
				options.onProgress(Math.min(99, Math.round((event.loaded / event.total) * 99)));
			}
		};
		const abort = () => xhr.abort();
		options.signal.addEventListener('abort', abort, { once: true });
		xhr.onload = async () => {
			options.signal.removeEventListener('abort', abort);
			if (xhr.status < 200 || xhr.status >= 300) {
				reject(
					new TransferAPIError(`Upload ke cloud gagal (HTTP ${xhr.status})`, undefined, xhr.status)
				);
				return;
			}
			try {
				await options.onPhase('committing');
				await options.onSession(
					transferSession(
						'single-put',
						intent.upload_id,
						fingerprint,
						{
							base,
							upload_id: intent.upload_id,
							filename: file.name,
							size_bytes: file.size,
							parent_id: record.parentId,
							unique_name: true,
							stage: 'committing'
						},
						intent.expires_at ? Date.parse(intent.expires_at) : undefined
					)
				);
				await request(`${base}/sync-cloud`, {
					method: 'POST',
					headers: { 'Content-Type': 'application/json' },
					body: JSON.stringify({ upload_id: intent.upload_id })
				});
				options.onProgress(100);
				resolve();
			} catch (error) {
				reject(error);
			}
		};
		xhr.onerror = () => {
			options.signal.removeEventListener('abort', abort);
			reject(
				new TransferAPIError(
					tx('Koneksi ke cloud storage gagal', 'Connection to cloud storage failed'),
					'NETWORK_ERROR',
					0
				)
			);
		};
		xhr.onabort = () => {
			options.signal.removeEventListener('abort', abort);
			reject(options.signal.reason ?? new DOMException('Aborted', 'AbortError'));
		};
		if (options.signal.aborted) {
			abort();
			return;
		}
		xhr.send(file);
	});
}

async function runSinglePut(
	base: string,
	file: File,
	record: TransferRecord,
	fingerprint: string,
	options: Parameters<TransferDriver>[2],
	session?: TransferSessionSnapshot
): Promise<void> {
	if (session) {
		const saved = singlePutSnapshot(session);
		if (
			saved.filename !== file.name ||
			saved.size_bytes !== file.size ||
			saved.parent_id !== record.parentId
		) {
			throw new FileMismatchError();
		}
		if (saved.stage === 'committing') {
			try {
				await request(`${base}/sync-cloud`, {
					method: 'POST',
					headers: { 'Content-Type': 'application/json' },
					body: JSON.stringify({ upload_id: saved.upload_id })
				});
				options.onProgress(100);
				return;
			} catch (error) {
				if ((error as TransferAPIError).status === 413) throw error;
			}
		}
		const renewed = await request<{
			upload_id: string;
			url: string;
			method?: string;
			expires_at?: string;
		}>(`${base}/upload-intent/${encodeURIComponent(saved.upload_id)}/renew-url`, {
			method: 'POST'
		});
		await options.onPhase('uploading');
		await uploadSinglePut(base, file, renewed, fingerprint, record, options);
		return;
	}

	const intent = await request<MultipartIntentResponse>(`${base}/multipart-intent`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({
			filename: file.name,
			size: file.size,
			parent_id: record.parentId,
			unique_name: true
		})
	});
	if (intent.mode !== 'single_put' || !intent.upload_id || !intent.url) {
		throw new TransferAPIError('Respons cloud single-PUT tidak lengkap');
	}
	const uploadIntent = {
		upload_id: intent.upload_id,
		url: intent.url,
		method: intent.method,
		expires_at: intent.expires_at
	};
	const expiresAt = intent.expires_at ? Date.parse(intent.expires_at) : undefined;
	await options.onSession(
		transferSession(
			'single-put',
			intent.upload_id,
			fingerprint,
			{
				base,
				upload_id: intent.upload_id,
				filename: file.name,
				size_bytes: file.size,
				parent_id: record.parentId,
				unique_name: true,
				stage: 'uploading'
			} satisfies SinglePutSnapshot,
			expiresAt
		)
	);
	await options.onPhase('uploading');
	await uploadSinglePut(base, file, uploadIntent, fingerprint, record, options);
}

export function createResumableUploadDriver(
	base: string,
	area: ExplorerVariant
): {
	driver: TransferDriver;
	cancel: TransferCancelDriver;
} {
	const scope = area === 'organization' ? 'organization' : 'my-storage';
	const driver: TransferDriver = async (file, record, options) => {
		if (area !== 'organization' && area !== 'my-storage')
			throw new Error(tx('Area upload tidak didukung', 'This upload area is not supported'));
		await options.onPhase('verifying');
		const digest = await sha256File(file, options.signal);
		if (record.fingerprint && record.fingerprint !== digest.hex) throw new FileMismatchError();
		if (record.session?.fingerprint && record.session.fingerprint !== digest.hex)
			throw new FileMismatchError();
		await options.onFingerprint(digest.hex);

		if (record.session?.transport === 'tus') {
			await uploadTus(base, file, record, digest.hex, digest.base64, options);
			return;
		}
		if (record.session?.transport === 'multipart') {
			await uploadMultipart(base, scope, file, record, digest.hex, options, record.session);
			return;
		}
		if (record.session?.transport === 'single-put') {
			await runSinglePut(base, file, record, digest.hex, options, record.session);
			return;
		}

		if (area === 'organization') {
			const intent = await request<MultipartIntentResponse>(`${base}/multipart-intent`, {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({
					filename: file.name,
					size: file.size,
					parent_id: record.parentId,
					unique_name: true
				})
			});
			if (intent.mode === 'multipart' && intent.upload_id) {
				const state = await request<BackendMultipartState>(
					`${base}/multipart/${encodeURIComponent(intent.upload_id)}`
				);
				const session = asMultipartSnapshot(base, scope, state, record, true);
				await uploadMultipart(
					base,
					scope,
					file,
					record,
					digest.hex,
					options,
					transferSession(
						'multipart',
						intent.upload_id,
						digest.hex,
						session,
						state.expires_at ? Date.parse(state.expires_at) : undefined
					)
				);
				return;
			}
			await uploadTus(base, file, record, digest.hex, digest.base64, options);
			return;
		}

		const intent = await request<MultipartIntentResponse>(`${base}/multipart-intent`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({
				filename: file.name,
				size: file.size,
				parent_id: record.parentId,
				unique_name: true
			})
		});
		if (intent.mode === 'local') {
			await uploadTus(base, file, record, digest.hex, digest.base64, options);
			return;
		}
		if (intent.mode === 'single_put') {
			if (!intent.upload_id || !intent.url)
				throw new TransferAPIError('Respons cloud single-PUT tidak lengkap');
			const expiresAt = intent.expires_at ? Date.parse(intent.expires_at) : undefined;
			await options.onSession(
				transferSession(
					'single-put',
					intent.upload_id,
					digest.hex,
					{
						base,
						upload_id: intent.upload_id,
						filename: file.name,
						size_bytes: file.size,
						parent_id: record.parentId,
						unique_name: true,
						stage: 'uploading'
					} satisfies SinglePutSnapshot,
					expiresAt
				)
			);
			await options.onPhase('uploading');
			await uploadSinglePut(
				base,
				file,
				intent as Required<Pick<MultipartIntentResponse, 'upload_id' | 'url'>> &
					MultipartIntentResponse,
				digest.hex,
				record,
				options
			);
			return;
		}
		if (intent.mode === 'multipart' && intent.upload_id) {
			const state = await request<BackendMultipartState>(
				`${base}/multipart/${encodeURIComponent(intent.upload_id)}`
			);
			const session = asMultipartSnapshot(base, scope, state, record, true);
			await uploadMultipart(
				base,
				scope,
				file,
				record,
				digest.hex,
				options,
				transferSession(
					'multipart',
					intent.upload_id,
					digest.hex,
					session,
					state.expires_at ? Date.parse(state.expires_at) : undefined
				)
			);
			return;
		}
		throw new TransferAPIError(tx('Respons multipart upload tidak dapat dikenali', 'Unrecognized multipart upload response'));
	};

	const cancel: TransferCancelDriver = async (record) => {
		const session = record.session;
		if (!session) return;
		if (session.transport === 'tus') {
			const snapshot = tusSnapshot(session);
			if (!snapshot.uploadUrl) return;
			const response = await fetch(snapshot.uploadUrl, {
				method: 'DELETE',
				headers: { 'Tus-Resumable': '1.0.0' },
				credentials: 'include'
			});
			if (!response.ok && response.status !== 404 && response.status !== 410) {
				throw new TransferAPIError(
					tx('Sesi TUS belum dapat dibatalkan.', 'The TUS session could not be cancelled.'),
					undefined,
					response.status
				);
			}
			return;
		}
		const snapshot =
			session.transport === 'multipart' ? multipartSnapshot(session) : singlePutSnapshot(session);
		const path =
			session.transport === 'multipart'
				? `${base}/multipart/${encodeURIComponent(snapshot.upload_id)}`
				: `${base}/upload-intent/${encodeURIComponent(snapshot.upload_id)}`;
		const response = await fetch(path, { method: 'DELETE', credentials: 'include' });
		if (!response.ok && response.status !== 404 && response.status !== 410) {
			const body = (await response.json().catch(() => null)) as APIErrorBody | null;
			throw new TransferAPIError(
				apiErrorMessage(body?.error, response.status),
				body?.error,
				response.status
			);
		}
	};
	return { driver, cancel };
}
