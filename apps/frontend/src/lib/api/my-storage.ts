import type { FileEntry } from '$lib/types/drive.js';

const base = '/api/v1/drive/my-storage';

export interface StorageItem {
	id: string;
	name: string;
	kind: 'file' | 'folder';
	size_bytes: number;
	parent_id: string | null;
	modified_at: string;
}

export interface StorageListing {
	items: StorageItem[];
	breadcrumbs: { id: string; name: string }[];
}

export interface FolderListing {
	items: { id: string; name: string; parent_id: string | null }[];
}

function storageErrorMessage(error: string, status?: number): string {
	if (error === 'USER_QUOTA_EXCEEDED') return 'Kuota My Storage penuh. Hubungi admin.';
	if (error === 'STORAGE_POOL_FULL') return 'Penyimpanan server penuh.';
	return error || `HTTP ${status ?? 'error'}`;
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
	const response = await fetch(`${base}${path}`, { ...init, credentials: 'include' });
	const text = await response.text();
	const body = text ? JSON.parse(text) : undefined;
	if (!response.ok) {
		const error = new Error(storageErrorMessage(body?.error, response.status)) as Error & {
			status: number;
			suggested_name?: string;
			existing_id?: string;
			existing_kind?: string;
		};
		error.status = response.status;
		error.suggested_name = body?.suggested_name;
		error.existing_id = body?.existing_id;
		error.existing_kind = body?.existing_kind;
		throw error;
	}
	return body as T;
}

export const myStorageAPI = {
	list(parentId: string | null): Promise<StorageListing> {
		return request(`?${parentId ? `parent_id=${encodeURIComponent(parentId)}` : ''}`);
	},
	folders(): Promise<FolderListing> {
		return request('/folders');
	},
	createFolder(name: string, parentId: string | null, uniqueName = false): Promise<StorageItem> {
		return request('/folders', {
			method: 'POST', headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ name, parent_id: parentId, ...(uniqueName ? { unique_name: true } : {}) })
		});
	},
	async findOrCreateFolder(
		name: string,
		parentId: string | null,
		knownFolders?: Array<{ id: string; name: string; parent_id?: string | null; parentId?: string | null }>
	): Promise<StorageItem> {
		if (knownFolders) {
			const targetParent = parentId ?? null;
			const found = knownFolders.find((f) => {
				const fParent = f.parentId !== undefined ? (f.parentId ?? null) : (f.parent_id ?? null);
				return f.name === name && fParent === targetParent;
			});
			if (found) {
				return {
					id: found.id,
					name: found.name,
					kind: 'folder',
					size_bytes: 0,
					parent_id: targetParent,
					modified_at: ''
				};
			}
		}

		try {
			return await this.createFolder(name, parentId, false);
		} catch (error: unknown) {
			const err = error as { status?: number; existing_id?: string; existing_kind?: string };
			if (err?.status === 409) {
				if (err.existing_id && err.existing_kind === 'folder') {
					return {
						id: err.existing_id,
						name,
						kind: 'folder',
						size_bytes: 0,
						parent_id: parentId,
						modified_at: ''
					};
				}
				if (err.existing_kind === 'file') {
					throw error;
				}
				const listing = await this.folders();
				const existing = listing.items.find(
					(f) => f.name === name && (f.parent_id ?? null) === (parentId ?? null)
				);
				if (existing) {
					return {
						id: existing.id,
						name: existing.name,
						kind: 'folder',
						size_bytes: 0,
						parent_id: existing.parent_id,
						modified_at: ''
					};
				}
			}
			throw error;
		}
	},
	mutate(id: string, update: { name?: string; parent_id?: string | null }): Promise<StorageItem> {
		return request(`/${encodeURIComponent(id)}`, {
			method: 'PATCH', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(update)
		});
	},
	duplicate(id: string): Promise<StorageItem> {
		return request(`/${encodeURIComponent(id)}/duplicate`, { method: 'POST' });
	},
	remove(id: string): Promise<void> {
		return request(`/${encodeURIComponent(id)}`, { method: 'DELETE' });
	},
	upload(
		file: File,
		parentId: string | null,
		progress: (percent: number) => void
	): Promise<StorageItem> {
		const { promise, resolve, reject } = Promise.withResolvers<StorageItem>();
		const xhr = new XMLHttpRequest();
		const query = new URLSearchParams({ filename: file.name });
		if (parentId) query.set('parent_id', parentId);
		xhr.open('POST', `${base}/upload?${query}`);
		xhr.withCredentials = true;
		xhr.upload.onprogress = (event) => {
			if (event.lengthComputable && event.total > 0) {
				progress(Math.round((event.loaded / event.total) * 100));
			} else {
				progress(100);
			}
		};
		xhr.onload = () => {
			if (xhr.status >= 200 && xhr.status < 300) {
				try { resolve(JSON.parse(xhr.responseText) as StorageItem); }
				catch { reject(new Error('Respons upload tidak valid')); }
			} else {
				let body: { error?: string } = {};
				try { body = JSON.parse(xhr.responseText); } catch { /* response may be empty */ }
				const err = new Error(storageErrorMessage(body.error ?? '', xhr.status)) as Error & { status?: number; code?: string };
				err.status = xhr.status;
				err.code = body.error;
				reject(err);
			}
		};
		xhr.onerror = () => reject(new Error('Network error'));
		const form = new FormData();
		form.append('file', file, file.name);
		xhr.send(form);
		return promise;
	},
	async download(id: string, name: string): Promise<void> {
		const anchor = document.createElement('a');
		anchor.href = `${base}/${encodeURIComponent(id)}/download`;
		anchor.download = name;
		document.body.append(anchor);
		anchor.click();
		anchor.remove();
	},
	toEntry(item: StorageItem): FileEntry {
		const extension = item.name.split('.').pop()?.toLowerCase() ?? '';
		const category: FileEntry['kind'] = item.kind === 'folder' ? 'folder'
			: ['png', 'jpg', 'jpeg', 'gif', 'webp', 'svg', 'heic'].includes(extension) ? 'image'
			: ['mp4', 'webm', 'mov', 'mkv'].includes(extension) ? 'video'
			: ['mp3', 'wav', 'ogg', 'm4a'].includes(extension) ? 'audio'
			: ['zip', 'rar', '7z', 'tar', 'gz'].includes(extension) ? 'archive'
			: ['txt', 'csv', 'json', 'sql'].includes(extension) ? 'text' : 'document';
		return {
			id: item.id,
			name: item.name,
			kind: category,
			sizeBytes: item.size_bytes,
			parentId: item.parent_id,
			modifiedAt: item.modified_at
		};
	}
};
