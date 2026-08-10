import type { StarredFileItem } from '$lib/types/drive.js';

export const MOCK_STARRED_FILES: StarredFileItem[] = [
	{
		id: 'ms-file-catatan',
		name: 'Catatan Meeting.txt',
		kind: 'text',
		sizeBytes: 6144,
		modifiedAt: '2 jam lalu',
		parentId: null,
		starred: true,
		source: 'my-storage',
		sourceLabel: 'My Storage',
		storageNode: 'ssd'
	},
	{
		id: 'ms-file-brief',
		name: 'Brief Produk.pdf',
		kind: 'document',
		sizeBytes: 512_000,
		modifiedAt: '4 jam lalu',
		parentId: 'ms-folder-proyek',
		starred: true,
		source: 'my-storage',
		sourceLabel: 'My Storage / Proyek Q3',
		storageNode: 'ssd'
	},
	{
		id: 'ms-file-lab1',
		name: 'Foto Lab.jpg',
		kind: 'image',
		sizeBytes: 3_250_000,
		modifiedAt: 'Kemarin',
		parentId: 'ms-folder-foto',
		starred: true,
		source: 'my-storage',
		sourceLabel: 'My Storage / Foto Lab',
		storageNode: 'r2'
	},
	{
		id: 'org-file-logo',
		name: 'Logo CubyCloud.svg',
		kind: 'image',
		sizeBytes: 42_000,
		modifiedAt: '2 minggu lalu',
		parentId: null,
		starred: true,
		source: 'organization',
		sourceLabel: 'Organization',
		storageNode: 's3'
	},
	{
		id: 'org-file-brand',
		name: 'Brand Guidelines.pdf',
		kind: 'document',
		sizeBytes: 3_400_000,
		modifiedAt: '5 hari lalu',
		parentId: 'org-folder-marketing',
		starred: true,
		source: 'organization',
		sourceLabel: 'Organization / Marketing',
		storageNode: 'r2'
	},
	{
		id: 'org-file-runbook',
		name: 'Runbook Deploy.md',
		kind: 'document',
		sizeBytes: 28_000,
		modifiedAt: '1 jam lalu',
		parentId: 'org-folder-engineering',
		starred: true,
		source: 'organization',
		sourceLabel: 'Organization / Engineering',
		storageNode: 'ssd'
	},
	{
		id: 'sh-5',
		name: 'Checklist QA.md',
		kind: 'document',
		sizeBytes: 14_000,
		modifiedAt: '5 hari lalu',
		parentId: null,
		starred: true,
		source: 'shared',
		sourceLabel: 'Dibagikan · Rina Eng',
		storageNode: 'ssd'
	},
	{
		id: 'st-8',
		name: 'Intro Audio.mp3',
		kind: 'audio',
		sizeBytes: 4_403_200,
		modifiedAt: '5 hari lalu',
		parentId: null,
		starred: true,
		source: 'my-storage',
		sourceLabel: 'My Storage',
		storageNode: 'ssd'
	},
	{
		id: 'st-9',
		name: 'Promo Reel.mp4',
		kind: 'video',
		sizeBytes: 62_000_000,
		modifiedAt: '1 minggu lalu',
		parentId: 'org-folder-marketing',
		starred: true,
		source: 'organization',
		sourceLabel: 'Organization / Marketing',
		storageNode: 'r2'
	},
	{
		id: 'st-10',
		name: 'Backup Config.zip',
		kind: 'archive',
		sizeBytes: 1_048_576,
		modifiedAt: '5 hari lalu',
		parentId: null,
		starred: true,
		source: 'my-storage',
		sourceLabel: 'My Storage',
		storageNode: 'hdd'
	}
];
