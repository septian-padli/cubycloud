export type ThemeMode = 'light' | 'dark' | 'system';

const STORAGE_KEY = 'cubycloud-theme';

function getSystemDark(): boolean {
	if (typeof window === 'undefined') return false;
	return window.matchMedia('(prefers-color-scheme: dark)').matches;
}

function resolveDark(mode: ThemeMode): boolean {
	return mode === 'dark' || (mode === 'system' && getSystemDark());
}

function applyDom(isDark: boolean) {
	if (typeof document === 'undefined') return;
	document.documentElement.classList.toggle('dark', isDark);
}

function readStored(): ThemeMode {
	if (typeof localStorage === 'undefined') return 'system';
	const raw = localStorage.getItem(STORAGE_KEY);
	if (raw === 'light' || raw === 'dark' || raw === 'system') return raw;
	return 'system';
}

class ThemeState {
	mode = $state<ThemeMode>('system');
	#ready = false;

	get isDark(): boolean {
		return resolveDark(this.mode);
	}

	init() {
		if (this.#ready || typeof window === 'undefined') return;
		this.#ready = true;
		this.mode = readStored();
		applyDom(this.isDark);

		const mq = window.matchMedia('(prefers-color-scheme: dark)');
		mq.addEventListener('change', () => {
			if (this.mode === 'system') applyDom(getSystemDark());
		});
	}

	setMode(mode: ThemeMode) {
		this.mode = mode;
		if (typeof localStorage !== 'undefined') {
			localStorage.setItem(STORAGE_KEY, mode);
		}
		applyDom(this.isDark);
	}

	toggle() {
		this.setMode(this.isDark ? 'light' : 'dark');
	}
}

export const theme = new ThemeState();
