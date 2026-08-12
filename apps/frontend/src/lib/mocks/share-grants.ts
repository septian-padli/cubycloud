import type { ShareGrant } from '$lib/types/drive.js';
import { MOCK_ADMIN_USERS } from '$lib/mocks/admin-users.js';
import { MOCK_DRIVE_USER } from '$lib/mocks/drive.js';

const DEFAULT_GRANT: ShareGrant = {
	userId: 'user-siti',
	username: 'siti',
	displayName: 'Siti Rahayu',
	permission: 'viewer'
};

/** In-memory grants keyed by file/folder id. ponytail: replace with API. */
const grantsByEntry = new Map<string, ShareGrant[]>();

function cloneGrants(grants: ShareGrant[]): ShareGrant[] {
	return grants.map((g) => ({ ...g }));
}

export function shareableUsers(excludeUserId: string = MOCK_DRIVE_USER.id) {
	return MOCK_ADMIN_USERS.filter((u) => u.id !== excludeUserId).map((u) => ({
		userId: u.id,
		username: u.username,
		displayName: u.displayName
	}));
}

export function getGrants(entryId: string): ShareGrant[] {
	const stored = grantsByEntry.get(entryId);
	if (stored) return cloneGrants(stored);
	return cloneGrants([DEFAULT_GRANT]);
}

export function setGrants(entryId: string, grants: ShareGrant[]): ShareGrant[] {
	const next = cloneGrants(grants);
	grantsByEntry.set(entryId, next);
	return next;
}
