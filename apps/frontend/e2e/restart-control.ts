import { tmpdir } from 'node:os';
import { resolve } from 'node:path';

export const e2eDataDir =
	process.env.MY_STORAGE_E2E_DATA_DIR ?? resolve(tmpdir(), 'cubycloud-my-storage-e2e');
export const restartControlDir =
	process.env.MY_STORAGE_E2E_RESTART_DIR ?? resolve(import.meta.dirname, '.restart-control');
