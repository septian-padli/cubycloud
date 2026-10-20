import { execFileSync, spawn, type ChildProcess } from 'node:child_process';
import { existsSync, mkdirSync, readFileSync, renameSync, rmSync, writeFileSync } from 'node:fs';
import { join, resolve } from 'node:path';
import { restartControlDir } from './restart-control.js';
import { setTimeout as delay } from 'node:timers/promises';

const backendDir = resolve(import.meta.dirname, '../../backend');
const controlDir = restartControlDir;
const dataDir = process.env.DATA_DIR_PATH;
if (!dataDir) throw new Error('DATA_DIR_PATH is required for the E2E backend supervisor');
rmSync(dataDir, { recursive: true, force: true });
mkdirSync(join(dataDir, 'storage', 'default'), { recursive: true });
rmSync(controlDir, { recursive: true, force: true });
mkdirSync(controlDir, { recursive: true });
const binaryPath = join(controlDir, 'backend-server');
const requestPath = join(controlDir, 'restart-request.json');
const resultPath = join(controlDir, 'restart-result.json');
execFileSync('go', ['build', '-o', binaryPath, './cmd/server'], {
	cwd: backendDir,
	stdio: 'inherit'
});

let backend: ChildProcess | undefined;
let generation = 0;
let stopping = false;
let restartInProgress = false;
let stopPromise: Promise<void> | undefined;
let shutdownPromise: Promise<void> | undefined;
let requestedExitCode = 0;

function writeResult(result: {
	requestId: string | null;
	generation: number;
	pid: number | undefined;
}) {
	const temporaryPath = `${resultPath}.${process.pid}.tmp`;
	writeFileSync(temporaryPath, JSON.stringify(result));
	renameSync(temporaryPath, resultPath);
}

function waitForChildExit(child: ChildProcess) {
	return new Promise<void>((resolveExit) => {
		if (child.exitCode !== null || child.signalCode !== null) {
			resolveExit();
			return;
		}
		child.once('exit', () => resolveExit());
		if (child.exitCode !== null || child.signalCode !== null) resolveExit();
	});
}

function startBackend() {
	backend = spawn(binaryPath, [], {
		cwd: backendDir,
		env: process.env,
		stdio: 'inherit'
	});
	generation += 1;
	return backend;
}

function stopBackend(): Promise<void> {
	if (stopPromise) return stopPromise;
	const child = backend;
	if (!child || child.exitCode !== null || child.signalCode !== null) {
		backend = undefined;
		return Promise.resolve();
	}
	backend = undefined;
	const exited = waitForChildExit(child);
	child.kill('SIGTERM');
	stopPromise = Promise.race([
		exited,
		delay(5_000).then(() => {
			if (child.exitCode === null && child.signalCode === null) child.kill('SIGKILL');
			return exited;
		})
	]).finally(() => {
		stopPromise = undefined;
	});
	return stopPromise;
}

async function restart(requestId: string) {
	if (restartInProgress) throw new Error('Backend restart already in progress');
	restartInProgress = true;
	try {
		await stopBackend();
		if (stopping) return;
		const child = startBackend();
		writeResult({ requestId, generation, pid: child.pid });
	} finally {
		restartInProgress = false;
	}
}

async function main() {
	console.log(`Starting Playwright-owned backend supervisor; control result: ${resultPath}`);
	const initial = startBackend();
	writeResult({ requestId: null, generation, pid: initial.pid });
	console.log(`Playwright-owned backend process started (pid=${initial.pid})`);

	while (!stopping) {
		if (existsSync(requestPath)) {
			const request = JSON.parse(readFileSync(requestPath, 'utf8')) as { requestId: string };
			rmSync(requestPath, { force: true });
			await restart(request.requestId);
		}
		await delay(50);
	}
}

function shutdown(exitCode = 0): Promise<void> {
	if (exitCode !== 0) requestedExitCode = exitCode;
	stopping = true;
	if (!shutdownPromise) {
		shutdownPromise = (async () => {
			await stopBackend();
			rmSync(binaryPath, { force: true });
		})();
	}
	return shutdownPromise.then(() => {
		process.exitCode = requestedExitCode;
	});
}

process.once('SIGINT', () => void shutdown());
process.once('SIGTERM', () => void shutdown());
void main().catch((error: unknown) => {
	if (stopping) {
		void shutdown();
		return;
	}
	console.error(error);
	void shutdown(1).catch((shutdownError: unknown) => {
		console.error(shutdownError);
		process.exitCode = 1;
	});
});
