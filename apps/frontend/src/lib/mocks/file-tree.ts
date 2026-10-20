import type { BreadcrumbSegment, FileEntry } from '$lib/types/drive.js';

export const MOCK_MY_STORAGE_TREE: FileEntry[] = [
	{
		id: 'ms-folder-proyek',
		name: 'Proyek Q3',
		kind: 'folder',
		sizeBytes: 0,
		modifiedAt: '1 jam lalu',
		parentId: null,
		storageNode: 'ssd'
	},
	{
		id: 'ms-folder-foto',
		name: 'Foto Lab',
		kind: 'folder',
		sizeBytes: 0,
		modifiedAt: 'Kemarin',
		parentId: null,
		storageNode: 'r2'
	},
	{
		id: 'ms-file-catatan',
		name: 'Catatan Meeting.txt',
		kind: 'text',
		sizeBytes: 6144,
		modifiedAt: '2 jam lalu',
		parentId: null,
		starred: true,
		storageNode: 'ssd'
	},
	{
		id: 'ms-file-rencana',
		name: 'Rencana Q3.md',
		kind: 'document',
		sizeBytes: 24576,
		modifiedAt: '1 minggu lalu',
		parentId: null,
		storageNode: 'ssd'
	},
	{
		id: 'ms-file-backup',
		name: 'Backup Config.zip',
		kind: 'archive',
		sizeBytes: 1_048_576,
		modifiedAt: '5 hari lalu',
		parentId: null,
		storageNode: 'hdd'
	},
	{
		id: 'ms-file-demo',
		name: 'Demo Setup.mp4',
		kind: 'video',
		sizeBytes: 50_331_648,
		modifiedAt: '3 hari lalu',
		parentId: null,
		storageNode: 'r2'
	},
	{
		id: 'ms-file-intro',
		name: 'Intro Audio.mp3',
		kind: 'audio',
		sizeBytes: 4_403_200,
		modifiedAt: '5 hari lalu',
		parentId: null,
		storageNode: 'ssd'
	},
	// Proyek Q3 children
	{
		id: 'ms-folder-dokumen',
		name: 'Dokumen',
		kind: 'folder',
		sizeBytes: 0,
		modifiedAt: '3 jam lalu',
		parentId: 'ms-folder-proyek',
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
		storageNode: 'ssd'
	},
	{
		id: 'ms-file-wireframe',
		name: 'Wireframe.png',
		kind: 'image',
		sizeBytes: 890_000,
		modifiedAt: 'Kemarin',
		parentId: 'ms-folder-proyek',
		storageNode: 'r2'
	},
	// Dokumen (nested under Proyek Q3)
	{
		id: 'ms-file-arsitektur',
		name: 'Laporan_Arsitektur_v1.pdf',
		kind: 'document',
		sizeBytes: 2_516_582,
		modifiedAt: '2 jam lalu',
		parentId: 'ms-folder-dokumen',
		storageNode: 'ssd'
	},
	{
		id: 'ms-file-spec',
		name: 'Spesifikasi API.md',
		kind: 'document',
		sizeBytes: 18_432,
		modifiedAt: '6 jam lalu',
		parentId: 'ms-folder-dokumen',
		storageNode: 'ssd'
	},
	// Foto Lab children
	{
		id: 'ms-file-lab1',
		name: 'Foto Lab.jpg',
		kind: 'image',
		sizeBytes: 3_250_000,
		modifiedAt: 'Kemarin',
		parentId: 'ms-folder-foto',
		starred: true,
		storageNode: 'r2'
	},
	{
		id: 'ms-file-lab2',
		name: 'Mikroskop.heic',
		kind: 'image',
		sizeBytes: 4_100_000,
		modifiedAt: '2 hari lalu',
		parentId: 'ms-folder-foto',
		storageNode: 'r2'
	},
	{
		id: 'ms-file-lab3',
		name: 'Sampel Video.mp4',
		kind: 'video',
		sizeBytes: 28_000_000,
		modifiedAt: '4 hari lalu',
		parentId: 'ms-folder-foto',
		storageNode: 'r2'
	}
];

export const MOCK_ORG_TREE: FileEntry[] = [
	{
		id: 'org-folder-hr',
		name: 'HR & Kebijakan',
		kind: 'folder',
		sizeBytes: 0,
		modifiedAt: 'Kemarin',
		parentId: null,
		storageNode: 'ssd'
	},
	{
		id: 'org-folder-marketing',
		name: 'Marketing',
		kind: 'folder',
		sizeBytes: 0,
		modifiedAt: '3 hari lalu',
		parentId: null,
		storageNode: 'r2'
	},
	{
		id: 'org-folder-engineering',
		name: 'Engineering',
		kind: 'folder',
		sizeBytes: 0,
		modifiedAt: '1 jam lalu',
		parentId: null,
		storageNode: 'ssd'
	},
	{
		id: 'org-file-handbook',
		name: 'Employee Handbook.pdf',
		kind: 'document',
		sizeBytes: 1_200_000,
		modifiedAt: '1 minggu lalu',
		parentId: null,
		storageNode: 'ssd'
	},
	{
		id: 'org-file-logo',
		name: 'Logo CubyCloud.svg',
		kind: 'image',
		sizeBytes: 42_000,
		modifiedAt: '2 minggu lalu',
		parentId: null,
		starred: true,
		storageNode: 's3'
	},
	// HR children
	{
		id: 'org-file-cuti',
		name: 'Formulir Cuti.docx',
		kind: 'document',
		sizeBytes: 85_000,
		modifiedAt: 'Kemarin',
		parentId: 'org-folder-hr',
		storageNode: 'ssd'
	},
	{
		id: 'org-file-onboarding',
		name: 'Checklist Onboarding.md',
		kind: 'document',
		sizeBytes: 12_000,
		modifiedAt: '3 hari lalu',
		parentId: 'org-folder-hr',
		storageNode: 'ssd'
	},
	// Marketing children
	{
		id: 'org-folder-kampanye',
		name: 'Kampanye Q3',
		kind: 'folder',
		sizeBytes: 0,
		modifiedAt: '2 hari lalu',
		parentId: 'org-folder-marketing',
		storageNode: 'r2'
	},
	{
		id: 'org-file-brand',
		name: 'Brand Guidelines.pdf',
		kind: 'document',
		sizeBytes: 3_400_000,
		modifiedAt: '5 hari lalu',
		parentId: 'org-folder-marketing',
		starred: true,
		storageNode: 'r2'
	},
	{
		id: 'org-file-reel',
		name: 'Promo Reel.mp4',
		kind: 'video',
		sizeBytes: 62_000_000,
		modifiedAt: '1 minggu lalu',
		parentId: 'org-folder-marketing',
		storageNode: 'r2'
	},
	// Kampanye Q3 nested
	{
		id: 'org-file-banner',
		name: 'Banner Hero.png',
		kind: 'image',
		sizeBytes: 1_800_000,
		modifiedAt: '2 hari lalu',
		parentId: 'org-folder-kampanye',
		storageNode: 'r2'
	},
	{
		id: 'org-file-copy',
		name: 'Copy Ads.txt',
		kind: 'text',
		sizeBytes: 4_200,
		modifiedAt: '2 hari lalu',
		parentId: 'org-folder-kampanye',
		storageNode: 'ssd'
	},
	// Engineering children
	{
		id: 'org-file-runbook',
		name: 'Runbook Deploy.md',
		kind: 'document',
		sizeBytes: 28_000,
		modifiedAt: '1 jam lalu',
		parentId: 'org-folder-engineering',
		starred: true,
		storageNode: 'ssd'
	},
	{
		id: 'org-file-schema',
		name: 'Schema Dump.sql',
		kind: 'text',
		sizeBytes: 156_000,
		modifiedAt: 'Kemarin',
		parentId: 'org-folder-engineering',
		storageNode: 'ssd'
	},
	{
		id: 'org-file-logs',
		name: 'Logs Archive.zip',
		kind: 'archive',
		sizeBytes: 8_500_000,
		modifiedAt: '4 hari lalu',
		parentId: 'org-folder-engineering',
		storageNode: 'hdd'
	},
	{
		id: 'org-file-demo-audio',
		name: 'Standup Recording.mp3',
		kind: 'audio',
		sizeBytes: 6_200_000,
		modifiedAt: '3 hari lalu',
		parentId: 'org-folder-engineering',
		storageNode: 'ssd'
	}
];

export function findEntry(tree: FileEntry[], id: string): FileEntry | undefined {
	return tree.find((e) => e.id === id);
}

export function getChildren(tree: FileEntry[], folderId: string | null): FileEntry[] {
	return tree
		.filter((e) => e.parentId === folderId)
		.sort((a, b) => {
			if (a.kind === 'folder' && b.kind !== 'folder') return -1;
			if (a.kind !== 'folder' && b.kind === 'folder') return 1;
			return a.name.localeCompare(b.name, 'id');
		});
}

export function isAncestor(tree: FileEntry[], ancestorId: string, targetId: string): boolean {
	let current = findEntry(tree, targetId);
	while (current?.parentId) {
		if (current.parentId === ancestorId) return true;
		current = findEntry(tree, current.parentId);
	}
	return false;
}

export function getBreadcrumbPath(
	tree: FileEntry[],
	folderId: string | null,
	rootLabel: string
): BreadcrumbSegment[] {
	const segments: BreadcrumbSegment[] = [{ id: null, label: rootLabel }];
	if (!folderId) return segments;

	const chain: FileEntry[] = [];
	let current = findEntry(tree, folderId);
	while (current) {
		chain.unshift(current);
		current = current.parentId ? findEntry(tree, current.parentId) : undefined;
	}

	for (const entry of chain) {
		if (entry.kind === 'folder') {
			segments.push({ id: entry.id, label: entry.name });
		}
	}
	return segments;
}

export function getFolderOptions(tree: FileEntry[]): FileEntry[] {
	return tree.filter((e) => e.kind === 'folder').sort((a, b) => a.name.localeCompare(b.name, 'id'));
}

export function formatSize(bytes: number): string {
	if (bytes <= 0) return '—';
	if (bytes < 1024) return `${bytes} B`;
	if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`;
	if (bytes < 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
	return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)} GB`;
}

export function cloneTree(tree: FileEntry[]): FileEntry[] {
	return tree.map((e) => ({ ...e }));
}

export function nextId(prefix: string): string {
	return `${prefix}-${Date.now().toString(36)}`;
}
