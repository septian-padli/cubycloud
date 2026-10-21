export interface SearchItem {
	id: string;
	name: string;
	type: 'file' | 'folder';
	source: 'my-storage' | 'organization' | 'shared';
	size_bytes: number;
	mime_type: string;
	parent_id: string | null;
	location: string;
	updated_at: string;
}

export interface SearchResponse {
	items: SearchItem[];
	total: number;
	limit: number;
	offset: number;
}

export interface SearchOptions {
	limit?: number;
	offset?: number;
	signal?: AbortSignal;
}

/**
 * Searches across My Storage, Organization, and Shared With Me.
 * If query is empty or whitespace only, returns empty results immediately.
 */
export async function searchFiles(
	query: string,
	options: SearchOptions = {}
): Promise<SearchResponse> {
	const trimmed = query.trim();
	const limit = options.limit ?? 25;
	const offset = options.offset ?? 0;

	if (!trimmed) {
		return {
			items: [],
			total: 0,
			limit,
			offset
		};
	}

	const params = new URLSearchParams({
		q: trimmed,
		limit: String(limit),
		offset: String(offset)
	});

	const res = await fetch(`/api/v1/drive/search?${params.toString()}`, {
		headers: {
			Accept: 'application/json'
		},
		credentials: 'include',
		signal: options.signal
	});

	if (!res.ok) {
		const text = await res.text().catch(() => '');
		throw new Error(text || `Pencarian gagal: HTTP ${res.status}`);
	}

	return (await res.json()) as SearchResponse;
}
