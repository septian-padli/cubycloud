import { getContext, setContext } from 'svelte';
import type { DriveRole, DriveUser, QuotaState } from '$lib/types/drive.js';
import { authAPI } from '$lib/api/auth.js';

const KEY = Symbol('cubycloud-drive-chrome');
const QUOTA_CHANNEL_NAME = 'cubycloud-quota-sync';

export type SharedUploadPermission = 'viewer' | 'editor' | null;

export type DriveChrome = {
	user: DriveUser;
	quota: QuotaState;
	sharedUploadContext: { folderId: string | null; permission: SharedUploadPermission };
	setUserAndQuota: (user: DriveUser, quota: QuotaState) => void;
	updateProfile: (patch: Pick<DriveUser, 'displayName' | 'email'>) => void;
	requestUpload: () => void;
	registerUploadHandler: (handler: (() => void) | null) => void;
	setSharedUploadContext: (folderId: string | null, permission: SharedUploadPermission) => void;
	refreshQuota: (broadcast?: boolean) => Promise<void>;
	notifyStorageChange?: () => void;
	registerStorageChangeHandler?: (handler: (() => void) | null) => void;
	destroy?: () => void;
};

class DriveChromeState implements DriveChrome {
	user = $state({} as DriveUser);
	quota = $state({} as QuotaState);
	sharedUploadContext = $state({
		folderId: null as string | null,
		permission: null as SharedUploadPermission
	});
	#uploadHandler: (() => void) | null = null;
	#storageChangeHandler: (() => void) | null = null;
	#channel: BroadcastChannel | null = null;
	#isRefreshing = false;
	#queuedRefresh = false;

	constructor(user: DriveUser, quota: QuotaState) {
		this.user = { ...user };
		this.quota = { ...quota };

		if (typeof window !== 'undefined' && 'BroadcastChannel' in window) {
			try {
				this.#channel = new BroadcastChannel(QUOTA_CHANNEL_NAME);
				this.#channel.onmessage = (event) => {
					if (event.data?.type === 'QUOTA_SYNC') {
						const { usedBytes, quotaBytes, role } = event.data;
						const effectiveRole: DriveRole =
							role ??
							(this.user?.role?.toLowerCase() === 'super_admin' ? 'super_admin' : 'regular');
						const limitGb =
							effectiveRole === 'super_admin' || quotaBytes < 0
								? null
								: quotaBytes / (1024 * 1024 * 1024);
						const usedGb = (usedBytes ?? 0) / (1024 * 1024 * 1024);
						if (this.quota) {
							this.quota.usedGb = usedGb;
							this.quota.limitGb = limitGb;
						}
					}
				};
			} catch (e) {
				console.warn('[DriveChrome] BroadcastChannel init error:', e);
			}
		}
	}

	setUserAndQuota(user: DriveUser, quota: QuotaState) {
		if (this.user) {
			Object.assign(this.user, user);
		} else {
			this.user = { ...user };
		}
		if (this.quota) {
			this.quota.usedGb = quota.usedGb;
			this.quota.limitGb = quota.limitGb;
		} else {
			this.quota = { ...quota };
		}
	}

	updateProfile(patch: Partial<Pick<DriveUser, 'displayName' | 'email'>>) {
		if (this.user) {
			Object.assign(this.user, patch);
		}
	}
	setSharedUploadContext(folderId: string | null, permission: SharedUploadPermission) {
		this.sharedUploadContext = { folderId, permission };
	}

	requestUpload() {
		this.#uploadHandler?.();
	}

	registerUploadHandler(handler: (() => void) | null) {
		this.#uploadHandler = handler;
	}

	notifyStorageChange() {
		this.#storageChangeHandler?.();
	}

	registerStorageChangeHandler(handler: (() => void) | null) {
		this.#storageChangeHandler = handler;
	}

	async refreshQuota(broadcast = true): Promise<void> {
		if (this.#isRefreshing) {
			this.#queuedRefresh = true;
			return;
		}
		this.#isRefreshing = true;
		try {
			const meResp = await authAPI.getMe();
			if (meResp.data) {
				const u = meResp.data;
				const role: DriveRole = u.role.toLowerCase() === 'super_admin' ? 'super_admin' : 'regular';
				const quota: QuotaState = {
					usedGb: (u.used_bytes ?? 0) / (1024 * 1024 * 1024),
					limitGb:
						role === 'super_admin' || u.quota_bytes < 0
							? null
							: u.quota_bytes / (1024 * 1024 * 1024)
				};
				this.setUserAndQuota(
					{
						id: u.id,
						username: u.username,
						displayName: u.display_name || u.username,
						email: u.email,
						role
					},
					quota
				);
				if (broadcast && this.#channel) {
					this.#channel.postMessage({
						type: 'QUOTA_SYNC',
						usedBytes: u.used_bytes,
						quotaBytes: u.quota_bytes,
						role
					});
				}
			}
		} catch (e) {
			console.warn('[DriveChrome] Failed to refresh quota:', e);
		} finally {
			this.#isRefreshing = false;
			if (this.#queuedRefresh) {
				this.#queuedRefresh = false;
				void this.refreshQuota(broadcast);
			}
		}
	}

	destroy() {
		this.#channel?.close();
		this.#channel = null;
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
