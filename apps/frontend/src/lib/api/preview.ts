export type PreviewSource = 'my-storage' | 'organization' | 'shared' | 'public';

/** The download endpoints enforce permission and serve inline only for ?preview=1. */
export function previewUrl(source: PreviewSource, id: string, token?: string, rootFile = false): string {
	if (source === 'public') {
		if (!token) throw new Error('Token tautan berbagi tidak tersedia');
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
	if (!response.ok) throw new Error('Gagal memuat pratinjau berkas');
	if (Number(response.headers.get('Content-Length')) > MAX_TEXT_BYTES) {
		await response.body?.cancel();
		throw new Error('Berkas terlalu besar untuk pratinjau teks (maksimum 1 MB)');
	}
	if (!response.body) throw new Error('Pratinjau teks tidak tersedia');
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
				throw new Error('Berkas terlalu besar untuk pratinjau teks (maksimum 1 MB)');
			chunks.push(decoder.decode(value, { stream: true }));
		}
	} finally {
		await reader.cancel().catch(() => {});
		reader.releaseLock();
	}
}
