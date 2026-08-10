import type { SharedFileItem } from '$lib/types/drive.js';

export const MOCK_SHARED_FILES: SharedFileItem[] = [
	{
		id: 'sh-1',
		name: 'Brief Kampanye.pdf',
		kind: 'document',
		sizeBytes: 1_200_000,
		modifiedAt: '2 jam lalu',
		parentId: null,
		ownerName: 'Siti Marketing',
		permission: 'editor',
		storageNode: 'r2'
	},
	{
		id: 'sh-2',
		name: 'Logo Partner.svg',
		kind: 'image',
		sizeBytes: 48_000,
		modifiedAt: 'Kemarin',
		parentId: null,
		ownerName: 'Budi Design',
		permission: 'viewer',
		storageNode: 's3'
	},
	{
		id: 'sh-3',
		name: 'Rekaman Rapat.mp3',
		kind: 'audio',
		sizeBytes: 8_400_000,
		modifiedAt: '3 hari lalu',
		parentId: null,
		ownerName: 'Andi HR',
		permission: 'viewer',
		storageNode: 'ssd'
	},
	{
		id: 'sh-4',
		name: 'Demo Produk.mp4',
		kind: 'video',
		sizeBytes: 72_000_000,
		modifiedAt: '4 hari lalu',
		parentId: null,
		ownerName: 'Siti Marketing',
		permission: 'editor',
		storageNode: 'r2'
	},
	{
		id: 'sh-5',
		name: 'Checklist QA.md',
		kind: 'document',
		sizeBytes: 14_000,
		modifiedAt: '5 hari lalu',
		parentId: null,
		ownerName: 'Rina Eng',
		permission: 'editor',
		starred: true,
		storageNode: 'ssd'
	},
	{
		id: 'sh-6',
		name: 'Budget Q3.xlsx',
		kind: 'document',
		sizeBytes: 220_000,
		modifiedAt: '1 minggu lalu',
		parentId: null,
		ownerName: 'Dewi Finance',
		permission: 'viewer',
		storageNode: 'ssd'
	},
	{
		id: 'sh-7',
		name: 'Asset Pack.zip',
		kind: 'archive',
		sizeBytes: 45_000_000,
		modifiedAt: '1 minggu lalu',
		parentId: null,
		ownerName: 'Budi Design',
		permission: 'viewer',
		storageNode: 'hdd'
	},
	{
		id: 'sh-8',
		name: 'Catatan Sync.txt',
		kind: 'text',
		sizeBytes: 3_200,
		modifiedAt: '2 minggu lalu',
		parentId: null,
		ownerName: 'Rina Eng',
		permission: 'editor',
		storageNode: 'ssd'
	},
	{
		id: 'sh-9',
		name: 'Foto Event.jpg',
		kind: 'image',
		sizeBytes: 4_100_000,
		modifiedAt: '2 minggu lalu',
		parentId: null,
		ownerName: 'Siti Marketing',
		permission: 'viewer',
		storageNode: 'r2'
	},
	{
		id: 'sh-10',
		name: 'SOP Onboarding.pdf',
		kind: 'document',
		sizeBytes: 980_000,
		modifiedAt: '3 minggu lalu',
		parentId: null,
		ownerName: 'Andi HR',
		permission: 'editor',
		storageNode: 'ssd'
	}
];
