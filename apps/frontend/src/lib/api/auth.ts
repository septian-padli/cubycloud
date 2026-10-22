import { locale } from '$lib/locale.svelte.js';
export const API_BASE = '/api/v1';
const AUTH_ERROR_MESSAGES: Record<string, [string, string]> = {
	'invalid request body': ['Permintaan tidak valid.', 'Invalid request.'],
	'username/email and password are required': [
		'Username/email dan password wajib diisi.',
		'Username/email and password are required.'
	],
	'invalid username or password': [
		'Username atau password salah.',
		'Invalid username or password.'
	],
	'recovery key must contain 30 alphanumeric characters in six groups of five': [
		'Recovery key harus terdiri dari 30 karakter alfanumerik dalam enam kelompok, masing-masing lima karakter.',
		'The recovery key must contain 30 alphanumeric characters in six groups of five.'
	],
	'new password must be at least 8 characters': [
		'Password baru harus terdiri dari minimal 8 karakter.',
		'The new password must be at least 8 characters.'
	],
	'new password must be at most 72 bytes': [
		'Password baru tidak boleh lebih dari 72 byte.',
		'The new password must be no more than 72 bytes.'
	],
	'invalid recovery key': ['Recovery key tidak valid.', 'Invalid recovery key.'],
	'recovery key not set for this user': [
		'Recovery key belum diatur untuk akun ini.',
		'A recovery key has not been set for this account.'
	],
	'current password is required to change email': [
		'Password saat ini wajib diisi untuk mengubah email.',
		'The current password is required to change the email.'
	],
	'incorrect current password': ['Password saat ini salah.', 'The current password is incorrect.'],
	'email already in use': ['Email sudah digunakan.', 'This email is already in use.'],
	'invalid email format': ['Format email tidak valid.', 'The email format is invalid.'],
	'display name must contain 1 to 100 characters': [
		'Nama tampilan harus terdiri dari 1 hingga 100 karakter.',
		'The display name must contain 1 to 100 characters.'
	],
	'account changed concurrently; reload and try again': [
		'Akun berubah secara bersamaan. Muat ulang halaman, lalu coba lagi.',
		'The account changed concurrently. Reload the page and try again.'
	],
	'passwords do not match': ['Password tidak cocok.', 'Passwords do not match.'],
	'user not found': ['Pengguna tidak ditemukan.', 'User not found.'],
	'unauthorized': [
		'Sesi Anda telah berakhir. Silakan masuk kembali.',
		'Your session has expired. Please sign in again.'
	],
	'authentication required': [
		'Silakan masuk untuk melanjutkan.',
		'Please sign in to continue.'
	],
	forbidden: [
		'Anda tidak memiliki izin untuk melakukan tindakan ini.',
		'You do not have permission to perform this action.'
	],
	'super admin access required': [
		'Tindakan ini memerlukan akses Super Admin.',
		'This action requires Super Admin access.'
	],
	'invalid token': [
		'Sesi Anda tidak valid. Silakan masuk kembali.',
		'Your session is invalid. Please sign in again.'
	],
	'invalid or expired token': [
		'Sesi Anda tidak valid atau sudah berakhir. Silakan masuk kembali.',
		'Your session is invalid or has expired. Please sign in again.'
	],
	'invalid token claims': [
		'Sesi Anda tidak valid. Silakan masuk kembali.',
		'Your session is invalid. Please sign in again.'
	],
	'missing user identifier': [
		'Sesi pengguna tidak valid. Silakan masuk kembali.',
		'The user session is invalid. Please sign in again.'
	],
	'account inactive or deleted': [
		'Akun ini tidak aktif atau telah dihapus.',
		'This account is inactive or has been deleted.'
	],
	'invalid or expired session': [
		'Sesi Anda tidak valid atau sudah berakhir. Silakan masuk kembali.',
		'Your session is invalid or has expired. Please sign in again.'
	],
	'failed to generate token': [
		'Gagal membuat sesi masuk. Silakan coba lagi.',
		'Could not create a sign-in session. Please try again.'
	],
	'failed to fetch user': [
		'Gagal memuat profil pengguna. Silakan coba lagi.',
		'Could not load the user profile. Please try again.'
	],
	'failed to fetch user usage': [
		'Gagal memuat informasi penggunaan akun. Silakan coba lagi.',
		'Could not load account usage information. Please try again.'
	],
	'userId is required': ['ID pengguna wajib diisi.', 'The user ID is required.']
};

function localizeAuthError(message: string): string {
	const normalized = message
		.trim()
		.replace(/^code=\d+,\s*message=/i, '')
		.replace(/,\s*internal=.*$/i, '')
		.trim();
	const key = normalized.toLowerCase().replace(/[.]+$/, '');
	return AUTH_ERROR_MESSAGES[key]?.[locale.language === 'id' ? 0 : 1] ?? message;
}

export interface UserResponse {
	id: string;
	username: string;
	email: string;
	display_name?: string;
	role: string;
	quota_bytes: number;
	used_bytes?: number;
}

export interface UpdateProfileRequest {
	display_name?: string;
	email?: string;
	current_password?: string;
}

export interface ChangePasswordRequest {
	current_password: string;
	new_password: string;
	confirm_password: string;
}

export interface LoginResponse {
	user: UserResponse;
	message: string;
}

export interface ResetPasswordRequest {
	username: string;
	recovery_key: string;
	new_password: string;
}

export interface ResetPasswordResponse {
	message: string;
	new_recovery_key: string;
}

class AuthAPI {
	private async request<T>(
		endpoint: string,
		options: RequestInit = {}
	): Promise<{ data?: T; error?: string }> {
		try {
			const response = await fetch(`${API_BASE}${endpoint}`, {
				...options,
				headers: {
					'Content-Type': 'application/json',
					...options.headers
				},
				credentials: 'include'
			});

			if (!response.ok) {
				const errorData = await response.json().catch(() => null);
				const message =
					typeof errorData?.error === 'string'
						? errorData.error
						: typeof errorData?.message === 'string'
							? errorData.message
							: typeof errorData?.message?.error === 'string'
								? errorData.message.error
								: undefined;
				return {
					error:
						typeof message === 'string' && message.trim()
							? localizeAuthError(message)
							: locale.language === 'id'
								? `Permintaan gagal (HTTP ${response.status})`
								: `Request failed (HTTP ${response.status})`
				};
			}

			const data = await response.json();
			return { data };
		} catch {
			return {
				error: locale.language === 'id' ? 'Kesalahan jaringan' : 'Network error'
			};
		}
	}

	async login(identifier: string, password: string): Promise<{ data?: LoginResponse; error?: string }> {
		return this.request<LoginResponse>('/auth/login', {
			method: 'POST',
			body: JSON.stringify({ identifier, password })
		});
	}

	async logout(): Promise<{ error?: string }> {
		return this.request('/auth/logout', {
			method: 'POST'
		});
	}

	async getMe(): Promise<{ data?: UserResponse; error?: string }> {
		return this.request<UserResponse>('/auth/me');
	}

	async resetPassword(
		req: ResetPasswordRequest
	): Promise<{ data?: ResetPasswordResponse; error?: string }> {
		return this.request<ResetPasswordResponse>('/auth/reset-password', {
			method: 'POST',
			body: JSON.stringify(req)
		});
	}

	async updateProfile(
		req: UpdateProfileRequest
	): Promise<{ data?: UserResponse; error?: string }> {
		return this.request<UserResponse>('/auth/profile', {
			method: 'PATCH',
			body: JSON.stringify(req)
		});
	}

	async changePassword(
		req: ChangePasswordRequest
	): Promise<{ data?: { message: string }; error?: string }> {
		return this.request<{ message: string }>('/auth/change-password', {
			method: 'POST',
			body: JSON.stringify(req)
		});
	}

	async getUserProfile(
		userId: string
	): Promise<{ data?: UserResponse; error?: string }> {
		return this.request<UserResponse>(`/auth/profile/${encodeURIComponent(userId)}`);
	}
}

export const authAPI = new AuthAPI();

