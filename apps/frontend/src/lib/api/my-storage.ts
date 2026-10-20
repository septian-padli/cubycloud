import type { FileEntry, LargestFileItem } from '$lib/types/drive.js';
import { downloadObject, putPresigned, type UploadIntent } from '$lib/api/cloud-transfer.js';

const base = '/api/v1/drive/my-storage';
export interface VFSNode {
	id: string;
	name: string;
	kind: 'file' | 'folder';
	size_bytes: number;
	parent_id: string | null;
	modified_at: string;
	is_starred?: boolean;
}

export type StorageItem = VFSNode;

export interface StorageListing {
	items: StorageItem[];
	breadcrumbs: { id: string; name: string }[];
}

export interface FolderListing {
	items: { id: string; name: string; parent_id: string | null }[];
}

export interface LargestFilesResponse {
	items: {
		id: string;
		name: string;
		kind: string;
		size_bytes: number;
		parent_id: string | null;
		location: string;
		modified_at: string;
	}[];
	total: number;
	has_more: boolean;
	next_offset: number;
}

export interface CategoryUsageResponse {
	total_bytes: number;
	total_gb: number;
	categories: {
		id: 'video' | 'image' | 'document' | 'archive' | 'other' | 'trash' | string;
		label: string;
		bytes: number;
		used_gb: number;
	}[];
}

function formatFileDate(isoString: string): string {
	try {
		const date = new Date(isoString);
		if (isNaN(date.getTime())) return isoString;
		return new Intl.DateTimeFormat('id-ID', {
			day: 'numeric',
			month: 'short',
			year: 'numeric'
		}).format(date);
	} catch {
		return isoString;
	}
}

function storageErrorMessage(error: string, status?: number): string {
	if (error === 'USER_QUOTA_EXCEEDED') return 'Kuota My Storage penuh. Hubungi admin.';
	if (error === 'STORAGE_POOL_FULL') return 'Penyimpanan server penuh.';
	if (error === 'name already in use') return 'Nama sudah digunakan di folder tujuan.';
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
			code?: string;
			existing_id?: string;
			existing_kind?: string;
		};
		error.status = response.status;
		error.suggested_name = body?.suggested_name;
		error.code = body?.error;
		error.existing_id = body?.existing_id;
		error.existing_kind = body?.existing_kind;
		throw error;
	}
	return body as T;
}
async function driveRequest<T>(path: string, init?: RequestInit): Promise<T> {
	const response = await fetch(`/api/v1/drive${path}`, { ...init, credentials: 'include' });
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

export async function moveItem(id: string, targetParentId: string | null): Promise<VFSNode> {
	return driveRequest<VFSNode>(`/items/${encodeURIComponent(id)}/move`, {
		method: 'PATCH',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ target_parent_id: targetParentId })
	});
}

export async function toggleStar(id: string): Promise<{ id: string; is_starred: boolean }> {
	return driveRequest<{ id: string; is_starred: boolean }>(
		`/items/${encodeURIComponent(id)}/star`,
		{
			method: 'POST'
		}
	);
}

export async function getStarredItems(): Promise<VFSNode[]> {
	const res = await driveRequest<{ items: VFSNode[] }>('/starred');
	return res.items ?? [];
}

function encodeTusMetadata(meta: Record<string, string>): string {
	return Object.entries(meta)
		.filter(([_, val]) => val !== undefined && val !== null && val !== '')
		.map(([key, val]) => `${key} ${btoa(unescape(encodeURIComponent(val)))}`)
		.join(',');
}

async function uploadTus(
	file: File,
	parentId: string | null,
	progress: (percent: number) => void
): Promise<StorageItem> {
	const metadata = encodeTusMetadata({
		filename: file.name,
		...(parentId ? { parent_id: parentId } : {})
	});

	const qs = new URLSearchParams({ filename: file.name });
	if (parentId) qs.set('parent_id', parentId);
	const initRes = await fetch(`${base}/upload-tus?${qs.toString()}`, {
		method: 'POST',
		headers: {
			'Upload-Length': file.size.toString(),
			'Upload-Metadata': metadata,
			'Tus-Resumable': '1.0.0'
		},
		credentials: 'include'
	});

	if (!initRes.ok) {
		let body: { error?: string } = {};
		try {
			body = await initRes.json();
		} catch {}
		const err = new Error(storageErrorMessage(body.error ?? '', initRes.status)) as Error & {
			status?: number;
			code?: string;
		};
		err.status = initRes.status;
		err.code = body.error;
		throw err;
	}

	const location = initRes.headers.get('Location');
	if (!location) {
		throw new Error('Header Location tidak ditemukan pada respon TUS');
	}
	const uploadUrl = location.startsWith('http') ? location : location.startsWith('/') ? location : `${base}/${location}`;

	const CHUNK_SIZE = 2 * 1024 * 1024;
	let offset = 0;
	const total = file.size;

	if (total === 0) {
		progress(100);
	}

	let lastPatchRes: { status: number; ok: boolean; headers: { get(name: string): string | null }; text: string } | null = null;
	const MAX_RETRIES = 3;

	while (offset < total || (total === 0 && !lastPatchRes)) {
		const chunk = total === 0 ? new Blob([]) : file.slice(offset, Math.min(offset + CHUNK_SIZE, total));
		let success = false;
		let attempts = 0;

		while (!success && attempts < MAX_RETRIES) {
			attempts++;
			try {
				const { promise: patchPromise, resolve: patchResolve, reject: patchReject } = Promise.withResolvers<{
					status: number;
					ok: boolean;
					headers: { get(name: string): string | null };
					text: string;
				}>();
				const xhr = new XMLHttpRequest();
				xhr.open('PATCH', uploadUrl);
				xhr.setRequestHeader('Upload-Offset', offset.toString());
				xhr.setRequestHeader('Content-Type', 'application/offset+octet-stream');
				xhr.setRequestHeader('Tus-Resumable', '1.0.0');
				xhr.withCredentials = true;
				xhr.upload.onprogress = (event) => {
					if (total > 0 && event.lengthComputable) {
						const currentLoaded = offset + event.loaded;
						progress(Math.min(99, Math.round((currentLoaded / total) * 100)));
					}
				};
				xhr.onload = () => {
					patchResolve({
						status: xhr.status,
						ok: xhr.status >= 200 && xhr.status < 300,
						headers: {
							get: (name: string) => xhr.getResponseHeader(name)
						},
						text: xhr.responseText
					});
				};
				xhr.onerror = () => patchReject(new Error('Network error'));
				xhr.send(chunk);
				const patchRes = await patchPromise;
				if (patchRes.ok) {
					lastPatchRes = patchRes;
					const newOffsetHeader = patchRes.headers.get('Upload-Offset');
					if (newOffsetHeader) {
						offset = Number(newOffsetHeader);
					} else {
						offset += chunk.size;
					}
					success = true;
					if (total > 0) {
						progress(Math.min(100, Math.round((offset / total) * 100)));
					}
					break;
				}

				if (patchRes.status === 409 || patchRes.status >= 500) {
					const headRes = await fetch(uploadUrl, {
						method: 'HEAD',
						headers: { 'Tus-Resumable': '1.0.0' },
						credentials: 'include'
					});
					if (headRes.ok) {
						const remoteOffset = Number(headRes.headers.get('Upload-Offset'));
						if (!isNaN(remoteOffset)) {
							offset = remoteOffset;
							if (total > 0) {
								progress(Math.min(100, Math.round((offset / total) * 100)));
							}
							break;
						}
					}
				}

				if (patchRes.status >= 400 && patchRes.status < 500 && patchRes.status !== 409) {
					let body: { error?: string } = {};
					try {
						body = JSON.parse(patchRes.text);
					} catch {}
					const err = new Error(storageErrorMessage(body.error ?? '', patchRes.status)) as Error & {
						status?: number;
						code?: string;
					};
					err.status = patchRes.status;
					err.code = body.error;
					throw err;
				}
			} catch (err: unknown) {
				if (attempts >= MAX_RETRIES) {
					throw err;
				}
				try {
					const headRes = await fetch(uploadUrl, {
						method: 'HEAD',
						headers: { 'Tus-Resumable': '1.0.0' },
						credentials: 'include'
					});
					if (headRes.ok) {
						const remoteOffset = Number(headRes.headers.get('Upload-Offset'));
						if (!isNaN(remoteOffset)) {
							offset = remoteOffset;
							if (total > 0) {
								progress(Math.min(100, Math.round((offset / total) * 100)));
							}
							break;
						}
					}
				} catch {
					const { promise: delayPromise, resolve: delayResolve } = Promise.withResolvers<void>();
					setTimeout(delayResolve, 500 * attempts);
					await delayPromise;
				}
			}
		}

		if (!success && offset < total) {
			throw new Error('Gagal melanjutkan upload TUS setelah beberapa percobaan');
		}
	}

	if (lastPatchRes) {
		const itemHeader = lastPatchRes.headers.get('Upload-Item');
		if (itemHeader) {
			try {
				const jsonStr = decodeURIComponent(escape(atob(itemHeader)));
				return JSON.parse(jsonStr) as StorageItem;
			} catch {}
		}
		const itemId = lastPatchRes.headers.get('Upload-Item-Id');
		if (itemId) {
			try {
				return await driveRequest<StorageItem>(`/items/${encodeURIComponent(itemId)}`);
			} catch {
				return {
					id: itemId,
					name: file.name,
					kind: 'file',
					size_bytes: file.size,
					parent_id: parentId,
					modified_at: new Date().toISOString()
				};
			}
		}
	}

	return {
		id: '',
		name: file.name,
		kind: 'file',
		size_bytes: file.size,
		parent_id: parentId,
		modified_at: new Date().toISOString()
	};
}

export const myStorageAPI = {
	list(parentId: string | null): Promise<StorageListing> {
		return request(`?${parentId ? `parent_id=${encodeURIComponent(parentId)}` : ''}`);
	},
	folders(): Promise<FolderListing> {
		return request('/folders');
	},
	getItem(id: string): Promise<StorageItem> {
		return request(`/items/${encodeURIComponent(id)}`);
	},
	createFolder(name: string, parentId: string | null, uniqueName = false): Promise<StorageItem> {
		return request('/folders', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({
				name,
				parent_id: parentId,
				...(uniqueName ? { unique_name: true } : {})
			})
		});
	},
	async findOrCreateFolder(
		name: string,
		parentId: string | null,
		knownFolders?: Array<{
			id: string;
			name: string;
			parent_id?: string | null;
			parentId?: string | null;
		}>
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
			method: 'PATCH',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(update)
		});
	},
	duplicate(id: string): Promise<StorageItem> {
		return request(`/${encodeURIComponent(id)}/duplicate`, { method: 'POST' });
	},
	remove(id: string): Promise<void> {
		return request(`/${encodeURIComponent(id)}`, { method: 'DELETE' });
	},
	async upload(
		file: File,
		parentId: string | null,
		progress: (percent: number) => void
	): Promise<StorageItem> {
		const intent = await request<UploadIntent>('/upload-intent', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ filename: file.name, size: file.size, parent_id: parentId, unique_name: true })
		});
		if (intent.placement === 's3') {
			if (!intent.url || !intent.upload_id) throw new Error('Respons cloud upload tidak lengkap');
			try {
				await putPresigned(intent.url, intent.headers ?? {}, file, progress);
				return await driveRequest<StorageItem>('/sync-cloud', {
					method: 'POST',
					headers: { 'Content-Type': 'application/json' },
					body: JSON.stringify({ upload_id: intent.upload_id })
				});
			} catch (error) {
				await request(`/upload-intent/${encodeURIComponent(intent.upload_id)}`, { method: 'DELETE' }).catch(() => {});
				throw error;
			}
		}
		return uploadTus(file, parentId, progress);
	},
	async download(id: string, name: string, folder = false): Promise<void> {
		await downloadObject(base, id, name, folder);
	},
	async downloadBatchZip(ids: string[], onProgress?: (percent: number) => void): Promise<void> {
		const res = await fetch(`${base}/batch/download-zip`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			credentials: 'include',
			body: JSON.stringify({ ids })
		});
		if (!res.ok) {
			throw new Error('Gagal mengunduh berkas ZIP');
		}
		const contentLength = Number(res.headers.get('Content-Length') || 0);
		if (!res.body) {
			const blob = await res.blob();
			const url = URL.createObjectURL(blob);
			const anchor = document.createElement('a');
			anchor.href = url;
			anchor.download = 'cubycloud-archive.zip';
			document.body.append(anchor);
			anchor.click();
			anchor.remove();
			URL.revokeObjectURL(url);
			onProgress?.(100);
			return;
		}

		const reader = res.body.getReader();
		const chunks: Uint8Array[] = [];
		let receivedBytes = 0;

		while (true) {
			const { done, value } = await reader.read();
			if (done) break;
			if (value) {
				chunks.push(value);
				receivedBytes += value.length;
				if (contentLength > 0 && onProgress) {
					onProgress(Math.min(Math.round((receivedBytes / contentLength) * 100), 99));
				} else if (onProgress) {
					onProgress(Math.min(Math.round((receivedBytes / (1024 * 1024)) * 10), 90));
				}
			}
		}

		const blob = new Blob(chunks as BlobPart[], { type: 'application/zip' });
		const url = URL.createObjectURL(blob);
		const anchor = document.createElement('a');
		anchor.href = url;
		anchor.download = 'cubycloud-archive.zip';
		document.body.append(anchor);
		anchor.click();
		anchor.remove();
		URL.revokeObjectURL(url);
		onProgress?.(100);
	},
	toEntry(item: StorageItem): FileEntry {
		const extension = item.name.split('.').pop()?.toLowerCase() ?? '';
		const category: FileEntry['kind'] =
			item.kind === 'folder'
				? 'folder'
				: ['png', 'jpg', 'jpeg', 'gif', 'webp', 'svg', 'heic'].includes(extension)
					? 'image'
					: ['mp4', 'webm', 'mov', 'mkv'].includes(extension)
						? 'video'
						: ['mp3', 'wav', 'ogg', 'm4a'].includes(extension)
							? 'audio'
							: ['zip', 'rar', '7z', 'tar', 'gz'].includes(extension)
								? 'archive'
								: ['txt', 'csv', 'json', 'sql'].includes(extension)
									? 'text'
									: 'document';
		return {
			id: item.id,
			name: item.name,
			kind: category,
			sizeBytes: item.size_bytes,
			parentId: item.parent_id,
			modifiedAt: item.modified_at,
			starred: item.is_starred ?? false
		};
	},
	async largestFiles(limit = 20, offset = 0): Promise<LargestFilesResponse> {
		return request<LargestFilesResponse>(`/largest-files?limit=${limit}&offset=${offset}`);
	},
	async categoryUsage(): Promise<CategoryUsageResponse> {
		return request<CategoryUsageResponse>('/category-usage');
	},
	toLargestFileItem(raw: LargestFilesResponse['items'][number]): LargestFileItem {
		const extension = raw.name.split('.').pop()?.toLowerCase() ?? '';
		const category: FileEntry['kind'] = [
			'png',
			'jpg',
			'jpeg',
			'gif',
			'webp',
			'svg',
			'heic'
		].includes(extension)
			? 'image'
			: ['mp4', 'webm', 'mov', 'mkv'].includes(extension)
				? 'video'
				: ['mp3', 'wav', 'ogg', 'm4a'].includes(extension)
					? 'audio'
					: ['zip', 'rar', '7z', 'tar', 'gz'].includes(extension)
						? 'archive'
						: ['txt', 'csv', 'json', 'sql', 'md'].includes(extension)
							? 'text'
							: 'document';
		return {
			id: raw.id,
			name: raw.name,
			kind: category,
			sizeBytes: raw.size_bytes,
			location: raw.location || 'My Storage',
			modifiedAt: formatFileDate(raw.modified_at)
		};
	},
	downloadUrl(id: string): string {
		return `${base}/${encodeURIComponent(id)}/download`;
	},
	moveItem,
	toggleStar,
	getStarredItems
};
