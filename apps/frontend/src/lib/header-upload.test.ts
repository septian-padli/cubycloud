import { beforeEach, describe, expect, it } from 'bun:test';
import { locale } from './locale.svelte.js';
import {
	handleHeaderFilesUpload,
	isQuotaError,
	resolveHeaderUploadTarget,
	type HeaderUploadDrivers,
	type HeaderUploadTarget
} from './header-upload.js';
beforeEach(() => locale.set('id'));

describe('resolveHeaderUploadTarget', () => {
	describe('default destination: My Storage root', () => {
		it('resolves to upload-my-storage on root "/"', () => {
			const result = resolveHeaderUploadTarget('/', null, null);
			expect(result).toEqual({ action: 'upload-my-storage' });
		});

		it('resolves to upload-my-storage on "/drive/home"', () => {
			const result = resolveHeaderUploadTarget('/drive/home', null, null);
			expect(result).toEqual({ action: 'upload-my-storage' });
		});

		it('resolves to upload-my-storage on "/drive/home" even if currentFolderId is passed', () => {
			const result = resolveHeaderUploadTarget('/drive/home', 'some-subfolder', 'editor');
			expect(result).toEqual({ action: 'upload-my-storage' });
		});

		it('resolves to upload-my-storage on "/drive/trash"', () => {
			const result = resolveHeaderUploadTarget('/drive/trash', null, null);
			expect(result).toEqual({ action: 'upload-my-storage' });
		});

		it('resolves to upload-my-storage on "/drive/starred"', () => {
			const result = resolveHeaderUploadTarget('/drive/starred', null, null);
			expect(result).toEqual({ action: 'upload-my-storage' });
		});

		it('resolves to upload-my-storage on "/drive/storage-stats"', () => {
			const result = resolveHeaderUploadTarget('/drive/storage-stats', null, null);
			expect(result).toEqual({ action: 'upload-my-storage' });
		});

		it('resolves to upload-my-storage on admin pages like "/admin/users" and "/admin/storage"', () => {
			expect(resolveHeaderUploadTarget('/admin/users', null, null)).toEqual({
				action: 'upload-my-storage'
			});
			expect(resolveHeaderUploadTarget('/admin/storage', null, null)).toEqual({
				action: 'upload-my-storage'
			});
		});

		it('resolves to upload-my-storage on subfolders of My Storage (always root destination)', () => {
			expect(
				resolveHeaderUploadTarget('/drive/my-storage/subfolder-123', 'subfolder-123', null)
			).toEqual({
				action: 'upload-my-storage'
			});
			expect(
				resolveHeaderUploadTarget('/drive/my-storage', 'subfolder-123', 'editor')
			).toEqual({
				action: 'upload-my-storage'
			});
		});
	});

	describe('Shared With Me listing (root, no folder opened)', () => {
		it('denies upload on "/drive/shared" when no folder is opened', () => {
			const result = resolveHeaderUploadTarget('/drive/shared', null, null);
			expect(result).toEqual({
				action: 'denied',
				reason: 'Anda tidak memiliki izin mengunggah ke folder bersama ini'
			});
		});

		it('denies upload on "/drive/shared/" with trailing slash', () => {
			const result = resolveHeaderUploadTarget('/drive/shared/', null, null);
			expect(result).toEqual({
				action: 'denied',
				reason: 'Anda tidak memiliki izin mengunggah ke folder bersama ini'
			});
		});

		it('denies upload on "/drive/shared?view=grid#top" with query and hash', () => {
			const result = resolveHeaderUploadTarget('/drive/shared?view=grid#top', null, null);
			expect(result).toEqual({
				action: 'denied',
				reason: 'Anda tidak memiliki izin mengunggah ke folder bersama ini'
			});
		});

		it('denies upload on "/drive/shared" when currentFolderId is an empty string', () => {
			const result = resolveHeaderUploadTarget('/drive/shared', '   ', 'editor');
			expect(result).toEqual({
				action: 'denied',
				reason: 'Anda tidak memiliki izin mengunggah ke folder bersama ini'
			});
		});
	});

	describe('Shared With Me actively opened folder', () => {
		it('allows upload directly into shared folder when user has editor permission (via currentFolderId)', () => {
			const result = resolveHeaderUploadTarget('/drive/shared', 'shared-folder-1', 'editor');
			expect(result).toEqual({
				action: 'upload-shared',
				targetFolderId: 'shared-folder-1'
			});
		});

		it('allows upload directly into shared folder when user has editor permission (via pathname)', () => {
			const result = resolveHeaderUploadTarget('/drive/shared/shared-folder-2', null, 'editor');
			expect(result).toEqual({
				action: 'upload-shared',
				targetFolderId: 'shared-folder-2'
			});
		});

		it('decodes URL-encoded folder names from pathname', () => {
			const result = resolveHeaderUploadTarget(
				'/drive/shared/Tim%20Desain%202026',
				null,
				'editor'
			);
			expect(result).toEqual({
				action: 'upload-shared',
				targetFolderId: 'Tim Desain 2026'
			});
		});

		it('denies upload when user has viewer permission in active shared folder', () => {
			const result = resolveHeaderUploadTarget('/drive/shared', 'shared-folder-1', 'viewer');
			expect(result).toEqual({
				action: 'denied',
				targetFolderId: 'shared-folder-1',
				reason: 'Anda tidak memiliki izin mengunggah ke folder bersama ini'
			});
		});

		it('denies upload when user has viewer permission on shared subfolder route', () => {
			const result = resolveHeaderUploadTarget('/drive/shared/folder-sub', null, 'viewer');
			expect(result).toEqual({
				action: 'denied',
				targetFolderId: 'folder-sub',
				reason: 'Anda tidak memiliki izin mengunggah ke folder bersama ini'
			});
		});

		it('denies upload when permission is null in active shared folder', () => {
			const result = resolveHeaderUploadTarget('/drive/shared/folder-sub', null, null);
			expect(result).toEqual({
				action: 'denied',
				targetFolderId: 'folder-sub',
				reason: 'Anda tidak memiliki izin mengunggah ke folder bersama ini'
			});
		});
	});
});

describe('isQuotaError', () => {
	it('detects HTTP 413 as quota error', () => {
		expect(isQuotaError({ status: 413 })).toBe(true);
	});

	it('detects USER_QUOTA_EXCEEDED error code', () => {
		expect(isQuotaError({ code: 'USER_QUOTA_EXCEEDED' })).toBe(true);
	});

	it('detects STORAGE_POOL_FULL error code', () => {
		expect(isQuotaError({ code: 'STORAGE_POOL_FULL' })).toBe(true);
	});

	it('detects error message containing "kuota"', () => {
		expect(isQuotaError(new Error('Batas kuota penyimpanan tercapai'))).toBe(true);
	});

	it('detects error message containing "quota"', () => {
		expect(isQuotaError(new Error('User quota exceeded'))).toBe(true);
	});

	it('returns false for generic or unrelated errors', () => {
		expect(isQuotaError(new Error('Network connection reset'))).toBe(false);
		expect(isQuotaError({ status: 500, message: 'Internal Server Error' })).toBe(false);
		expect(isQuotaError(null)).toBe(false);
		expect(isQuotaError(undefined)).toBe(false);
	});
});

describe('handleHeaderFilesUpload', () => {
	const createFile = (name: string, content = 'content') =>
		new File([content], name, { type: 'text/plain' });

	it('returns immediately without doing work when files array is empty', async () => {
		let myStorageCalled = false;
		let sharedCalled = false;

		const drivers: HeaderUploadDrivers = {
			uploadMyStorage: async () => {
				myStorageCalled = true;
			},
			uploadShared: async () => {
				sharedCalled = true;
			}
		};

		await handleHeaderFilesUpload(
			[],
			{ action: 'upload-my-storage' },
			{},
			drivers
		);

		expect(myStorageCalled).toBe(false);
		expect(sharedCalled).toBe(false);
	});

	it('throws and invokes onError if target action is denied', async () => {
		const target: HeaderUploadTarget = {
			action: 'denied',
			reason: 'Anda tidak memiliki izin mengunggah ke folder bersama ini'
		};

		let errorReceived: unknown = null;
		const files = [createFile('test.txt')];

		await expect(
			handleHeaderFilesUpload(
				files,
				target,
				{
					onError: (err) => {
						errorReceived = err;
					}
				}
			)
		).rejects.toThrow('Anda tidak memiliki izin mengunggah ke folder bersama ini');

		expect(errorReceived).toBeInstanceOf(Error);
		if (errorReceived instanceof Error) {
			expect(errorReceived.message).toBe(
				'Anda tidak memiliki izin mengunggah ke folder bersama ini'
			);
		}
	});

	it('uploads to root of My Storage with parentId null and reports progress', async () => {
		const files = [createFile('file1.txt'), createFile('file2.txt')];
		const target: HeaderUploadTarget = { action: 'upload-my-storage' };

		const uploaded: { name: string; parentId: string | null }[] = [];
		const progressUpdates: number[] = [];
		let successCalled = false;

		const drivers: HeaderUploadDrivers = {
			uploadMyStorage: async (file, parentId, onProgress) => {
				uploaded.push({ name: file.name, parentId });
				onProgress(50);
				onProgress(100);
			}
		};

		await handleHeaderFilesUpload(
			files,
			target,
			{
				onProgress: (p) => progressUpdates.push(p),
				onSuccess: () => {
					successCalled = true;
				}
			},
			drivers
		);

		expect(uploaded).toEqual([
			{ name: 'file1.txt', parentId: null },
			{ name: 'file2.txt', parentId: null }
		]);
		expect(successCalled).toBe(true);
		expect(progressUpdates[0]).toBe(0);
		expect(progressUpdates[progressUpdates.length - 1]).toBe(100);
	});

	it('uploads to shared folder when target is upload-shared', async () => {
		const files = [createFile('shared-file.png')];
		const target: HeaderUploadTarget = {
			action: 'upload-shared',
			targetFolderId: 'folder-shared-99'
		};

		const sharedUploads: { folderId: string; fileName: string }[] = [];
		let successCalled = false;
		const progressUpdates: number[] = [];

		const drivers: HeaderUploadDrivers = {
			uploadShared: async (folderId, file) => {
				sharedUploads.push({ folderId, fileName: file.name });
			}
		};

		await handleHeaderFilesUpload(
			files,
			target,
			{
				onProgress: (p) => progressUpdates.push(p),
				onSuccess: () => {
					successCalled = true;
				}
			},
			drivers
		);

		expect(sharedUploads).toEqual([
			{ folderId: 'folder-shared-99', fileName: 'shared-file.png' }
		]);
		expect(successCalled).toBe(true);
		expect(progressUpdates).toContain(100);
	});

	it('stops immediately when quota error occurs during multi-file upload', async () => {
		const files = [
			createFile('first.txt'),
			createFile('second.txt'),
			createFile('third.txt')
		];
		const target: HeaderUploadTarget = { action: 'upload-my-storage' };

		const uploadedNames: string[] = [];
		let errorReported: unknown = null;

		const drivers: HeaderUploadDrivers = {
			uploadMyStorage: async (file) => {
				if (file.name === 'second.txt') {
					const quotaError = Object.assign(new Error('Kuota My Storage penuh'), { status: 413 });
					throw quotaError;
				}
				uploadedNames.push(file.name);
			}
		};

		await expect(
			handleHeaderFilesUpload(
				files,
				target,
				{
					onError: (err) => {
						errorReported = err;
					}
				},
				drivers
			)
		).rejects.toThrow('Kuota My Storage penuh');

		// First succeeded, second threw quota error, third was never attempted!
		expect(uploadedNames).toEqual(['first.txt']);
		expect(errorReported).toBeDefined();
		expect(isQuotaError(errorReported)).toBe(true);
	});

	it('stops immediately when quota error occurs in shared folder upload', async () => {
		const files = [createFile('first.txt'), createFile('second.txt')];
		const target: HeaderUploadTarget = {
			action: 'upload-shared',
			targetFolderId: 'folder-shared-xyz'
		};

		const uploadedNames: string[] = [];
		const drivers: HeaderUploadDrivers = {
			uploadShared: async (_, file) => {
				if (file.name === 'first.txt') {
					throw { code: 'USER_QUOTA_EXCEEDED', message: 'Kuota My Storage pemilik penuh.' };
				}
				uploadedNames.push(file.name);
			}
		};

		await expect(
			handleHeaderFilesUpload(files, target, {}, drivers)
		).rejects.toMatchObject({ code: 'USER_QUOTA_EXCEEDED' });

		expect(uploadedNames).toEqual([]);
	});
});
