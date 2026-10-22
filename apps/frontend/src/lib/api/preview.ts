import { locale } from '$lib/locale.svelte.js';
export type PreviewSource = 'my-storage' | 'organization' | 'shared' | 'public';

/** The download endpoints enforce permission and serve inline only for ?preview=1. */
export function previewUrl(source: PreviewSource, id: string, token?: string, rootFile = false): string {
	if (source === 'public') {
		if (!token)
			throw new Error(
				locale.language === 'id'
					? 'Token tautan berbagi tidak tersedia'
					: 'Share link token is unavailable'
			);
		const url = `/api/v1/s/${encodeURIComponent(token)}/download`;
		return `${url}?${rootFile ? '' : `file_id=${encodeURIComponent(id)}&`}preview=1`;
	}
	if (source === 'shared')
		return `/api/v1/drive/shared/items/${encodeURIComponent(id)}/download?preview=1`;
	return `/api/v1/drive/${source}/${encodeURIComponent(id)}/download?preview=1`;
}

const MAX_TEXT_BYTES = 1024 * 1024;

/** Stream with a hard limit even when Content-Length is absent or incorrect. */
export async function getPreviewText(url: string, signal: AbortSignal): Promise<string> {
	const response = await fetch(url, { credentials: 'include', signal });
	if (!response.ok)
		throw new Error(
			locale.language === 'id' ? 'Gagal memuat pratinjau berkas' : 'Failed to load file preview'
		);
	if (Number(response.headers.get('Content-Length')) > MAX_TEXT_BYTES) {
		await response.body?.cancel();
		throw new Error(
			locale.language === 'id'
				? 'Berkas terlalu besar untuk pratinjau teks (maksimum 1 MB)'
				: 'File is too large for text preview (maximum 1 MB)'
		);
	}
	if (!response.body)
		throw new Error(
			locale.language === 'id' ? 'Pratinjau teks tidak tersedia' : 'Text preview is unavailable'
		);
	const reader = response.body.getReader();
	const decoder = new TextDecoder();
	const chunks: string[] = [];
	let size = 0;
	try {
		while (true) {
			const { value, done } = await reader.read();
			if (done) return chunks.join('') + decoder.decode();
			size += value.byteLength;
			if (size > MAX_TEXT_BYTES)
				throw new Error(
					locale.language === 'id'
						? 'Berkas terlalu besar untuk pratinjau teks (maksimum 1 MB)'
						: 'File is too large for text preview (maximum 1 MB)'
				);
			chunks.push(decoder.decode(value, { stream: true }));
		}
	} finally {
		await reader.cancel().catch(() => {});
		reader.releaseLock();
	}
}
