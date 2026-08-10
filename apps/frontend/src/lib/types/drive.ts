export type DriveRole = 'super_admin' | 'regular';

export type FileKind = 'folder' | 'text' | 'image' | 'video' | 'audio' | 'document' | 'archive';

export type ActivityAction = 'upload' | 'edit' | 'share';

export type StorageNodeKind = 'r2' | 's3' | 'ssd' | 'hdd';

export type FileCategory = 'all' | FileKind;

export type ExplorerVariant = 'my-storage' | 'organization';

export interface DriveUser {
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
}

export interface ActivityItem {
	id: string;
	action: ActivityAction;
	fileName: string;
	at: string;
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
