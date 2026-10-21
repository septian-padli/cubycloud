import { resolve } from 'node:path';
import { defineConfig } from '@playwright/test';
import {
	e2eDataDir,
	restartControlDir as defaultRestartControlDir
} from './e2e/restart-control.js';

const backendPort = Number(process.env.MY_STORAGE_E2E_BACKEND_PORT ?? 18383);
const frontendPort = Number(process.env.MY_STORAGE_E2E_FRONTEND_PORT ?? 15188);
const dataDir = e2eDataDir;
const restartControlDir = defaultRestartControlDir;
process.env.MY_STORAGE_E2E_DATA_DIR = dataDir;
process.env.MY_STORAGE_E2E_RESTART_DIR = restartControlDir;
process.env.MY_STORAGE_E2E_BACKEND_PORT = String(backendPort);

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
			command: 'bun run e2e/backend-supervisor.ts',
			cwd: resolve(import.meta.dirname),
			env: {
				PORT: String(backendPort),
				DATA_DIR_PATH: dataDir,
				JWT_SECRET: 'my-storage-e2e-secret',
				MY_STORAGE_E2E_DATA_DIR: dataDir,
				MY_STORAGE_E2E_RESTART_DIR: restartControlDir,
				DISABLE_RATE_LIMIT: 'true'
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
