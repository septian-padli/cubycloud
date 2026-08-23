import type { PlacementStrategy, CloudProvider } from '$lib/types/setup';

const API_BASE = 'http://localhost:8080/api/v1';

export interface StorageNodeResponse {
	path: string;
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

export async function getSetupStatus(): Promise<SetupStatusResponse> {
	const res = await fetch(`${API_BASE}/setup/status`);
	if (!res.ok) throw new Error(`Status failed: ${res.status}`);
	return res.json();
}

export async function scanStorageNodes(): Promise<ScanNodesResponse> {
	const res = await fetch(`${API_BASE}/setup/scan-nodes`);
	if (!res.ok) throw new Error(`Scan failed: ${res.status}`);
	return res.json();
}

export async function completeSetup(req: CompleteSetupRequest): Promise<CompleteSetupResponse> {
	const res = await fetch(`${API_BASE}/setup/complete`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(req)
	});
	if (!res.ok) {
		const error = await res.json().catch(() => ({ message: 'Unknown error' }));
		throw { status: res.status, ...error };
	}
	return res.json();
}
