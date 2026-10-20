import { rmSync } from 'node:fs';
import { restartControlDir } from './restart-control.js';

export default function globalTeardown() {
	const dataDir = process.env.MY_STORAGE_E2E_DATA_DIR;
	if (dataDir) rmSync(dataDir, { recursive: true, force: true });
	rmSync(restartControlDir, { recursive: true, force: true });
}
