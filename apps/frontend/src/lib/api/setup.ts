import { locale } from '$lib/locale.svelte.js';
import type { PlacementStrategy, CloudProvider } from '$lib/types/setup';

const API_BASE = '/api/v1';

export interface StorageNodeResponse {
	path: string;
	kind: string;
	capacity_bytes: number;
	free_bytes: number;
}

export interface ScanNodesResponse {
	nodes: StorageNodeResponse[];
	devices: unknown[];
}

export interface CompleteSetupRequest {
	admin: {
		username: string;
		email: string;
		password: string;
		recovery_key: string; // Client-generated recovery key
	};
	storage_nodes: Array<{
		path: string;
		capacity_bytes: number;
		enabled: boolean;
	}>;
	placement_strategy: PlacementStrategy;
	cloud?: {
		enabled: true;
		provider: CloudProvider;
		endpoint: string;
		bucket: string;
		region?: string;
		access_key: string;
		secret_key: string;
	};
}

export interface CompleteSetupResponse {
	recovery_key: string;
	redirect: string;
}

export interface SetupStatusResponse {
	is_setup_completed: boolean;
}

const SETUP_ERROR_MESSAGES: Record<string, [string, string]> = {
	'setup already completed': ['Penyiapan sudah selesai.', 'Setup has already been completed.'],
	'invalid request body': ['Isi permintaan tidak valid.', 'Invalid request body.'],
	'invalid request': ['Permintaan tidak valid.', 'Invalid request.'],
	'username is required': ['Nama pengguna wajib diisi.', 'Username is required.'],
	'username must be at least 3 characters': [
		'Nama pengguna harus terdiri dari minimal 3 karakter.',
		'Username must be at least 3 characters.'
	],
	'email is required': ['Email wajib diisi.', 'Email is required.'],
	'invalid email': ['Format email tidak valid.', 'Invalid email.'],
	'invalid email format': ['Format email tidak valid.', 'Invalid email format.'],
	'password is required': ['Kata sandi wajib diisi.', 'Password is required.'],
	'password must be at least 8 characters': [
		'Kata sandi harus terdiri dari minimal 8 karakter.',
		'Password must be at least 8 characters.'
	],
	'password must be at least 8 characters long': [
		'Kata sandi harus terdiri dari minimal 8 karakter.',
		'Password must be at least 8 characters long.'
	],
	'recovery key is required': ['Kunci pemulihan wajib diisi.', 'Recovery key is required.'],
	'invalid recovery key': ['Kunci pemulihan tidak valid.', 'Invalid recovery key.'],
	'recovery key must contain 30 alphanumeric characters in six groups of five': [
		'Kunci pemulihan harus berisi 30 karakter alfanumerik dalam enam kelompok, masing-masing lima karakter.',
		'The recovery key must contain 30 alphanumeric characters in six groups of five.'
	],
	'at least one storage node is required': [
		'Pilih setidaknya satu node penyimpanan.',
		'At least one storage node is required.'
	],
	'no storage nodes provided': [
		'Tidak ada node penyimpanan yang diberikan.',
		'No storage nodes were provided.'
	],
	'invalid storage node': ['Node penyimpanan tidak valid.', 'Invalid storage node.'],
	'invalid token': [
		'Sesi Anda tidak valid. Silakan masuk kembali.',
		'Your session is invalid. Please sign in again.'
	],
	'invalid or expired token': [
		'Sesi Anda tidak valid atau sudah berakhir. Silakan masuk kembali.',
		'Your session is invalid or has expired. Please sign in again.'
	],
	'unauthorized': [
		'Sesi Anda telah berakhir. Silakan masuk kembali.',
		'Your session has expired. Please sign in again.'
	],
	'authentication required': ['Silakan masuk untuk melanjutkan.', 'Please sign in to continue.'],
	forbidden: [
		'Anda tidak memiliki izin untuk melakukan tindakan ini.',
		'You do not have permission to perform this action.'
	],
	'failed to fetch setup status': [
		'Gagal memuat status penyiapan.',
		'Failed to load setup status.'
	],
	'failed to scan storage nodes': [
		'Gagal memindai node penyimpanan.',
		'Failed to scan storage nodes.'
	]
};

function localizeSetupError(message: string): string {
	const echoMessage = message
		.trim()
		.match(/^code=\d+,\s*message=(.*?)(?:,\s*internal=.*)?$/i)?.[1]
		?.trim();
	const source = (echoMessage || message.trim()).replace(/^["']|["']$/g, '');
	const key = source
		.toLowerCase()
		.replace(/[_-]+/g, ' ')
		.replace(/\s+/g, ' ')
		.replace(/[.]+$/, '');
	const translated = SETUP_ERROR_MESSAGES[key];
	if (translated) return translated[locale.language === 'id' ? 0 : 1];

	const path =
		source.match(/(?:path|directory)\s+["']([^"']+)["']/i)?.[1] ??
		source
			.match(
				/(?:path|directory)\s+(.+?)(?=\s+(?:does not exist|doesn't exist|not found|no such file|not a directory|is not writable|not writable|cannot write|permission denied))/i
			)?.[1]
			?.replace(/^["']|["']$/g, '') ??
		source.match(/(?:\/[^,\s"'`]+|[A-Za-z]:\\[^,\s"'`]+)/)?.[0];
	if (/(?:directory|(?:local |storage |node )?path).*(?:does not exist|doesn't exist|not found|no such file|no such directory|not a directory|is missing)/i.test(key)) {
		return locale.language === 'id'
			? `Direktori penyimpanan tidak ditemukan atau bukan direktori${path ? `: ${path}` : '.'}`
			: `The storage directory was not found or is not a directory${path ? `: ${path}` : '.'}`;
	}
	if (/(?:directory|(?:local |storage |node )?path).*(?:not writable|cannot write|permission denied|not writeable|must be writable|must be writeable)/i.test(key) ||
		/(?:storage node|local node|node).*(?:not writable|cannot write|permission denied)/i.test(key)) {
		return locale.language === 'id'
			? `Direktori penyimpanan tidak dapat ditulisi${path ? `: ${path}` : '.'}`
			: `The storage directory is not writable${path ? `: ${path}` : '.'}`;
	}
	if (/(?:storage|local|node) path.*(?:required|invalid|empty|must be absolute|not absolute|must point)/i.test(key)) {
		return locale.language === 'id'
			? `Path node penyimpanan tidak valid atau belum lengkap${path ? `: ${path}` : '.'}`
			: `The storage node path is invalid or incomplete${path ? `: ${path}` : '.'}`;
	}
	if (/username.*(?:required|invalid|must|between|length)/i.test(key)) {
		return locale.language === 'id'
			? 'Nama pengguna tidak valid atau belum lengkap.'
			: 'The username is invalid or incomplete.';
	}
	if (/email.*(?:required|invalid|format|must)/i.test(key)) {
		return locale.language === 'id'
			? 'Alamat email tidak valid atau belum lengkap.'
			: 'The email address is invalid or incomplete.';
	}
	if (/password.*(?:required|invalid|at least|minimum|at most|maximum|character|byte)/i.test(key)) {
		return locale.language === 'id'
			? 'Kata sandi tidak valid atau belum memenuhi persyaratan.'
			: 'The password is invalid or does not meet the requirements.';
	}
	if (/recovery key.*(?:required|invalid|format|contain|character|group|must|length|30)/i.test(key)) {
		return locale.language === 'id'
			? 'Kunci pemulihan tidak valid atau formatnya salah.'
			: 'The recovery key is invalid or has the wrong format.';
	}
	if (/(?:storage node|storage nodes|node).*(?:required|invalid|empty|must|at least|missing|configured|capacity)/i.test(key)) {
		return locale.language === 'id'
			? `Konfigurasi node penyimpanan tidak valid atau belum lengkap${path ? `: ${path}` : '.'}`
			: `The storage node configuration is invalid or incomplete${path ? `: ${path}` : '.'}`;
	}
	if (/token.*(?:invalid|expired)|(?:invalid|expired).*token/i.test(key)) {
		return locale.language === 'id'
			? 'Sesi Anda tidak valid atau sudah berakhir. Silakan masuk kembali.'
			: 'Your session is invalid or has expired. Please sign in again.';
	}
	if (/(?:setup )?status.*(?:failed|unavailable)|(?:failed|unavailable).*status/i.test(key)) {
		return locale.language === 'id'
			? 'Gagal memuat status penyiapan.'
			: 'Failed to load setup status.';
	}
	if (/scan.*(?:failed|error)|(?:failed|error).*scan/i.test(key)) {
		return locale.language === 'id'
			? 'Gagal memindai node penyimpanan.'
			: 'Failed to scan storage nodes.';
	}
	if (/invalid.*request|request.*invalid|failed to fetch|request failed|network error/i.test(key)) {
		return locale.language === 'id' ? 'Permintaan ke server tidak valid atau gagal.' : 'The server request is invalid or failed.';
	}

	return message;
}

function setupRequestError(operation: 'status' | 'scan', status: number): string {
	const label =
		operation === 'status'
			? locale.language === 'id'
				? 'Gagal memuat status penyiapan'
				: 'Failed to load setup status'
			: locale.language === 'id'
				? 'Gagal memindai node penyimpanan'
				: 'Failed to scan storage nodes';
	return `${label} (HTTP ${status})`;
}

export async function getSetupStatus(): Promise<SetupStatusResponse> {
	const res = await fetch(`${API_BASE}/setup/status`);
	if (!res.ok) throw new Error(setupRequestError('status', res.status));
	return res.json();
}

export async function scanStorageNodes(): Promise<ScanNodesResponse> {
	const res = await fetch(`${API_BASE}/setup/scan-nodes`);
	if (!res.ok) throw new Error(setupRequestError('scan', res.status));
	return res.json();
}

export async function completeSetup(req: CompleteSetupRequest): Promise<CompleteSetupResponse> {
	const res = await fetch(`${API_BASE}/setup/complete`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(req)
	});
	if (!res.ok) {
		const body = (await res.json().catch(() => null)) as Record<string, unknown> | null;
		const nestedMessage =
			body?.message && typeof body.message === 'object'
				? 'error' in body.message && typeof body.message.error === 'string'
					? body.message.error
					: 'message' in body.message && typeof body.message.message === 'string'
						? body.message.message
						: undefined
				: body?.error && typeof body.error === 'object'
					? 'message' in body.error && typeof body.error.message === 'string'
						? body.error.message
						: 'error' in body.error && typeof body.error.error === 'string'
							? body.error.error
							: undefined
					: undefined;
		const rawMessage =
			typeof body?.message === 'string'
				? body.message
				: typeof body?.error === 'string'
					? body.error
					: nestedMessage;
		const fallback =
			res.status === 409
				? locale.language === 'id'
					? 'Penyiapan sudah selesai.'
					: 'Setup has already been completed.'
				: res.status === 400
					? locale.language === 'id'
						? 'Kesalahan validasi penyiapan.'
						: 'Setup validation error.'
					: res.status === 401 || res.status === 403
						? locale.language === 'id'
							? 'Sesi Anda tidak valid atau Anda tidak memiliki izin untuk melakukan tindakan ini.'
							: 'Your session is invalid or you do not have permission to perform this action.'
						: locale.language === 'id'
							? `Permintaan gagal (HTTP ${res.status})`
							: `Request failed (HTTP ${res.status})`;
		const message = rawMessage ? localizeSetupError(rawMessage) : fallback;
		throw { ...(body ?? {}), status: res.status, message };
	}
	return res.json();
}
