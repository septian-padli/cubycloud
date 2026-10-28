import { locale } from '$lib/locale.svelte.js';
import { myStorageAPI } from '$lib/api/my-storage.js';
import { uploadSharedFile } from '$lib/api/share.js';
import type { TransferRecord } from '$lib/transfer-queue.svelte.js';
const tx = (id: string, en: string): string => (locale.language === 'id' ? id : en);

export type HeaderUploadAction = 'upload-my-storage' | 'upload-shared' | 'denied';

export interface HeaderUploadTarget {
	action: HeaderUploadAction;
	targetFolderId?: string;
	reason?: string;
}

export interface HeaderUploadCallbacks {
	onProgress?: (p: number) => void;
	onSuccess?: () => void;
	onError?: (err: unknown) => void;
}

export interface HeaderUploadDrivers {
	uploadMyStorage?: (
		file: File,
		parentId: string | null,
		progress: (percent: number) => void
	) => Promise<unknown>;
	uploadShared?: (folderId: string, file: File) => Promise<unknown>;
	uploadQueue?: (
		ownerId: string,
		files: File[],
		progress: (percent: number) => void
	) => Promise<void>;
}

/**
 * Resolves the destination for files uploaded via the consistent Header Upload button.
 *
 * Rules:
 * - Default destination: root of My Storage (parentId: null), regardless of current page
 *   (e.g. /drive/home, /drive/trash, /admin/..., or subfolders of My Storage).
 * - Exception: inside an actively opened Shared With Me folder (/drive/shared/[folderId] or active shared folder context):
 *   - If user has effective Editor permission in that folder: upload directly into that folder.
 *   - If user has Viewer permission, or on the Shared With Me listing (/drive/shared) with no folder opened:
 *     show an informative denial dialog ("Anda tidak memiliki izin mengunggah ke folder bersama ini").
 *     Never silently redirect to My Storage!
 */
export function resolveHeaderUploadTarget(
	pathname: string,
	currentFolderId: string | null,
	permission: 'viewer' | 'editor' | null
): HeaderUploadTarget {
	const sanitized = pathname.split('?')[0].split('#')[0].replace(/\/+$/, '') || '/';
	const isSharedPath = sanitized === '/drive/shared' || sanitized.startsWith('/drive/shared/');

	if (!isSharedPath) {
		return { action: 'upload-my-storage' };
	}

	let folderFromPath: string | null = null;
	if (sanitized.startsWith('/drive/shared/')) {
		const sub = sanitized.slice('/drive/shared/'.length);
		if (sub.length > 0) {
			folderFromPath = decodeURIComponent(sub.split('/')[0]);
		}
	}

	const effectiveFolderId =
		(currentFolderId && currentFolderId.trim().length > 0 ? currentFolderId.trim() : null) ??
		(folderFromPath && folderFromPath.trim().length > 0 ? folderFromPath.trim() : null);

	if (!effectiveFolderId) {
		return {
			action: 'denied',
			reason: tx(
				'Anda tidak memiliki izin mengunggah ke folder bersama ini',
				'You do not have permission to upload to this shared folder.'
			)
		};
	}

	if (permission === 'editor') {
		return {
			action: 'upload-shared',
			targetFolderId: effectiveFolderId
		};
	}

	return {
		action: 'denied',
		targetFolderId: effectiveFolderId,
		reason: tx(
			'Anda tidak memiliki izin mengunggah ke folder bersama ini',
			'You do not have permission to upload to this shared folder.'
		)
	};
}
/**
 * Checks whether an error represents a storage quota limitation.
 */
export function isQuotaError(err: unknown): boolean {
	if (!err || typeof err !== 'object') return false;

	const status = 'status' in err && typeof err.status === 'number' ? err.status : undefined;
	const code = 'code' in err && typeof err.code === 'string' ? err.code : undefined;
	const message = 'message' in err && typeof err.message === 'string' ? err.message : undefined;

	return Boolean(
		status === 413 ||
		code === 'USER_QUOTA_EXCEEDED' ||
		code === 'STORAGE_POOL_FULL' ||
		(message &&
			(message.toLowerCase().includes('kuota') ||
				message.toLowerCase().includes('quota') ||
				message.includes('USER_QUOTA_EXCEEDED') ||
				message.includes('STORAGE_POOL_FULL')))
	);
}

async function uploadThroughTransferQueue(
	ownerId: string,
	files: File[],
	onProgress?: (percent: number) => void
): Promise<void> {
	// This Svelte-rune module is needed only by the browser's default transfer-queue path.
	const {
		enqueueTransfers,
		failPendingTransfers,
		isTransferQuotaError,
		loadTransferQueue,
		runTransfer,
		transferQueueState
	} = await import('$lib/transfer-queue.svelte.js');
	await loadTransferQueue(ownerId, 'my-storage');
	const ids = await enqueueTransfers(
		ownerId,
		'my-storage',
		files.map((file) => ({
			name: file.name,
			relativePath: file.name,
			parentId: null,
			file
		}))
	);
	const driver = (
		file: File,
		record: TransferRecord,
		options: Parameters<typeof myStorageAPI.uploadResumable>[2]
	) => myStorageAPI.uploadResumable(file, record, options);
	const cancelDriver = (record: TransferRecord) => myStorageAPI.cancelResumable(record);
	let firstError: Error | null = null;

	for (let index = 0; index < ids.length; index += 1) {
		const id = ids[index];
		const timer = setInterval(() => {
			const progress = ids.reduce((sum, transferId) => {
				const record = transferQueueState.items.find((item) => item.id === transferId);
				return sum + (record?.progress ?? 0);
			}, 0);
			onProgress?.(Math.round(progress / ids.length));
		}, 100);

		try {
			await runTransfer(id, files[index], driver, cancelDriver);
		} finally {
			clearInterval(timer);
		}

		const current = transferQueueState.items.find((item) => item.id === id);
		if (current?.state === 'failed' || current?.state === 'expired') {
			firstError ??= Object.assign(
				new Error(current.error || tx('Unggahan gagal', 'Upload failed')),
				{
					code: current.errorCode
				}
			);
			if (isTransferQuotaError(current)) {
				await failPendingTransfers(
					ids.slice(index + 1),
					tx('Dibatalkan: kuota penyimpanan tidak cukup.', 'Cancelled: not enough storage quota.')
				);
				break;
			}
		}
	}

	if (firstError) throw firstError;
	onProgress?.(100);
}

/**
 * Executes file uploads based on resolved destination target.
 * Supports multiple files, progress reporting, quota abort, and truthful non-resumable transport for shared folders.
 */
export async function handleHeaderFilesUpload(
	files: File[],
	target: HeaderUploadTarget,
	callbacks: HeaderUploadCallbacks = {},
	drivers: HeaderUploadDrivers = {},
	ownerId?: string
): Promise<void> {
	if (!files || files.length === 0) {
		return;
	}

	if (target.action === 'denied') {
		const denialError = new Error(
			target.reason ||
				tx(
					'Anda tidak memiliki izin mengunggah ke folder bersama ini',
					'You do not have permission to upload to this shared folder.'
				)
		);
		callbacks.onError?.(denialError);
		throw denialError;
	}

	if (target.action === 'upload-my-storage' && ownerId && !drivers.uploadMyStorage) {
		try {
			await (drivers.uploadQueue ?? uploadThroughTransferQueue)(ownerId, files, (progress) =>
				callbacks.onProgress?.(progress)
			);
			callbacks.onSuccess?.();
			return;
		} catch (err) {
			callbacks.onError?.(err);
			throw err;
		}
	}

	const uploadMyStorage =
		drivers.uploadMyStorage ??
		((file, parentId, onProgress) => myStorageAPI.upload(file, parentId, onProgress));
	const uploadShared =
		drivers.uploadShared ?? ((folderId, file) => uploadSharedFile(folderId, file));

	const totalFiles = files.length;
	let lastError: unknown = null;
	let failureCount = 0;
	let quotaAborted = false;

	callbacks.onProgress?.(0);

	for (let i = 0; i < totalFiles; i++) {
		const file = files[i];
		try {
			if (target.action === 'upload-shared') {
				if (!target.targetFolderId) {
					throw new Error(
						tx(
							'ID folder tujuan tidak ditemukan untuk folder bersama',
							'The destination folder ID was not found for this shared folder.'
						)
					);
				}
				await uploadShared(target.targetFolderId, file);
				const overallPercent = Math.round(((i + 1) / totalFiles) * 100);
				callbacks.onProgress?.(overallPercent);
			} else {
				// My Storage: always upload to root (parentId: null)
				await uploadMyStorage(file, null, (filePercent) => {
					const overallPercent = Math.round(((i + filePercent / 100) / totalFiles) * 100);
					callbacks.onProgress?.(overallPercent);
				});
				const fileDonePercent = Math.round(((i + 1) / totalFiles) * 100);
				callbacks.onProgress?.(fileDonePercent);
			}
		} catch (err: unknown) {
			lastError = err;
			failureCount++;
			if (isQuotaError(err)) {
				quotaAborted = true;
				break;
			}
		}
	}

	if (quotaAborted || failureCount > 0) {
		callbacks.onError?.(lastError);
		throw lastError;
	}

	callbacks.onSuccess?.();
}
