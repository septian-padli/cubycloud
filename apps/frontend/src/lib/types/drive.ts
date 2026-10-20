export type DriveRole = 'super_admin' | 'regular';

export type FileKind = 'folder' | 'text' | 'image' | 'video' | 'audio' | 'document' | 'archive';

export type ActivityAction = 'upload' | 'edit' | 'share';

export type StorageNodeKind = 'r2' | 's3' | 'ssd' | 'hdd';

export type FileCategory = 'all' | FileKind;

export type ExplorerVariant = 'my-storage' | 'organization';

export interface DriveUser {
	id: string;
	username: string;
	displayName: string;
	email: string;
	role: DriveRole;
}

export interface QuotaState {
	usedGb: number;
	/** null = unlimited */
	limitGb: number | null;
}

export interface QuickAccessItem {
	id: string;
	name: string;
	kind: FileKind;
	sizeLabel: string;
	openedAgo: string;
	source?: 'my-storage' | 'organization';
}

export interface ActivityItem {
	id: string;
	action: ActivityAction;
	fileName: string;
	at: string;
	itemKind?: FileKind;
	source?: 'my-storage' | 'organization';
	itemId?: string;
}

export interface FileEntry {
	id: string;
	name: string;
	kind: FileKind;
	sizeBytes: number;
	modifiedAt: string;
	starred?: boolean;
	storageNode?: StorageNodeKind;
	parentId: string | null;
}

export interface BreadcrumbSegment {
	id: string | null;
	label: string;
}

export type SharePermission = 'viewer' | 'editor';

export type ShareLinkAccess = 'public' | 'restricted';

export type FileSource = 'my-storage' | 'organization' | 'shared';

export interface ShareGrant {
	userId: string;
	username: string;
	displayName: string;
	permission: SharePermission;
}

export interface ShareLinkRecord {
	token: string;
	entryId: string;
	entryName: string;
	entryKind: FileKind;
	access: ShareLinkAccess;
	/** Plaintext mock only — never persist real passwords this way. */
	password: string | null;
	expiresAt: string | null;
	ownerId: string;
	ownerName: string;
}

export interface SharedFileItem extends FileEntry {
	ownerName: string;
	permission: SharePermission;
}

export interface StarredFileItem extends FileEntry {
	source: FileSource;
	sourceLabel: string;
}

export interface TrashFileItem extends FileEntry {
	deletedAt: string;
	daysRemaining: number;
	originalParentId: string | null;
	originalParentMissing?: boolean;
}

export interface CategoryUsage {
	id: 'video' | 'image' | 'document' | 'archive' | 'other' | 'trash' | (string & {});
	label: string;
	usedGb: number;
	bytes?: number;
}

export interface LargestFileItem {
	id: string;
	name: string;
	kind: FileKind;
	sizeBytes: number;
	location: string;
	modifiedAt: string;
}
