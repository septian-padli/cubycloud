import type { LinuxDevice, StorageNode } from '$lib/types/setup.js';

export const MOCK_STORAGE_NODES: StorageNode[] = [
	{
		id: 'node-ssd-1',
		name: 'SSD-1',
		path: '/app/data/storage/ssd-1',
		kind: 'SSD',
		totalGb: 500,
		freeGb: 412,
		usedBytes: 88 * 1024 ** 3,
		enabled: true,
		capacityCapGb: 475
	},
	{
		id: 'node-hdd-1',
		name: 'HDD-1',
		path: '/app/data/storage/hdd-1',
		kind: 'HDD',
		totalGb: 2000,
		freeGb: 1680,
		usedBytes: 320 * 1024 ** 3,
		enabled: true,
		capacityCapGb: 1900
	},
	{
		id: 'node-ssd-2',
		name: 'SSD-2',
		path: '/app/data/storage/ssd-2',
		kind: 'SSD',
		totalGb: 256,
		freeGb: 240,
		usedBytes: 16 * 1024 ** 3,
		enabled: false,
		capacityCapGb: 243
	}
];

export const MOCK_LINUX_DEVICES: LinuxDevice[] = [
	{
		id: 'dev-sdb',
		path: '/dev/sdb',
		sizeGb: 1000,
		model: 'Samsung SSD 870',
		selected: false
	},
	{
		id: 'dev-sdc',
		path: '/dev/sdc',
		sizeGb: 4000,
		model: 'WD Red Plus',
		selected: false
	}
];

export function cloneMockNodes(): StorageNode[] {
	return MOCK_STORAGE_NODES.map((n) => ({ ...n }));
}

export function cloneMockDevices(): LinuxDevice[] {
	return MOCK_LINUX_DEVICES.map((d) => ({ ...d }));
}

/** Cryptographically random Recovery Key in the 6×5 format (UI mock — not persisted). */
export function generateMockRecoveryKey(): string {
	const alphabet = 'ABCDEFGHJKLMNPQRSTUVWXYZ23456789';
	const characters = Array.from(crypto.getRandomValues(new Uint8Array(30)), (value) =>
		alphabet[value & 31]
	);
	return characters.reduce(
		(key, character, index) => key + (index > 0 && index % 5 === 0 ? '-' : '') + character,
		''
	);
}

export function simulateScan(ms = 900): Promise<void> {
	return new Promise((resolve) => setTimeout(resolve, ms));
}
