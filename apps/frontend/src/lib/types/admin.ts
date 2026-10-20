import type { PlacementStrategy } from '$lib/types/setup.js';
export type { PlacementStrategy };

export type AdminRole = 'super_admin' | 'user' | 'SUPER_ADMIN' | 'REGULAR';

export type AdminQuotaMode =
	| 'default'
	| 'override_gb'
	| 'unlimited'
	| 'DEFAULT'
	| 'CUSTOM'
	| 'UNLIMITED';

export type DefaultQuotaMode = 'limited' | 'unlimited';

export type AuditRetentionDays = 7 | 14 | 30 | 60 | 90;

export interface AdminUser {
	id: string;
	username: string;
	displayName: string;
	email: string;
	role: AdminRole;
	quotaMode: AdminQuotaMode;
	/** Set when quotaMode is `override_gb` or `CUSTOM`. */
	quotaGb?: number;
	usedGb: number;
	quotaBytes?: number;
	usedBytes?: number;
	createdAt?: string;
}

export interface DefaultQuotaPolicy {
	mode: DefaultQuotaMode;
	/** Applied when mode is `limited` (default mock: 50 GB). */
	gb: number;
}

export interface SystemSettings {
	defaultQuota: DefaultQuotaPolicy;
	auditRetentionDays: AuditRetentionDays;
	workerConcurrency: number;
	placementStrategy: PlacementStrategy;
}

export const AUDIT_RETENTION_OPTIONS: AuditRetentionDays[] = [7, 14, 30, 60, 90];

export interface AdminStorageCategoryUsage {
	id: string;
	label: string;
	bytes: number;
	used_gb: number;
}

export interface AdminStorageStats {
	total_capacity_bytes: number;
	total_capacity_gb: number;
	total_used_bytes: number;
	total_used_gb: number;
	free_bytes: number;
	free_gb: number;
	nodes_count: number;
	categories?: AdminStorageCategoryUsage[];
}

export interface AdminTopStorageUser {
	id: string;
	username: string;
	email: string;
	role: 'user' | 'super_admin' | string;
	quota_mode: string;
	quota_bytes: number;
	quota_gb?: number;
	used_bytes: number;
	used_gb: number;
	created_at: string;
}

export interface AdminTopStorageUsersResponse {
	items: AdminTopStorageUser[];
	total: number;
	has_more: boolean;
	next_offset: number;
}
