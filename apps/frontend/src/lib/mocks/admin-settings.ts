import { MOCK_STORAGE_NODES } from '$lib/mocks/setup.js';
import type { SystemSettings } from '$lib/types/admin.js';
import type { StorageNode } from '$lib/types/setup.js';

/** Free space below this ratio of total → badge Kritis. */
export const NODE_CRITICAL_FREE_RATIO = 0.05;

export const MOCK_SYSTEM_SETTINGS: SystemSettings = {
	defaultQuota: { mode: 'limited', gb: 50 },
	auditRetentionDays: 90,
	workerConcurrency: 2
};

/**
 * Admin node list — reuses setup StorageNode shape.
 * SSD-2 is near-full so `isNodeCritical` is true for UI badge demos.
 */
export const MOCK_ADMIN_STORAGE_NODES: StorageNode[] = [
	...MOCK_STORAGE_NODES.slice(0, 2),
	{
		...MOCK_STORAGE_NODES[2],
		enabled: true,
		freeGb: 8,
		capacityCapGb: 243
	}
];

export function isNodeCritical(node: StorageNode): boolean {
	if (node.totalGb <= 0) return false;
	return node.freeGb / node.totalGb < NODE_CRITICAL_FREE_RATIO;
}

export function cloneMockSystemSettings(): SystemSettings {
	return {
		defaultQuota: { ...MOCK_SYSTEM_SETTINGS.defaultQuota },
		auditRetentionDays: MOCK_SYSTEM_SETTINGS.auditRetentionDays,
		workerConcurrency: MOCK_SYSTEM_SETTINGS.workerConcurrency
	};
}

export function cloneMockAdminNodes(): StorageNode[] {
	return MOCK_ADMIN_STORAGE_NODES.map((n) => ({ ...n }));
}
