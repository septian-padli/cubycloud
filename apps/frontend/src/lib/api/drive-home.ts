import { locale } from '$lib/locale.svelte.js';
import type { ActivityItem, FileKind, QuickAccessItem } from '$lib/types/drive.js';

export interface BackendQuickAccessItem {
	id: string;
	name: string;
	kind: string;
	size_bytes: number;
	source: 'my-storage' | 'organization';
	accessed_at: string;
}

export interface BackendActivityItem {
	id: string;
	action: 'upload' | 'edit';
	item_id: string;
	file_name: string;
	item_kind: string;
	source: 'my-storage' | 'organization';
	created_at: string;
}

export interface BackendHomeResponse {
	quick_access: BackendQuickAccessItem[];
	activities: BackendActivityItem[];
}

export function guessKind(name: string, backendKind: string): FileKind {
	if (backendKind === 'folder') return 'folder';
	const ext = name.split('.').pop()?.toLowerCase() ?? '';
	if (['png', 'jpg', 'jpeg', 'gif', 'webp', 'svg', 'heic'].includes(ext)) return 'image';
	if (['mp4', 'webm', 'mov', 'mkv'].includes(ext)) return 'video';
	if (['mp3', 'wav', 'ogg', 'm4a'].includes(ext)) return 'audio';
	if (['zip', 'rar', '7z', 'tar', 'gz'].includes(ext)) return 'archive';
	if (['pdf', 'doc', 'docx'].includes(ext)) return 'document';
	if (['txt', 'csv', 'json', 'sql', 'md'].includes(ext)) return 'text';
	return 'document';
}

export function formatFileSize(bytes: number, kind: string): string {
	if (kind === 'folder') return 'Folder';
	if (bytes === 0) return '0 B';
	const k = 1024;
	const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
	const i = Math.floor(Math.log(bytes) / Math.log(k));
	const val = bytes / Math.pow(k, i);
	return `${Number.isInteger(val) ? val : val.toFixed(1).replace('.', locale.language === 'id' ? ',' : '.')} ${sizes[i]}`;
}

export function formatTimeAgo(dateStr: string): string {
	const id = locale.language === 'id';
	try {
		const date = new Date(dateStr);
		const now = new Date();
		const diffMs = now.getTime() - date.getTime();
		if (isNaN(diffMs)) return dateStr;
		const diffSec = Math.floor(diffMs / 1000);
		if (diffSec < 60) return id ? 'Baru saja' : 'Just now';
		const diffMin = Math.floor(diffSec / 60);
		if (diffMin < 60) return id ? `${diffMin} menit lalu` : `${diffMin} minutes ago`;
		const diffHour = Math.floor(diffMin / 60);
		if (diffHour < 24) return id ? `${diffHour} jam lalu` : `${diffHour} hours ago`;
		const diffDay = Math.floor(diffHour / 24);
		if (diffDay === 1) return id ? 'Kemarin' : 'Yesterday';
		if (diffDay < 7) return id ? `${diffDay} hari lalu` : `${diffDay} days ago`;
		const diffWeek = Math.floor(diffDay / 7);
		if (diffWeek < 4) return id ? `${diffWeek} minggu lalu` : `${diffWeek} weeks ago`;
		const diffMonth = Math.floor(diffDay / 30);
		if (diffMonth < 12) return id ? `${diffMonth} bulan lalu` : `${diffMonth} months ago`;
		const years = Math.floor(diffDay / 365);
		return id ? `${years} tahun lalu` : `${years} years ago`;
	} catch {
		return dateStr;
	}
}

export async function fetchHomeData(): Promise<{
	quickAccess: QuickAccessItem[];
	activities: ActivityItem[];
}> {
	const resp = await fetch('/api/v1/drive/home', { credentials: 'include' });
	if (!resp.ok) {
		throw new Error(
			locale.language === 'id'
				? `Gagal memuat data beranda: HTTP ${resp.status}`
				: `Failed to load home data (HTTP ${resp.status})`
		);
	}
	const data: BackendHomeResponse = await resp.json();

	const quickAccess: QuickAccessItem[] = (data.quick_access ?? []).map((item) => ({
		id: item.id,
		name: item.name,
		kind: guessKind(item.name, item.kind),
		sizeLabel: formatFileSize(item.size_bytes, item.kind),
		accessedAt: item.accessed_at,
		source: item.source
	}));

	const activities: ActivityItem[] = (data.activities ?? []).map((item) => ({
		id: item.id,
		action: item.action,
		fileName: item.file_name,
		createdAt: item.created_at,
		itemKind: guessKind(item.file_name, item.item_kind),
		source: item.source,
		itemId: item.item_id
	}));

	return { quickAccess, activities };
}

export async function recordItemAccess(
	itemId: string,
	source: 'my-storage' | 'organization' = 'my-storage'
): Promise<void> {
	try {
		await fetch('/api/v1/drive/access', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			credentials: 'include',
			body: JSON.stringify({ item_id: itemId, source })
		});
	} catch (e) {
		console.warn('[Drive Access] Failed to record access:', e);
	}
}
