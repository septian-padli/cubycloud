import { getContext, setContext } from 'svelte';
import type { DriveRole, DriveUser, QuotaState } from '$lib/types/drive.js';

const KEY = Symbol('cubycloud-drive-chrome');

export type DriveChrome = {
	user: DriveUser;
	quota: QuotaState;
	setRole: (role: DriveRole) => void;
	updateProfile: (patch: Pick<DriveUser, 'displayName' | 'email'>) => void;
	requestUpload: () => void;
	registerUploadHandler: (handler: (() => void) | null) => void;
};

class DriveChromeState implements DriveChrome {
	user = $state({} as DriveUser);
	quota = $state({} as QuotaState);
	#uploadHandler: (() => void) | null = null;

	constructor(user: DriveUser, quota: QuotaState) {
		this.user = { ...user };
		this.quota = { ...quota };
	}

	setRole(role: DriveRole) {
		this.user.role = role;
	}

	updateProfile(patch: Pick<DriveUser, 'displayName' | 'email'>) {
		this.user = { ...this.user, ...patch };
	}

	requestUpload() {
		this.#uploadHandler?.();
	}

	registerUploadHandler(handler: (() => void) | null) {
		this.#uploadHandler = handler;
	}
}

export function setDriveChrome(value: { user: DriveUser; quota: QuotaState }): DriveChrome {
	const chrome = new DriveChromeState(value.user, value.quota);
	setContext(KEY, chrome);
	return chrome;
}

export function getDriveChrome(): DriveChrome {
	return getContext(KEY);
}
