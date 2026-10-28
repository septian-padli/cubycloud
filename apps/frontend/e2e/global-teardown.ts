import { rmSync } from 'node:fs';
import { e2eDataDir, restartControlDir } from './restart-control.js';

export default function globalTeardown() {
	rmSync(e2eDataDir, { recursive: true, force: true });
	rmSync(restartControlDir, { recursive: true, force: true });
}
