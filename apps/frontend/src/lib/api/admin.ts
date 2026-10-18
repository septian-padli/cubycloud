export const API_BASE = '/api/v1';

export interface QuotaPolicy {
	mode: 'limited' | 'unlimited';
	gb: number;
}

export interface SystemSettings {
	default_quota: QuotaPolicy;
	audit_retention_days: number;
	worker_concurrency: number;
	placement_strategy: string;
}

export interface UpdateSettingsRequest {
	default_quota?: QuotaPolicy;
	audit_retention_days?: number;
	worker_concurrency?: number;
}

export interface StorageNode {
	id: string;
	type: string;
	name: string;
	path: string;
	kind: string;
	total_gb: number;
	free_gb: number;
	capacity_cap_gb: number;
	used_bytes: number;
	enabled: boolean;
}

export interface StorageNodesResponse {
	nodes: StorageNode[];
}

class AdminAPI {
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
					error: errorData?.error || `HTTP ${response.status}: ${response.statusText}`
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

	async getSettings(): Promise<{ data?: SystemSettings; error?: string }> {
		return this.request<SystemSettings>('/admin/settings');
	}

	async updateSettings(
		updates: UpdateSettingsRequest
	): Promise<{ data?: SystemSettings; error?: string }> {
		return this.request<SystemSettings>('/admin/settings', {
			method: 'PUT',
			body: JSON.stringify(updates)
		});
	}

	async getStorageNodes(): Promise<{ data?: StorageNodesResponse; error?: string }> {
		return this.request<StorageNodesResponse>('/admin/storage-nodes');
	}

	async disableNode(nodeId: string): Promise<{ error?: string }> {
		return this.request(`/admin/storage-nodes/${nodeId}/disable`, {
			method: 'PUT'
		});
	}

	async rescanNodes(): Promise<{ data?: StorageNodesResponse; error?: string }> {
		return this.request<StorageNodesResponse>('/admin/storage-nodes/rescan', {
			method: 'POST'
		});
	}

	async downloadBackup(): Promise<{ error?: string }> {
		try {
			const response = await fetch(`${API_BASE}/admin/backup`, {
				credentials: 'include'
			});

			if (!response.ok) {
				return { error: `HTTP ${response.status}` };
			}

			const blob = await response.blob();
			const url = window.URL.createObjectURL(blob);
			const a = document.createElement('a');
			a.href = url;
			a.download = `cubycloud-backup-${Date.now()}.db`;
			document.body.appendChild(a);
			a.click();
			document.body.removeChild(a);
			window.URL.revokeObjectURL(url);

			return {};
		} catch (err) {
			return {
				error: err instanceof Error ? err.message : 'Download failed'
			};
		}
	}

	async addLocalNode(data: {
		name: string;
		path: string;
		capacity_cap_gb: number;
		enabled: boolean;
	}): Promise<{ data?: any; error?: string }> {
		return this.request('/admin/storage-nodes', {
			method: 'POST',
			body: JSON.stringify(data)
		});
	}

	async addCloudNode(data: {
		name: string;
		endpoint: string;
		bucket: string;
		region: string;
		access_key: string;
		secret_key: string;
		capacity_cap_gb: number;
		enabled: boolean;
	}): Promise<{ data?: any; error?: string }> {
		return this.request('/admin/storage-nodes/cloud', {
			method: 'POST',
			body: JSON.stringify(data)
		});
	}

	async updateNode(
		nodeId: string,
		data: { name?: string; capacity_cap_gb?: number; enabled?: boolean }
	): Promise<{ data?: { message: string }; error?: string }> {
		return this.request(`/admin/storage-nodes/${nodeId}`, {
			method: 'PUT',
			body: JSON.stringify(data)
		});
	}

	async deleteNode(nodeId: string): Promise<{ data?: { message: string }; error?: string }> {
		return this.request(`/admin/storage-nodes/${nodeId}`, {
			method: 'DELETE'
		});
	}
}

export const adminAPI = new AdminAPI();
