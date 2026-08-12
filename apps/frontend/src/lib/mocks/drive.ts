import type {
	ActivityItem,
	DriveUser,
	QuotaState,
	QuickAccessItem
} from '$lib/types/drive.js';

export const MOCK_DRIVE_USER: DriveUser = {
	id: 'user-admin',
	username: 'admin',
	displayName: 'Super Admin',
	email: 'admin@contoh.local',
	role: 'super_admin'
};

/** Limited quota demo — ~55% used (bar bg-primary). */
export const MOCK_QUOTA: QuotaState = {
	usedGb: 8.2,
	limitGb: 15
};

export const MOCK_QUICK_ACCESS: QuickAccessItem[] = [
	{
		id: 'qa-1',
		name: 'Proyek Q3',
		kind: 'folder',
		sizeLabel: '12 item',
		openedAgo: '1 jam lalu'
	},
	{
		id: 'qa-2',
		name: 'Catatan Meeting.txt',
		kind: 'text',
		sizeLabel: '6 KB',
		openedAgo: '2 jam lalu'
	},
	{
		id: 'qa-3',
		name: 'Foto Lab.jpg',
		kind: 'image',
		sizeLabel: '3.1 MB',
		openedAgo: 'Kemarin'
	},
	{
		id: 'qa-4',
		name: 'Demo Setup.mp4',
		kind: 'video',
		sizeLabel: '48 MB',
		openedAgo: '3 hari lalu'
	},
	{
		id: 'qa-5',
		name: 'Intro Audio.mp3',
		kind: 'audio',
		sizeLabel: '4.2 MB',
		openedAgo: '5 hari lalu'
	},
	{
		id: 'qa-6',
		name: 'Rencana Q3.md',
		kind: 'document',
		sizeLabel: '24 KB',
		openedAgo: '1 minggu lalu'
	}
];

export const MOCK_ACTIVITIES: ActivityItem[] = [
	{
		id: 'act-1',
		action: 'upload',
		fileName: 'Foto Lab.jpg',
		at: '2 jam lalu'
	},
	{
		id: 'act-2',
		action: 'share',
		fileName: 'Rencana Q3.md',
		at: 'Kemarin'
	},
	{
		id: 'act-3',
		action: 'edit',
		fileName: 'Catatan Meeting.txt',
		at: '2 hari lalu'
	},
	{
		id: 'act-4',
		action: 'upload',
		fileName: 'Demo Setup.mp4',
		at: '3 hari lalu'
	},
	{
		id: 'act-5',
		action: 'share',
		fileName: 'Backup Config.zip',
		at: '5 hari lalu'
	},
	{
		id: 'act-6',
		action: 'upload',
		fileName: 'Intro Audio.mp3',
		at: '1 minggu lalu'
	},
	{
		id: 'act-7',
		action: 'edit',
		fileName: 'Rencana Q3.md',
		at: '1 minggu lalu'
	}
];
