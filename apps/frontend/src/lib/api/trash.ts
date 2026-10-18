import type { TrashFileItem, FileEntry } from '$lib/types/drive.js';

const base = '/api/v1/drive/trash';

export interface TrashItemResponse {
	id: string;
	name: string;
	kind: 'file' | 'folder';
	size_bytes: number;
	parent_id: string | null;
	modified_at: string;
	deleted_at: string;
	days_remaining: number;
	original_parent_missing: boolean;
}

export interface TrashListingResponse {
	items: TrashItemResponse[];
}

export interface RestoreResponse {
	item: {
		id: string;
		name: string;
		kind: 'file' | 'folder';
		size_bytes: number;
		parent_id: string | null;
		modified_at: string;
	};
	restored_to_root: boolean;
}

function mapCategory(name: string, kind: 'file' | 'folder'): FileEntry['kind'] {
	if (kind === 'folder') return 'folder';
	const ext = name.split('.').pop()?.toLowerCase() ?? '';
	if (['png', 'jpg', 'jpeg', 'gif', 'webp', 'svg', 'heic'].includes(ext)) return 'image';
	if (['mp4', 'webm', 'mov', 'mkv'].includes(ext)) return 'video';
	if (['mp3', 'wav', 'ogg', 'm4a'].includes(ext)) return 'audio';
	if (['zip', 'rar', '7z', 'tar', 'gz'].includes(ext)) return 'archive';
	if (['txt', 'csv', 'json', 'sql', 'md'].includes(ext)) return 'text';
	return 'document';
}

function formatDeletedDate(isoString: string): string {
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

async function request<T>(path: string, init?: RequestInit): Promise<T> {
	const response = await fetch(`${base}${path}`, { ...init, credentials: 'include' });
	if (response.status === 204) {
		return undefined as T;
	}
	const text = await response.text();
	const body = text ? JSON.parse(text) : undefined;
	if (!response.ok) {
		const err = new Error(body?.error || `HTTP ${response.status}`);
		throw err;
	}
	return body as T;
}

export const trashAPI = {
	async list(): Promise<TrashFileItem[]> {
		const data = await request<TrashListingResponse>('');
		return (data.items ?? []).map((item) => ({
			id: item.id,
			name: item.name,
			kind: mapCategory(item.name, item.kind),
			sizeBytes: item.size_bytes,
			modifiedAt: item.modified_at,
			deletedAt: formatDeletedDate(item.deleted_at),
			daysRemaining: item.days_remaining,
			originalParentId: item.parent_id,
			originalParentMissing: item.original_parent_missing,
			parentId: item.parent_id
		}));
	},
	restore(id: string, toRoot = false): Promise<RestoreResponse> {
		return request<RestoreResponse>(
			`/${encodeURIComponent(id)}/restore${toRoot ? '?to_root=true' : ''}`,
			{
				method: 'POST'
			}
		);
	},
	purgeOne(id: string): Promise<void> {
		return request<void>(`/${encodeURIComponent(id)}`, {
			method: 'DELETE'
		});
	},
	purgeAll(): Promise<void> {
		return request<void>('', {
			method: 'DELETE'
		});
	}
};
