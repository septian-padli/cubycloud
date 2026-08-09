import { getContext, setContext } from 'svelte';
import type { DriveUser, QuotaState } from '$lib/types/drive.js';

const KEY = Symbol('cubycloud-drive-chrome');

export type DriveChrome = {
	user: DriveUser;
	quota: QuotaState;
};

export function setDriveChrome(value: DriveChrome) {
	setContext(KEY, value);
}

export function getDriveChrome(): DriveChrome {
	return getContext(KEY);
}
