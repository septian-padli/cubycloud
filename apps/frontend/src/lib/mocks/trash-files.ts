import type { TrashFileItem } from '$lib/types/drive.js';

export const MOCK_TRASH_FILES: TrashFileItem[] = [
	{
		id: 'tr-1',
		name: 'Draft Lama.md',
		kind: 'document',
		sizeBytes: 18_000,
		modifiedAt: '1 minggu lalu',
		parentId: null,
		deletedAt: '2 hari lalu',
		daysRemaining: 28,
		originalParentId: 'ms-folder-proyek',
		storageNode: 'ssd'
	},
	{
		id: 'tr-2',
		name: 'Screenshot Bug.png',
		kind: 'image',
		sizeBytes: 1_800_000,
		modifiedAt: '5 hari lalu',
		parentId: null,
		deletedAt: '4 hari lalu',
		daysRemaining: 26,
		originalParentId: null,
		storageNode: 'r2'
	},
	{
		id: 'tr-3',
		name: 'Temp Export.zip',
		kind: 'archive',
		sizeBytes: 12_000_000,
		modifiedAt: '2 minggu lalu',
		parentId: null,
		deletedAt: '6 hari lalu',
		daysRemaining: 24,
		originalParentId: 'ms-folder-dokumen',
		originalParentMissing: true,
		storageNode: 'hdd'
	},
	{
		id: 'tr-4',
		name: 'Clip Demo.mp4',
		kind: 'video',
		sizeBytes: 38_000_000,
		modifiedAt: '3 minggu lalu',
		parentId: null,
		deletedAt: '8 hari lalu',
		daysRemaining: 22,
		originalParentId: 'ms-folder-foto',
		storageNode: 'r2'
	},
	{
		id: 'tr-5',
		name: 'Voice Memo.mp3',
		kind: 'audio',
		sizeBytes: 2_400_000,
		modifiedAt: '10 hari lalu',
		parentId: null,
		deletedAt: '9 hari lalu',
		daysRemaining: 21,
		originalParentId: null,
		storageNode: 'ssd'
	},
	{
		id: 'tr-6',
		name: 'Catatan Kuno.txt',
		kind: 'text',
		sizeBytes: 4_100,
		modifiedAt: '1 bulan lalu',
		parentId: null,
		deletedAt: '12 hari lalu',
		daysRemaining: 18,
		originalParentId: 'ms-folder-proyek',
		storageNode: 'ssd'
	},
	{
		id: 'tr-7',
		name: 'Folder Arsip 2024',
		kind: 'folder',
		sizeBytes: 0,
		modifiedAt: '2 bulan lalu',
		parentId: null,
		deletedAt: '14 hari lalu',
		daysRemaining: 16,
		originalParentId: null,
		storageNode: 'ssd'
	},
	{
		id: 'tr-8',
		name: 'Invoice Duplikat.pdf',
		kind: 'document',
		sizeBytes: 340_000,
		modifiedAt: '3 minggu lalu',
		parentId: null,
		deletedAt: '18 hari lalu',
		daysRemaining: 12,
		originalParentId: 'folder-gone',
		originalParentMissing: true,
		storageNode: 'ssd'
	},
	{
		id: 'tr-9',
		name: 'Old Banner.png',
		kind: 'image',
		sizeBytes: 2_200_000,
		modifiedAt: '1 bulan lalu',
		parentId: null,
		deletedAt: '22 hari lalu',
		daysRemaining: 8,
		originalParentId: 'ms-folder-foto',
		storageNode: 'r2'
	},
	{
		id: 'tr-10',
		name: 'Logs Minggu Lalu.zip',
		kind: 'archive',
		sizeBytes: 6_500_000,
		modifiedAt: '3 minggu lalu',
		parentId: null,
		deletedAt: '25 hari lalu',
		daysRemaining: 5,
		originalParentId: null,
		storageNode: 'hdd'
	}
];
