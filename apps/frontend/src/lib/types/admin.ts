export type AdminRole = 'super_admin' | 'user';

export type AdminQuotaMode = 'default' | 'override_gb' | 'unlimited';

export type DefaultQuotaMode = 'limited' | 'unlimited';

export type AuditRetentionDays = 7 | 14 | 30 | 60 | 90;

export type AddonId =
	| 'image_converter'
	| 'compress_zip'
	| 'download_zip'
	| 'pdf_previewer';

export type JobStatus = 'PENDING' | 'PROCESSING' | 'COMPLETED' | 'FAILED';

export interface AdminUser {
	id: string;
	username: string;
	email: string;
	role: AdminRole;
	quotaMode: AdminQuotaMode;
	/** Set when quotaMode is `override_gb`. */
	quotaGb?: number;
	usedGb: number;
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
}

export interface AddonConfig {
	id: AddonId;
	name: string;
	description: string;
	enabled: boolean;
}

export interface JobQueueItem {
	id: string;
	type: string;
	status: JobStatus;
	fileLabel: string;
	createdAt: string;
}

export const AUDIT_RETENTION_OPTIONS: AuditRetentionDays[] = [7, 14, 30, 60, 90];
