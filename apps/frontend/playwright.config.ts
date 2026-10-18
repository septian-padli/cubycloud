import { mkdirSync, mkdtempSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { resolve } from 'node:path';
import { defineConfig } from '@playwright/test';

const backendPort = Number(process.env.MY_STORAGE_E2E_BACKEND_PORT ?? 18383);
const frontendPort = Number(process.env.MY_STORAGE_E2E_FRONTEND_PORT ?? 15188);
const dataDir = mkdtempSync(resolve(tmpdir(), 'cubycloud-my-storage-e2e-'));
mkdirSync(resolve(dataDir, 'storage', 'default'), { recursive: true });
process.env.MY_STORAGE_E2E_DATA_DIR = dataDir;

const backendURL = `http://127.0.0.1:${backendPort}`;
const frontendURL = `http://127.0.0.1:${frontendPort}`;

export default defineConfig({
	testDir: './e2e',
	fullyParallel: false,
	workers: 1,
	retries: 0,
	timeout: 60_000,
	reporter: 'list',
	use: {
		baseURL: frontendURL,
		browserName: 'chromium',
		trace: 'retain-on-failure'
	},
	globalTeardown: './e2e/global-teardown.ts',
	webServer: [
		{
			command: 'go run ./cmd/server',
			cwd: resolve(import.meta.dirname, '../backend'),
			env: {
				PORT: String(backendPort),
				DATA_DIR_PATH: dataDir,
				JWT_SECRET: 'my-storage-e2e-secret'
			},
			url: `${backendURL}/api/v1/setup/status`,
			reuseExistingServer: false,
			timeout: 120_000
		},
		{
			command: `bun run dev --host 127.0.0.1 --port ${frontendPort}`,
			env: {
				VITE_API_PROXY_TARGET: backendURL,
				VITE_HMR_PORT: String(frontendPort + 1)
			},
			url: `${frontendURL}/login`,
			reuseExistingServer: false,
			timeout: 120_000
		}
	]
});
