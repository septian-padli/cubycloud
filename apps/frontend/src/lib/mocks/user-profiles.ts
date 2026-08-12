import type { AdminUser, DefaultQuotaPolicy } from '$lib/types/admin.js';
import type { CategoryUsage, QuotaState } from '$lib/types/drive.js';
import {
	MOCK_ADMIN_USERS,
	MOCK_DEFAULT_QUOTA,
	effectiveQuotaLabel
} from '$lib/mocks/admin-users.js';
import { MOCK_CATEGORY_USAGE } from '$lib/mocks/storage-stats.js';

export function findAdminUserById(userId: string): AdminUser | undefined {
	return MOCK_ADMIN_USERS.find((u) => u.id === userId);
}

/** Scale baseline category mix to match a user's My Storage used total. */
export function categoryUsageForUser(usedGb: number): CategoryUsage[] {
	const baselineTotal = MOCK_CATEGORY_USAGE.reduce((sum, c) => sum + c.usedGb, 0);
	if (baselineTotal <= 0 || usedGb <= 0) {
		return MOCK_CATEGORY_USAGE.map((c) => ({ ...c, usedGb: 0 }));
	}
	const factor = usedGb / baselineTotal;
	return MOCK_CATEGORY_USAGE.map((c) => ({
		...c,
		usedGb: Math.round(c.usedGb * factor * 10) / 10
	}));
}

export function quotaForAdminUser(
	user: AdminUser,
	defaultQuota: DefaultQuotaPolicy = MOCK_DEFAULT_QUOTA
): QuotaState {
	if (user.role === 'super_admin' || user.quotaMode === 'unlimited') {
		return { usedGb: user.usedGb, limitGb: null };
	}
	if (user.quotaMode === 'override_gb' && user.quotaGb != null) {
		return { usedGb: user.usedGb, limitGb: user.quotaGb };
	}
	if (defaultQuota.mode === 'unlimited') {
		return { usedGb: user.usedGb, limitGb: null };
	}
	return { usedGb: user.usedGb, limitGb: defaultQuota.gb };
}

export function roleLabel(role: AdminUser['role']): string {
	return role === 'super_admin' ? 'Super Admin' : 'User';
}

export { effectiveQuotaLabel };
