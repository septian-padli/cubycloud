export type DriveViewMode = 'list' | 'grid';

const STORAGE_KEY = 'cubycloud-drive-view';

function readStored(): DriveViewMode {
	if (typeof localStorage === 'undefined') return 'list';
	const raw = localStorage.getItem(STORAGE_KEY);
	if (raw === 'list' || raw === 'grid') return raw;
	return 'list';
}

class DriveViewState {
	mode = $state<DriveViewMode>('list');
	#ready = false;

	init() {
		if (this.#ready || typeof window === 'undefined') return;
		this.#ready = true;
		this.mode = readStored();
	}

	setMode(mode: DriveViewMode) {
		this.mode = mode;
		if (typeof localStorage !== 'undefined') {
			localStorage.setItem(STORAGE_KEY, mode);
		}
	}

	toggle() {
		this.setMode(this.mode === 'list' ? 'grid' : 'list');
	}
}

export const driveView = new DriveViewState();
