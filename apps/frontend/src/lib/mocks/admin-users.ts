import type { AdminUser, DefaultQuotaPolicy } from '$lib/types/admin.js';

/** Default policy used when a user's quotaMode is `default`. */
export const MOCK_DEFAULT_QUOTA: DefaultQuotaPolicy = {
	mode: 'limited',
	gb: 50
};

export const MOCK_ADMIN_USERS: AdminUser[] = [
	{
		id: 'user-admin',
		username: 'admin',
		displayName: 'Super Admin',
		email: 'admin@contoh.local',
		role: 'super_admin',
		quotaMode: 'unlimited',
		usedGb: 12.4
	},
	{
		id: 'user-siti',
		username: 'siti',
		displayName: 'Siti Rahayu',
		email: 'siti@contoh.local',
		role: 'user',
		quotaMode: 'default',
		usedGb: 18.2
	},
	{
		id: 'user-budi',
		username: 'budi',
		displayName: 'Budi Santoso',
		email: 'budi@contoh.local',
		role: 'user',
		quotaMode: 'override_gb',
		quotaGb: 100,
		usedGb: 42.5
	},
	{
		id: 'user-andi',
		username: 'andi',
		displayName: 'Andi Wijaya',
		email: 'andi@contoh.local',
		role: 'user',
		quotaMode: 'override_gb',
		quotaGb: 25,
		usedGb: 21.8
	},
	{
		id: 'user-rina',
		username: 'rina',
		displayName: 'Rina Melati',
		email: 'rina@contoh.local',
		role: 'user',
		quotaMode: 'default',
		usedGb: 3.1
	}
];

export function cloneMockAdminUsers(): AdminUser[] {
	return MOCK_ADMIN_USERS.map((u) => ({ ...u }));
}

/** Effective quota label for table display (GB or Unlimited). */
export function effectiveQuotaLabel(
	user: AdminUser,
	defaultQuota: DefaultQuotaPolicy = MOCK_DEFAULT_QUOTA
): string {
	const role = String(user.role).toLowerCase();
	const mode = String(user.quotaMode).toLowerCase();
	if (role === 'super_admin' || mode === 'unlimited') {
		return 'Unlimited';
	}
	if (mode === 'override_gb' || mode === 'custom') {
		if (user.quotaGb != null) {
			return `${user.quotaGb} GB`;
		}
	}
	if (user.quotaGb != null) {
		return `${user.quotaGb} GB`;
	}
	if (user.quotaBytes != null) {
		if (user.quotaBytes < 0) return 'Unlimited';
		const gb = user.quotaBytes / (1024 * 1024 * 1024);
		return `${Number(gb.toFixed(2))} GB`;
	}
	if (defaultQuota.mode === 'unlimited') {
		return 'Unlimited';
	}
	return `${defaultQuota.gb} GB`;
}
