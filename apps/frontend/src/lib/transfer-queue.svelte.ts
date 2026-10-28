import type { ExplorerVariant } from '$lib/types/drive.js';

const DB_NAME = 'cubycloud-transfer-queue';
const DB_VERSION = 1;
const STORE_NAME = 'transfers';
const RETENTION_MS = 24 * 60 * 60 * 1000;
const LEASE_MS = 30_000;
const HEARTBEAT_MS = 5_000;
const CHANNEL_NAME = 'cubycloud-transfer-queue';

export type TransferState =
	| 'queued'
	| 'uploading'
	| 'retrying'
	| 'paused'
	| 'waiting-for-file'
	| 'verifying'
	| 'committing'
	| 'completed'
	| 'failed'
	| 'expired';

export interface TransferSessionSnapshot {
	transport: 'tus' | 'multipart' | 'single-put';
	sessionId: string;
	fingerprint?: string;
	expiresAt?: number;
	parts?: { number: number; etag?: string; state: 'pending' | 'complete' | 'failed' }[];
	snapshot?: unknown;
}

export interface TransferRecord {
	id: string;
	ownerId: string;
	area: ExplorerVariant;
	batchId: string;
	name: string;
	relativePath: string;
	parentId: string | null;
	size: number;
	lastModified: number;
	state: TransferState;
	progress: number;
	createdAt: number;
	updatedAt: number;
	expiresAt: number;
	retainUntil: number;
	error?: string;
	errorCode?: string;
	fingerprint?: string;
	session?: TransferSessionSnapshot;
	lockOwner?: string;
	lockExpiresAt?: number;
}

export interface NewTransfer {
	name: string;
	relativePath: string;
	parentId: string | null;
	file: File;
}

export interface TransferDriverOptions {
	signal: AbortSignal;
	session?: TransferSessionSnapshot;
	onFingerprint: (fingerprint: string) => Promise<void>;
	onSession: (session: TransferSessionSnapshot) => Promise<void>;
	onPhase: (phase: 'verifying' | 'uploading' | 'committing') => Promise<void>;
	onProgress: (progress: number) => void;
}

export type TransferDriver = (
	file: File,
	record: TransferRecord,
	options: TransferDriverOptions
) => Promise<void>;
export type TransferCancelDriver = (record: TransferRecord) => Promise<void>;

export const transferQueueState = $state({
	items: [] as TransferRecord[],
	ownerId: '',
	area: 'my-storage' as ExplorerVariant,
	ready: false
});

const tabId = globalThis.crypto.randomUUID();
const inMemoryFiles = new Map<string, File>();
const activeTransfers = new Map<
	string,
	{ controller: AbortController; action: 'pause' | 'cancel' | null; phase: TransferState }
>();
let database: Promise<IDBDatabase> | null = null;
let channel: BroadcastChannel | null = null;
let loadedOwner = '';
let loadedArea: ExplorerVariant = 'my-storage';

function openDatabase(): Promise<IDBDatabase> {
	if (typeof indexedDB === 'undefined')
		return Promise.reject(new Error('IndexedDB tidak tersedia di browser ini'));
	if (database) return database;

	const { promise, resolve, reject } = Promise.withResolvers<IDBDatabase>();
	database = promise;
	const request = indexedDB.open(DB_NAME, DB_VERSION);
	request.onupgradeneeded = () => {
		const db = request.result;
		const store = db.createObjectStore(STORE_NAME, { keyPath: 'id' });
		store.createIndex('ownerArea', ['ownerId', 'area'], { unique: false });
		store.createIndex('retainUntil', 'retainUntil', { unique: false });
	};
	request.onsuccess = () => {
		request.result.onversionchange = () => request.result.close();
		resolve(request.result);
	};
	request.onerror = () => {
		database = null;
		reject(request.error ?? new Error('Gagal membuka antrean unggahan'));
	};
	request.onblocked = () => {
		database = null;
		reject(new Error('Database antrean unggahan sedang dipakai tab lain'));
	};
	return promise;
}

function requestResult<T>(request: IDBRequest<T>): Promise<T> {
	const { promise, resolve, reject } = Promise.withResolvers<T>();
	request.onsuccess = () => resolve(request.result);
	request.onerror = () => reject(request.error ?? new Error('Operasi antrean gagal'));
	return promise;
}

function transactionDone(transaction: IDBTransaction): Promise<void> {
	const { promise, resolve, reject } = Promise.withResolvers<void>();
	transaction.oncomplete = () => resolve();
	transaction.onabort = transaction.onerror = () =>
		reject(transaction.error ?? new Error('Transaksi antrean gagal'));
	return promise;
}

async function readAll(): Promise<TransferRecord[]> {
	const db = await openDatabase();
	const transaction = db.transaction(STORE_NAME, 'readonly');
	const done = transactionDone(transaction);
	const records = await requestResult(
		transaction.objectStore(STORE_NAME).getAll() as IDBRequest<TransferRecord[]>
	);
	await done;
	return records;
}

async function readRecord(id: string): Promise<TransferRecord | undefined> {
	const db = await openDatabase();
	const transaction = db.transaction(STORE_NAME, 'readonly');
	const done = transactionDone(transaction);
	const record = await requestResult(
		transaction.objectStore(STORE_NAME).get(id) as IDBRequest<TransferRecord | undefined>
	);
	await done;
	return record;
}

async function updateRecord(
	id: string,
	patch: Partial<TransferRecord>
): Promise<TransferRecord | undefined> {
	const db = await openDatabase();
	const transaction = db.transaction(STORE_NAME, 'readwrite');
	const done = transactionDone(transaction);
	const store = transaction.objectStore(STORE_NAME);
	const request = store.get(id) as IDBRequest<TransferRecord | undefined>;
	let updated: TransferRecord | undefined;
	request.onsuccess = () => {
		if (!request.result) return;
		updated = { ...request.result, ...patch, updatedAt: Date.now() };
		store.put(updated);
	};
	await done;
	if (updated) publish(updated);
	return updated;
}

async function deleteRecord(id: string): Promise<void> {
	const db = await openDatabase();
	const transaction = db.transaction(STORE_NAME, 'readwrite');
	const done = transactionDone(transaction);
	transaction.objectStore(STORE_NAME).delete(id);
	await done;
	inMemoryFiles.delete(id);
	await refreshQueue();
}

function publish(record: TransferRecord): void {
	if (record.ownerId === loadedOwner && record.area === loadedArea) {
		const index = transferQueueState.items.findIndex((item) => item.id === record.id);
		if (index === -1) transferQueueState.items = [...transferQueueState.items, record];
		else transferQueueState.items[index] = record;
	}
	channel?.postMessage({ type: 'changed', ownerId: record.ownerId, area: record.area });
}

async function refreshQueue(): Promise<void> {
	if (!loadedOwner) return;
	const records = await readAll();
	const visible = records.filter(
		(record) => record.ownerId === loadedOwner && record.area === loadedArea
	);
	transferQueueState.items = visible.sort((a, b) => a.createdAt - b.createdAt);
	for (const record of visible) {
		if (record.state === 'queued' && !inMemoryFiles.has(record.id)) {
			await updateRecord(record.id, {
				state: 'waiting-for-file',
				error: 'Pilih kembali file untuk melanjutkan unggahan yang tersimpan.'
			});
		}
	}
}
export async function resumeFailedTransfer(id: string): Promise<boolean> {
	const record = await readRecord(id);
	if (!record || record.state !== 'failed' || !record.session) return false;
	await updateRecord(id, { state: 'paused', error: undefined, errorCode: undefined });
	return true;
}

export async function failPendingTransfers(ids: string[], message: string): Promise<void> {
	for (const id of ids) {
		const record = await readRecord(id);
		if (!record || record.state !== 'queued') continue;
		inMemoryFiles.delete(id);
		await updateRecord(id, {
			state: 'failed',
			error: message,
			retainUntil: Date.now() + RETENTION_MS
		});
	}
}

function ensureChannel(): void {
	if (channel || typeof BroadcastChannel === 'undefined') return;
	channel = new BroadcastChannel(CHANNEL_NAME);
	channel.onmessage = (
		event: MessageEvent<{
			type?: string;
			ownerId?: string;
			area?: ExplorerVariant;
			id?: string;
			action?: 'pause' | 'cancel';
		}>
	) => {
		const message = event.data;
		if (message.type === 'command' && message.id && message.action) {
			const active = activeTransfers.get(message.id);
			if (active && !(message.action === 'cancel' && active.phase === 'committing')) {
				active.action = message.action;
				active.controller.abort(message.action);
			}
			return;
		}
		if (
			message.type === 'changed' &&
			message.ownerId === loadedOwner &&
			message.area === loadedArea
		) {
			void refreshQueue();
		}
	};
}

function isActiveState(state: TransferState): boolean {
	return (
		state === 'queued' ||
		state === 'uploading' ||
		state === 'retrying' ||
		state === 'verifying' ||
		state === 'committing'
	);
}
function isCommittingPhase(phase: TransferState): boolean {
	return phase === 'committing';
}

export async function loadTransferQueue(ownerId: string, area: ExplorerVariant): Promise<void> {
	ensureChannel();
	if (loadedOwner !== ownerId || loadedArea !== area) inMemoryFiles.clear();
	loadedOwner = ownerId;
	loadedArea = area;
	transferQueueState.ownerId = ownerId;
	transferQueueState.area = area;
	transferQueueState.ready = false;

	const now = Date.now();
	const records = await readAll();
	const db = await openDatabase();
	const transaction = db.transaction(STORE_NAME, 'readwrite');
	const done = transactionDone(transaction);
	const store = transaction.objectStore(STORE_NAME);
	const current: TransferRecord[] = [];
	for (const record of records) {
		if (record.retainUntil <= now) {
			store.delete(record.id);
			inMemoryFiles.delete(record.id);
			continue;
		}
		if (
			record.expiresAt <= now &&
			record.state !== 'completed' &&
			record.state !== 'failed' &&
			record.state !== 'expired'
		) {
			const expired = {
				...record,
				state: 'expired' as const,
				error: 'Sesi unggahan kedaluwarsa. Mulai ulang dari nol dengan memilih file kembali.',
				lockOwner: undefined,
				lockExpiresAt: 0,
				updatedAt: now,
				retainUntil: now + RETENTION_MS
			};
			store.put(expired);
			inMemoryFiles.delete(record.id);
			if (record.ownerId === ownerId && record.area === area) current.push(expired);
			continue;
		}
		if (record.ownerId !== ownerId || record.area !== area) continue;
		if (isActiveState(record.state) && (!record.lockOwner || (record.lockExpiresAt ?? 0) <= now)) {
			const waiting: TransferRecord = {
				...record,
				state: 'waiting-for-file',
				error: 'Pilih kembali file untuk melanjutkan unggahan yang tersimpan.',
				lockOwner: undefined,
				lockExpiresAt: 0,
				updatedAt: now
			};
			store.put(waiting);
			current.push(waiting);
		} else {
			current.push(record);
		}
	}
	await done;
	transferQueueState.items = current.sort((a, b) => a.createdAt - b.createdAt);
	transferQueueState.ready = true;
}

export async function enqueueTransfers(
	ownerId: string,
	area: ExplorerVariant,
	transfers: NewTransfer[]
): Promise<string[]> {
	if (!ownerId) throw new Error('Sesi pengguna tidak tersedia untuk menyimpan antrean unggahan');
	const now = Date.now();
	const batchId = crypto.randomUUID();
	const records = transfers.map((transfer): TransferRecord => {
		const id = crypto.randomUUID();
		inMemoryFiles.set(id, transfer.file);
		return {
			id,
			ownerId,
			area,
			batchId,
			name: transfer.name,
			relativePath: transfer.relativePath,
			parentId: transfer.parentId,
			size: transfer.file.size,
			lastModified: transfer.file.lastModified,
			state: 'queued',
			progress: 0,
			createdAt: now,
			updatedAt: now,
			expiresAt: now + RETENTION_MS,
			retainUntil: now + RETENTION_MS * 2
		};
	});
	const db = await openDatabase();
	const transaction = db.transaction(STORE_NAME, 'readwrite');
	const done = transactionDone(transaction);
	const store = transaction.objectStore(STORE_NAME);
	for (const record of records) store.add(record);
	await done;
	for (const record of records) publish(record);
	return records.map((record) => record.id);
}

function errorDetails(error: unknown): { message: string; code?: string; status?: number } {
	const err = error as { message?: string; code?: string; status?: number };
	return {
		message: err?.message || 'Unggahan gagal',
		code: err?.code,
		status: err?.status
	};
}

function isQuotaError(error: unknown): boolean {
	const { message, code, status } = errorDetails(error);
	return (
		status === 413 ||
		code === 'USER_QUOTA_EXCEEDED' ||
		code === 'STORAGE_POOL_FULL' ||
		message.toLowerCase().includes('kuota')
	);
}

function isExpiredError(error: unknown): boolean {
	const { code, status } = errorDetails(error);
	return (
		status === 410 ||
		code === 'UPLOAD_EXPIRED' ||
		code === 'UPLOAD_SESSION_EXPIRED' ||
		code === 'UPLOAD_UNRECOVERABLE' ||
		code === 'MULTIPART_EXPIRED' ||
		code === 'MULTIPART_UNRECOVERABLE' ||
		code === 'multipart_session_expired' ||
		code === 'multipart_unrecoverable'
	);
}

function isFileMismatchError(error: unknown): boolean {
	const { code } = errorDetails(error);
	return code === 'FILE_MISMATCH' || code === 'CHECKSUM_MISMATCH';
}

function isRetryable(error: unknown): boolean {
	const { message, status, code } = errorDetails(error);
	if (status === 0 || code === 'NETWORK_ERROR' || code === 'network') return true;
	if (status === 408 || status === 429 || (status !== undefined && status >= 500)) return true;
	if (status !== undefined) return false;
	return /network|connection|timeout|temporarily unavailable|failed to fetch|koneksi/i.test(
		message
	);
}

function isFileMatch(file: File, record: TransferRecord): boolean {
	return (
		file.name === record.name.split(/[\\/]/).pop() &&
		file.size === record.size &&
		file.lastModified === record.lastModified
	);
}

function waitForRetry(delayMs: number, signal: AbortSignal): Promise<void> {
	if (signal.aborted) return Promise.reject(new DOMException('Aborted', 'AbortError'));
	const { promise, resolve, reject } = Promise.withResolvers<void>();
	let timer: ReturnType<typeof setTimeout> | undefined;
	const cleanup = () => {
		if (timer !== undefined) {
			clearTimeout(timer);
			timer = undefined;
		}
		window.removeEventListener('online', onOnline);
		signal.removeEventListener('abort', onAbort);
	};
	const finish = () => {
		cleanup();
		resolve();
	};
	const onOnline = () => finish();
	const onAbort = () => {
		cleanup();
		reject(new DOMException('Aborted', 'AbortError'));
	};
	if (typeof navigator !== 'undefined' && navigator.onLine === false) {
		window.addEventListener('online', onOnline, { once: true });
	} else {
		timer = setTimeout(finish, delayMs);
	}
	signal.addEventListener('abort', onAbort, { once: true });
	return promise;
}

async function claimLease(id: string, allowTakeover: boolean): Promise<boolean> {
	const db = await openDatabase();
	const transaction = db.transaction(STORE_NAME, 'readwrite');
	const done = transactionDone(transaction);
	const store = transaction.objectStore(STORE_NAME);
	const request = store.get(id) as IDBRequest<TransferRecord | undefined>;
	let updated: TransferRecord | undefined;
	request.onsuccess = () => {
		const record = request.result;
		if (!record) return;
		const now = Date.now();
		if (
			!allowTakeover &&
			record.lockOwner &&
			record.lockOwner !== tabId &&
			(record.lockExpiresAt ?? 0) > now
		) {
			return;
		}
		updated = { ...record, lockOwner: tabId, lockExpiresAt: now + LEASE_MS, updatedAt: now };
		store.put(updated);
	};
	await done;
	if (updated) publish(updated);
	return updated !== undefined;
}

async function releaseLease(id: string): Promise<void> {
	const record = await readRecord(id);
	if (record?.lockOwner === tabId)
		await updateRecord(id, { lockOwner: undefined, lockExpiresAt: 0 });
}

async function withTransferLock<T>(id: string, callback: () => Promise<T>): Promise<T | undefined> {
	const run = async (allowTakeover: boolean) => {
		if (!(await claimLease(id, allowTakeover))) return undefined;
		try {
			return await callback();
		} finally {
			await releaseLease(id);
		}
	};
	if (typeof navigator !== 'undefined' && navigator.locks?.request) {
		return navigator.locks.request(`cubycloud-transfer:${id}`, { ifAvailable: true }, (lock) =>
			lock ? run(true) : undefined
		);
	}
	return run(false);
}

export async function runTransfer(
	id: string,
	file: File,
	driver: TransferDriver,
	cancelDriver: TransferCancelDriver,
	allowPaused = false
): Promise<boolean> {
	inMemoryFiles.set(id, file);
	const result = await withTransferLock(id, async () => {
		const record = await readRecord(id);
		if (!record || record.ownerId !== loadedOwner || record.area !== loadedArea) {
			inMemoryFiles.delete(id);
			return false;
		}
		if (record.expiresAt <= Date.now()) {
			inMemoryFiles.delete(id);
			await updateRecord(id, {
				state: 'expired',
				error: 'Sesi unggahan kedaluwarsa. Mulai ulang dari nol dengan memilih file kembali.',
				retainUntil: Date.now() + RETENTION_MS
			});
			return false;
		}
		if (!isFileMatch(file, record)) {
			inMemoryFiles.delete(id);
			await updateRecord(id, {
				state: 'waiting-for-file',
				error:
					'File yang dipilih tidak cocok dengan unggahan tersimpan. Pilih file asli yang benar.'
			});
			return false;
		}
		if (
			record.state === 'completed' ||
			record.state === 'expired' ||
			record.state === 'failed' ||
			(record.state === 'paused' && !allowPaused)
		) {
			inMemoryFiles.delete(id);
			return false;
		}

		const active = {
			controller: new AbortController(),
			action: null as 'pause' | 'cancel' | null,
			phase: 'verifying' as TransferState
		};
		activeTransfers.set(id, active);
		const heartbeat = setInterval(
			() => void updateRecord(id, { lockOwner: tabId, lockExpiresAt: Date.now() + LEASE_MS }),
			HEARTBEAT_MS
		);
		const finishCancel = async () => {
			try {
				const latest = (await readRecord(id)) ?? record;
				if (latest.session) await cancelDriver(latest);
				await deleteRecord(id);
				inMemoryFiles.delete(id);
				return true;
			} catch (error) {
				const { message, status } = errorDetails(error);
				if (status === 404 || status === 410 || isExpiredError(error)) {
					await deleteRecord(id);
					inMemoryFiles.delete(id);
					return true;
				}
				await updateRecord(id, {
					state: 'failed',
					error: `Pembatalan belum dapat dikonfirmasi: ${message}`,
					lockOwner: undefined,
					lockExpiresAt: 0
				});
				return false;
			}
		};
		try {
			active.phase = 'verifying';
			record.state = 'verifying';
			await updateRecord(id, { state: 'verifying', error: undefined, errorCode: undefined });
			let attempt = 0;
			while (Date.now() < record.expiresAt) {
				if (active.action === 'pause') {
					await updateRecord(id, { state: 'paused', error: undefined });
					return false;
				}
				if (active.action === 'cancel' && !isCommittingPhase(active.phase)) return finishCancel();
				if (active.action === 'cancel') {
					active.action = null;
					await updateRecord(id, {
						state: 'retrying',
						error: 'Finalisasi sedang berlangsung; memeriksa hasil sebelum mengubah sesi.'
					});
				}

				const controller = new AbortController();
				active.controller = controller;
				let progressWrite = Promise.resolve();
				let progressError: unknown;
				let lastProgressWrite = 0;
				try {
					await driver(file, record, {
						signal: controller.signal,
						onFingerprint: async (fingerprint) => {
							record.fingerprint = fingerprint;
							await updateRecord(id, { fingerprint });
						},
						onSession: async (session) => {
							record.session = session;
							record.expiresAt = session.expiresAt ?? record.expiresAt;
							record.retainUntil = record.expiresAt + RETENTION_MS;
							await updateRecord(id, {
								session,
								expiresAt: record.expiresAt,
								retainUntil: record.retainUntil
							});
						},
						onPhase: async (phase) => {
							active.phase = phase;
							record.state = phase;
							await updateRecord(id, { state: phase, error: undefined, errorCode: undefined });
						},
						onProgress: (progress) => {
							record.progress = Math.max(0, Math.min(99, progress));
							if (Date.now() - lastProgressWrite < 250) return;
							lastProgressWrite = Date.now();
							progressWrite = progressWrite
								.then(async () => {
									await updateRecord(id, {
										progress: record.progress,
										lockOwner: tabId,
										lockExpiresAt: Date.now() + LEASE_MS
									});
								})
								.catch((error: unknown) => {
									progressError = error;
								});
						}
					});
					await progressWrite;
					if (progressError) throw progressError;
					await updateRecord(id, {
						state: 'completed',
						progress: 100,
						error: undefined,
						errorCode: undefined,
						session: undefined,
						retainUntil: Date.now() + RETENTION_MS
					});
					inMemoryFiles.delete(id);
					return true;
				} catch (error) {
					await progressWrite;
					if (active.action === 'pause') {
						await updateRecord(id, { state: 'paused', error: undefined });
						return false;
					}
					if (active.action === 'cancel' && !isCommittingPhase(active.phase)) return finishCancel();
					if (active.action === 'cancel') {
						active.action = null;
						await updateRecord(id, {
							state: 'retrying',
							error: 'Finalisasi sedang berlangsung; memeriksa hasil sebelum mengubah sesi.'
						});
						continue;
					}
					if (error instanceof DOMException && error.name === 'AbortError') {
						await updateRecord(id, { state: 'paused', error: undefined });
						return false;
					}
					if (isFileMismatchError(error)) {
						inMemoryFiles.delete(id);
						await updateRecord(id, {
							state: 'waiting-for-file',
							error: 'Isi file tidak cocok dengan sesi tersimpan. Pilih file asli yang benar.'
						});
						return false;
					}
					if (isExpiredError(error) || Date.now() >= record.expiresAt) {
						inMemoryFiles.delete(id);
						await updateRecord(id, {
							state: 'expired',
							error:
								'Sesi unggahan kedaluwarsa atau tidak dapat dipulihkan. Mulai ulang dari nol dengan memilih file kembali.',
							lockOwner: undefined,
							lockExpiresAt: 0,
							retainUntil: Date.now() + RETENTION_MS
						});
						return false;
					}
					if (!isRetryable(error) || isQuotaError(error)) {
						const detail = errorDetails(error);
						await updateRecord(id, {
							state: 'failed',
							error: detail.message,
							errorCode: detail.code
						});
						inMemoryFiles.delete(id);
						return false;
					}
					await updateRecord(id, {
						state: 'retrying',
						error: errorDetails(error).message,
						lockOwner: tabId,
						lockExpiresAt: Date.now() + LEASE_MS
					});
					try {
						await waitForRetry(
							Math.min(30_000, 500 * 2 ** Math.min(attempt, 6)),
							controller.signal
						);
					} catch {
						if (!active.action) {
							await updateRecord(id, { state: 'paused', error: undefined });
							return false;
						}
					}
					attempt += 1;
					const latest = await readRecord(id);
					if (latest) Object.assign(record, latest);
				}
			}
			await updateRecord(id, {
				state: 'expired',
				error: 'Sesi unggahan kedaluwarsa. Mulai ulang dari nol dengan memilih file kembali.',
				retainUntil: Date.now() + RETENTION_MS
			});
			inMemoryFiles.delete(id);
			return false;
		} finally {
			clearInterval(heartbeat);
			activeTransfers.delete(id);
		}
	});
	if (result === undefined && !activeTransfers.has(id)) inMemoryFiles.delete(id);
	return result ?? false;
}

async function waitForSettled(
	id: string,
	states: TransferState[],
	timeoutMs = 5_000
): Promise<boolean> {
	const deadline = Date.now() + timeoutMs;
	while (Date.now() < deadline) {
		const record = await readRecord(id);
		if (!record || states.includes(record.state)) return true;
		const { promise, resolve } = Promise.withResolvers<void>();
		setTimeout(resolve, 100);
		await promise;
	}
	return false;
}

export async function pauseTransfer(id: string): Promise<boolean> {
	const record = await readRecord(id);
	if (
		!record ||
		record.state === 'completed' ||
		record.state === 'failed' ||
		record.state === 'expired'
	)
		return false;
	const active = activeTransfers.get(id);
	if (active) {
		active.action = 'pause';
		active.controller.abort('pause');
		const settled = await waitForSettled(id, ['paused', 'completed', 'failed', 'expired']);
		const latest = await readRecord(id);
		return settled && (!latest || latest.state === 'paused' || latest.state === 'completed');
	}
	if (record.lockOwner && record.lockOwner !== tabId && (record.lockExpiresAt ?? 0) > Date.now()) {
		channel?.postMessage({ type: 'command', id, action: 'pause' });
		const settled = await waitForSettled(id, ['paused', 'completed', 'failed', 'expired']);
		const latest = await readRecord(id);
		return settled && (!latest || latest.state === 'paused' || latest.state === 'completed');
	}
	await updateRecord(id, { state: 'paused', error: undefined });
	return true;
}

export async function cancelTransfer(
	id: string,
	cancelDriver: TransferCancelDriver
): Promise<boolean> {
	const record = await readRecord(id);
	if (!record || record.state === 'completed' || record.state === 'committing') return false;
	const active = activeTransfers.get(id);
	if (active) {
		if (active.phase === 'committing') return false;
		active.action = 'cancel';
		active.controller.abort('cancel');
		return (await waitForSettled(id, ['completed', 'failed'])) && !(await readRecord(id));
	}
	if (record.lockOwner && record.lockOwner !== tabId && (record.lockExpiresAt ?? 0) > Date.now()) {
		channel?.postMessage({ type: 'command', id, action: 'cancel' });
		return (await waitForSettled(id, ['completed', 'failed'])) && !(await readRecord(id));
	}
	return (
		(await withTransferLock(id, async () => {
			const latest = await readRecord(id);
			if (!latest || latest.state === 'committing' || latest.state === 'completed') return false;
			try {
				if (latest.session) await cancelDriver(latest);
				await deleteRecord(id);
				inMemoryFiles.delete(id);
				return true;
			} catch (error) {
				const { status } = errorDetails(error);
				if (status === 404 || status === 410 || isExpiredError(error)) {
					await deleteRecord(id);
					inMemoryFiles.delete(id);
					return true;
				}
				await updateRecord(id, {
					state: 'failed',
					error: `Pembatalan belum dapat dikonfirmasi: ${errorDetails(error).message}`
				});
				return false;
			}
		})) ?? false
	);
}

export async function restartTransfer(
	id: string,
	cancelDriver: TransferCancelDriver
): Promise<boolean> {
	return (
		(await withTransferLock(id, async () => {
			const record = await readRecord(id);
			if (
				!record ||
				(record.state !== 'expired' &&
					record.state !== 'failed' &&
					record.state !== 'waiting-for-file')
			) {
				return false;
			}
			if (record.session) {
				try {
					await cancelDriver(record);
				} catch (error) {
					const { status } = errorDetails(error);
					if (status !== 404 && status !== 410 && !isExpiredError(error)) {
						await updateRecord(id, {
							error: `Sesi lama belum dapat dibersihkan: ${errorDetails(error).message}`
						});
						return false;
					}
				}
			}
			inMemoryFiles.delete(id);
			await updateRecord(id, {
				state: 'waiting-for-file',
				progress: 0,
				session: undefined,
				error: 'Pilih file kembali untuk memulai unggahan dari nol.',
				errorCode: undefined,
				expiresAt: Date.now() + RETENTION_MS,
				retainUntil: Date.now() + RETENTION_MS * 2
			});
			return true;
		})) ?? false
	);
}

export async function pauseUserTransfers(ownerId: string): Promise<void> {
	const records = await readAll();
	const pending: Promise<boolean>[] = [];
	for (const record of records) {
		if (record.ownerId !== ownerId || !isActiveState(record.state)) continue;
		const active = activeTransfers.get(record.id);
		if (active) {
			active.action = 'pause';
			active.controller.abort('logout');
		} else if (
			record.lockOwner &&
			record.lockOwner !== tabId &&
			(record.lockExpiresAt ?? 0) > Date.now()
		) {
			channel?.postMessage({ type: 'command', id: record.id, action: 'pause' });
		} else {
			await updateRecord(record.id, { state: 'paused', error: undefined });
		}
		pending.push(waitForSettled(record.id, ['paused', 'completed', 'failed', 'expired'], 10_000));
	}
	await Promise.all(pending);
	for (const record of records) {
		if (record.ownerId === ownerId) inMemoryFiles.delete(record.id);
	}
}
export function getInMemoryTransferFile(id: string): File | undefined {
	return inMemoryFiles.get(id);
}

export function isTransferQuotaError(record: TransferRecord): boolean {
	return (
		record.errorCode === 'USER_QUOTA_EXCEEDED' ||
		record.errorCode === 'STORAGE_POOL_FULL' ||
		record.error?.toLowerCase().includes('kuota') === true
	);
}
