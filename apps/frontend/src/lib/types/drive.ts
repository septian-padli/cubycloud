export type DriveRole = 'super_admin' | 'regular';

export type FileKind = 'folder' | 'text' | 'image' | 'video' | 'audio' | 'document';

export type ActivityAction = 'upload' | 'edit' | 'share';

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
