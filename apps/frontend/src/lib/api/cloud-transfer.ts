export interface UploadIntent {
	placement: 'local' | 's3';
	upload_id?: string;
	url?: string;
	method?: 'PUT';
	headers?: Record<string, string>;
	expires_at?: string;
}

/** Upload directly to the presigned object URL, without application cookies or multipart encoding. */
export function putPresigned(
	url: string,
	headers: Record<string, string>,
	file: File,
	progress: (percent: number) => void
): Promise<void> {
	const { promise, resolve, reject } = Promise.withResolvers<void>();
	const xhr = new XMLHttpRequest();
	xhr.open('PUT', url);
	for (const [name, value] of Object.entries(headers)) xhr.setRequestHeader(name, value);
	xhr.upload.onprogress = (event) => {
		if (event.lengthComputable && event.total > 0) progress(Math.round(event.loaded / event.total * 100));
	};
	xhr.onload = () => xhr.status >= 200 && xhr.status < 300
		? resolve()
		: reject(new Error(`Upload ke cloud gagal (HTTP ${xhr.status})`));
	xhr.onerror = () => reject(new Error('Koneksi ke cloud storage gagal'));
	xhr.send(file);
	return promise;
}

/** Keep the browser on the direct S3 path for files, and preserve the authenticated ZIP route for folders. */
export async function downloadObject(base: string, id: string, name: string, folder = false): Promise<void> {
	let url = `${base}/${encodeURIComponent(id)}/download`;
	if (!folder) {
		const response = await fetch(`${base}/${encodeURIComponent(id)}/download-url`, { credentials: 'include' });
		if (!response.ok) {
			const body = await response.json().catch(() => null);
			throw new Error(body?.error ?? 'Gagal menyiapkan unduhan');
		}
		const signed = await response.json() as { url: string | null };
		if (signed.url) url = signed.url;
	}
	const anchor = document.createElement('a');
	anchor.href = url;
	anchor.download = name;
	document.body.append(anchor);
	anchor.click();
	anchor.remove();
}
