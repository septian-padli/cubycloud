import { locale } from '$lib/locale.svelte.js';
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
		: reject(
				new Error(
					locale.language === 'id'
						? `Unggah ke cloud gagal (HTTP ${xhr.status})`
						: `Cloud upload failed (HTTP ${xhr.status})`
				)
			);
	xhr.onerror = () =>
		reject(
			new Error(
				locale.language === 'id'
					? 'Koneksi ke cloud storage gagal'
					: 'Connection to cloud storage failed'
			)
		);
	xhr.send(file);
	return promise;
}

/** Keep the browser on the direct S3 path for files, and preserve the authenticated ZIP route for folders. */
export async function downloadObject(base: string, id: string, name: string, folder = false): Promise<void> {
	let url = `${base}/${encodeURIComponent(id)}/download`;
	if (!folder) {
		const response = await fetch(`${base}/${encodeURIComponent(id)}/download-url`, { credentials: 'include' });
		if (!response.ok) {
			throw new Error(
				locale.language === 'id'
					? `Gagal menyiapkan unduhan (HTTP ${response.status})`
					: `Failed to prepare download (HTTP ${response.status})`
			);
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

const activeZipFrames = new Set<HTMLIFrameElement>();
let zipDownloadCounter = 0;

/** Let the browser stream POSTed ZIP bytes to Downloads; fetch/Blob would buffer the whole archive. */
export function downloadBatchZip(base: string, ids: string[], onError?: (message: string) => void): 'started' {
	for (const f of activeZipFrames) {
		if (f.isConnected === false) activeZipFrames.delete(f);
	}
	const frame = document.createElement('iframe');
	frame.name = activeZipFrames.size === 0 ? 'cubycloud-zip-download' : `cubycloud-zip-download-${++zipDownloadCounter}`;
	activeZipFrames.add(frame);
	frame.title = locale.language === 'id' ? 'Unduhan ZIP' : 'ZIP download';
	frame.hidden = true;
	document.body.append(frame);
	frame.onload = () => {
		if (frame.contentDocument?.URL === 'about:blank') return;
		const body = frame.contentDocument?.body?.textContent?.trim();
		if (!body) return;
		let message = locale.language === 'id' ? 'Gagal mengunduh berkas ZIP' : 'Failed to download ZIP file';
		try {
			const payload = JSON.parse(body) as { error?: unknown };
			if (typeof payload.error === 'string' && payload.error) message = payload.error;
		} catch {
			// ZIP responses are not JSON; only JSON errors can be reported here.
		}
		onError?.(message);
	};
	const form = document.createElement('form');
	form.action = `${base}/batch/download-zip`;
	form.method = 'POST';
	form.enctype = 'application/x-www-form-urlencoded';
	form.target = frame.name;
	form.style.display = 'none';
	for (const id of ids) {
		const input = document.createElement('input');
		input.type = 'hidden';
		input.name = 'ids';
		input.value = id;
		form.append(input);
	}
	document.body.append(form);
	try {
		form.submit();
	} finally {
		form.remove();
	}
	// Native downloads have no completion or byte-progress event for this POST.
	return 'started';
}
