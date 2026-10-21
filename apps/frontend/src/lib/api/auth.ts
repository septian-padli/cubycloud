export const API_BASE = '/api/v1';

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
				return {
					error: errorData?.error || errorData?.message || `HTTP ${response.status}: ${response.statusText}`
				};
			}

			const data = await response.json();
			return { data };
		} catch (err) {
			return {
				error: err instanceof Error ? err.message : 'Network error'
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

