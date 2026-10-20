import type { FileKind } from '$lib/types/drive.js';

export function detectFileKind(name: string, kind: 'file' | 'folder'): FileKind {
	if (kind === 'folder') return 'folder';
	const ext = name.split('.').pop()?.toLowerCase() ?? '';
	if (['png', 'jpg', 'jpeg', 'gif', 'webp', 'svg', 'heic'].includes(ext)) return 'image';
	if (['mp4', 'webm', 'mov', 'mkv'].includes(ext)) return 'video';
	if (['mp3', 'wav', 'ogg', 'm4a'].includes(ext)) return 'audio';
	if (['zip', 'rar', '7z', 'tar', 'gz'].includes(ext)) return 'archive';
	if (['txt', 'csv', 'json', 'sql', 'md', 'ts', 'js', 'html', 'css'].includes(ext)) return 'text';
	return 'document';
}

export type SharePermission = 'viewer' | 'editor';

export interface ShareUserItem {
	id: string;
	username: string;
	email: string;
}

export interface ShareGrantItem {
	id: string;
	user_id: string;
	username: string;
	email: string;
	permission: SharePermission;
}

export interface ShareLinkItem {
	id: string;
	token: string;
	permission: SharePermission;
	has_password: boolean;
	expires_at: string | null;
	created_at: string;
	url?: string;
}

export interface ItemSharesResponse {
	grants: ShareGrantItem[];
	links: ShareLinkItem[];
}

export interface CreateShareLinkOptions {
	permission: SharePermission;
	password?: string;
	expires_days?: number | null;
}

export interface SharedItem {
	id: string;
	name: string;
	kind: 'file' | 'folder';
	size_bytes: number;
	modified_at: string;
	owner_id: string;
	owner_name: string;
	permission: SharePermission;
	parent_id?: string | null;
}

export interface SharedFolderResponse {
	root_id: string;
	permission: SharePermission;
	items: SharedItem[];
	breadcrumbs: { id: string; name: string }[];
}

export interface PublicShareItem {
	id: string;
	name: string;
	kind: 'file' | 'folder';
	size_bytes: number;
	modified_at: string;
	parent_id?: string | null;
}

export interface PublicShareResponse {
	token: string;
	permission: SharePermission;
	has_password: boolean;
	unlocked: boolean;
	expires_at?: string | null;
	item_name?: string;
	item?: PublicShareItem;
	children?: PublicShareItem[];
}

function shareErrorMessage(error?: string, status?: number): string {
	if (error === 'USER_QUOTA_EXCEEDED') return 'Kuota My Storage pemilik penuh.';
	if (error === 'STORAGE_POOL_FULL') return 'Penyimpanan server penuh.';
	if (error === 'name already in use') return 'Nama sudah digunakan di folder tujuan.';
	if (error === 'PERMISSION_DENIED' || status === 403)
		return 'Anda tidak memiliki izin untuk tindakan ini.';
	if (status === 404) return 'Berkas atau folder tidak ditemukan.';
	if (status === 410) return 'Tautan berbagi sudah kedaluwarsa.';
	return error || `HTTP ${status ?? 'error'}`;
}

async function request<T>(url: string, init?: RequestInit): Promise<T> {
	const response = await fetch(url, { ...init, credentials: 'include' });
	if (response.status === 204) {
		return undefined as T;
	}
	const text = await response.text();
	const body = text ? JSON.parse(text) : undefined;
	if (!response.ok) {
		const err = new Error(shareErrorMessage(body?.error || body?.message, response.status)) as Error & {
			status: number;
		};
		err.status = response.status;
		throw err;
	}
	return body as T;
}

/**
 * Autocomplete pengguna untuk internal user share.
 * GET /api/v1/share/users?q={query}
 */
export async function searchShareUsers(query: string): Promise<ShareUserItem[]> {
	const res = await request<{ users: ShareUserItem[] }>(
		`/api/v1/share/users?q=${encodeURIComponent(query)}`
	);
	return res.users ?? [];
}

/**
 * Mengambil daftar grants dan links aktif untuk suatu item drive.
 * GET /api/v1/drive/items/:id/shares
 */
export async function getItemShares(itemId: string): Promise<ItemSharesResponse> {
	return request<ItemSharesResponse>(`/api/v1/drive/items/${encodeURIComponent(itemId)}/shares`);
}

/**
 * Menyimpan/memperbarui daftar user grants.
 * PUT /api/v1/drive/items/:id/shares/users
 */
export async function updateItemGrants(
	itemId: string,
	grants: { user_id: string; permission: SharePermission }[]
): Promise<{ grants: ShareGrantItem[] }> {
	return request<{ grants: ShareGrantItem[] }>(
		`/api/v1/drive/items/${encodeURIComponent(itemId)}/shares/users`,
		{
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ grants })
		}
	);
}

/**
 * Menghapus grant seorang user dari item.
 * DELETE /api/v1/drive/items/:id/shares/users/:userId
 */
export async function deleteUserGrant(itemId: string, userId: string): Promise<void> {
	return request<void>(
		`/api/v1/drive/items/${encodeURIComponent(itemId)}/shares/users/${encodeURIComponent(userId)}`,
		{
			method: 'DELETE'
		}
	);
}

/**
 * Membuat tautan berbagi baru.
 * POST /api/v1/drive/items/:id/shares/links
 */
export async function createShareLink(
	itemId: string,
	options: CreateShareLinkOptions
): Promise<{ link: ShareLinkItem }> {
	return request<{ link: ShareLinkItem }>(
		`/api/v1/drive/items/${encodeURIComponent(itemId)}/shares/links`,
		{
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify(options)
		}
	);
}

/**
 * Menghapus/mencabut tautan berbagi.
 * DELETE /api/v1/drive/items/:id/shares/links/:linkId
 */
export async function deleteShareLink(itemId: string, linkId: string): Promise<void> {
	return request<void>(
		`/api/v1/drive/items/${encodeURIComponent(itemId)}/shares/links/${encodeURIComponent(linkId)}`,
		{
			method: 'DELETE'
		}
	);
}

/**
 * Mengambil daftar item yang dibagikan kepada pengguna yang sedang login.
 * GET /api/v1/drive/shared
 */
export async function getSharedWithMe(): Promise<{ items: SharedItem[] }> {
	return request<{ items: SharedItem[] }>('/api/v1/drive/shared');
}

/**
 * Mengambil isi subfolder dari folder yang dibagikan.
 * GET /api/v1/drive/shared/folders/:id
 */
export async function getSharedFolder(folderId: string): Promise<SharedFolderResponse> {
	return request<SharedFolderResponse>(
		`/api/v1/drive/shared/folders/${encodeURIComponent(folderId)}`
	);
}

/**
 * Mengunggah berkas ke folder yang dibagikan (permission editor).
 * POST /api/v1/drive/shared/folders/:id/upload
 */
export async function uploadSharedFile(folderId: string, file: File): Promise<SharedItem> {
	const formData = new FormData();
	formData.append('file', file);
	return request<SharedItem>(
		`/api/v1/drive/shared/folders/${encodeURIComponent(folderId)}/upload`,
		{
			method: 'POST',
			body: formData
		}
	);
}

/**
 * Membuat folder baru di dalam folder yang dibagikan (permission editor).
 * POST /api/v1/drive/shared/folders/:id/folders
 */
export async function createSharedFolder(folderId: string, name: string): Promise<SharedItem> {
	return request<SharedItem>(
		`/api/v1/drive/shared/folders/${encodeURIComponent(folderId)}/folders`,
		{
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ name })
		}
	);
}

/**
 * Mengubah nama berkas/subfolder di dalam folder yang dibagikan (permission editor).
 * PATCH /api/v1/drive/shared/items/:id
 */
export async function renameSharedItem(itemId: string, name: string): Promise<SharedItem> {
	return request<SharedItem>(`/api/v1/drive/shared/items/${encodeURIComponent(itemId)}`, {
		method: 'PATCH',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ name })
	});
}

/**
 * Menghapus berkas/subfolder di dalam folder yang dibagikan (permission editor).
 * DELETE /api/v1/drive/shared/items/:id
 */
export async function deleteSharedItem(itemId: string): Promise<void> {
	return request<void>(`/api/v1/drive/shared/items/${encodeURIComponent(itemId)}`, {
		method: 'DELETE'
	});
}

/**
 * Memindahkan berkas/subfolder di dalam shared tree (permission editor).
 * PATCH /api/v1/drive/shared/items/:id/move
 */
export async function moveSharedItem(
	itemId: string,
	targetParentId: string
): Promise<SharedItem> {
	return request<SharedItem>(`/api/v1/drive/shared/items/${encodeURIComponent(itemId)}/move`, {
		method: 'PATCH',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ target_parent_id: targetParentId })
	});
}

/**
 * Mengunduh berkas shared.
 * GET /api/v1/drive/shared/items/:id/download
 */
export function downloadSharedItem(itemId: string, name: string): void {
	const anchor = document.createElement('a');
	anchor.href = `/api/v1/drive/shared/items/${encodeURIComponent(itemId)}/download`;
	anchor.download = name;
	document.body.append(anchor);
	anchor.click();
	anchor.remove();
}

/**
 * Mengunduh zip folder shared.
 * GET /api/v1/drive/shared/folders/:id/download-zip
 */
export function downloadSharedFolderZip(folderId: string, name: string): void {
	const anchor = document.createElement('a');
	anchor.href = `/api/v1/drive/shared/folders/${encodeURIComponent(folderId)}/download-zip`;
	anchor.download = `${name}.zip`;
	document.body.append(anchor);
	anchor.click();
	anchor.remove();
}

// -------------------------------------------------------------
// Public Share Link APIs
// -------------------------------------------------------------

/**
 * Mengambil detail public share link berdasarkan token.
 * GET /api/v1/s/:token
 */
export async function getPublicShare(
	token: string,
	folderId?: string | null
): Promise<PublicShareResponse> {
	const qs = new URLSearchParams();
	if (folderId) qs.set('folder_id', folderId);
	const query = qs.toString() ? `?${qs.toString()}` : '';
	return request<PublicShareResponse>(`/api/v1/s/${encodeURIComponent(token)}${query}`);
}

/**
 * Membuka proteksi password untuk public share link.
 * POST /api/v1/s/:token/unlock
 */
export function unlockPublicShare(token: string, password: string): Promise<{ success: boolean }> {
	return request<{ success: boolean }>(`/api/v1/s/${encodeURIComponent(token)}/unlock`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ password })
	});
}
/**
 * Mengunduh berkas public share langsung.
 * GET /api/v1/s/:token/download
 */
export function downloadPublicFile(token: string, fileId?: string | null, name?: string): void {
	const anchor = document.createElement('a');
	const qs = fileId ? `?file_id=${encodeURIComponent(fileId)}` : '';
	anchor.href = `/api/v1/s/${encodeURIComponent(token)}/download${qs}`;
	if (name) anchor.download = name;
	document.body.append(anchor);
	anchor.click();
	anchor.remove();
}

/**
 * Mengunduh zip folder public share langsung.
 * GET /api/v1/s/:token/download-zip
 */
export function downloadPublicZip(token: string, folderId?: string | null, name?: string): void {
	const anchor = document.createElement('a');
	const qs = folderId ? `?folder_id=${encodeURIComponent(folderId)}` : '';
	anchor.href = `/api/v1/s/${encodeURIComponent(token)}/download-zip${qs}`;
	anchor.download = name ? `${name}.zip` : 'archive.zip';
	document.body.append(anchor);
	anchor.click();
	anchor.remove();
}

/**
 * Mengunggah berkas ke public link folder (izin editor + logged in).
 * POST /api/v1/s/:token/upload
 */
export async function uploadPublicFile(
	token: string,
	file: File,
	folderId?: string | null
): Promise<PublicShareItem> {
	const formData = new FormData();
	formData.append('file', file);
	const qs = folderId ? `?parent_id=${encodeURIComponent(folderId)}` : '';
	return request<PublicShareItem>(`/api/v1/s/${encodeURIComponent(token)}/upload${qs}`, {
		method: 'POST',
		body: formData
	});
}

/**
 * Membuat folder di public link (izin editor + logged in).
 * POST /api/v1/s/:token/folders
 */
export async function createPublicFolder(
	token: string,
	name: string,
	folderId?: string | null
): Promise<PublicShareItem> {
	return request<PublicShareItem>(`/api/v1/s/${encodeURIComponent(token)}/folders`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ name, parent_id: folderId ?? null })
	});
}

/**
 * Mengubah nama berkas/folder di public link (izin editor + logged in).
 * PATCH /api/v1/s/:token/items/:itemId
 */
export async function renamePublicItem(
	token: string,
	itemId: string,
	name: string
): Promise<PublicShareItem> {
	return request<PublicShareItem>(
		`/api/v1/s/${encodeURIComponent(token)}/items/${encodeURIComponent(itemId)}`,
		{
			method: 'PATCH',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ name })
		}
	);
}

/**
 * Menghapus berkas/folder di public link (izin editor + logged in).
 * DELETE /api/v1/s/:token/items/:itemId
 */
export async function deletePublicItem(token: string, itemId: string): Promise<void> {
	return request<void>(
		`/api/v1/s/${encodeURIComponent(token)}/items/${encodeURIComponent(itemId)}`,
		{
			method: 'DELETE'
		}
	);
}
