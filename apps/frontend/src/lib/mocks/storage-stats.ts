import type { CategoryUsage, LargestFileItem } from '$lib/types/drive.js';

/** Usage breakdown for My Storage only (Organization excluded). */
export const MOCK_CATEGORY_USAGE: CategoryUsage[] = [
	{ id: 'video', label: 'Video', usedGb: 2.8 },
	{ id: 'image', label: 'Gambar', usedGb: 1.6 },
	{ id: 'document', label: 'Dokumen', usedGb: 1.2 },
	{ id: 'archive', label: 'Arsip', usedGb: 0.9 },
	{ id: 'trash', label: 'Trash', usedGb: 0.7 }
];

/** Top largest files — My Storage only (exclude Organization). */
export const MOCK_LARGEST_FILES: LargestFileItem[] = [
	{
		id: 'ms-file-demo',
		name: 'Demo Setup.mp4',
		kind: 'video',
		sizeBytes: 50_331_648,
		modifiedAt: '3 hari lalu'
	},
	{
		id: 'tr-4',
		name: 'Clip Demo.mp4',
		kind: 'video',
		sizeBytes: 38_000_000,
		modifiedAt: '3 minggu lalu'
	},
	{
		id: 'ms-file-lab1',
		name: 'Foto Lab.jpg',
		kind: 'image',
		sizeBytes: 3_250_000,
		modifiedAt: 'Kemarin'
	},
	{
		id: 'ms-file-arsitektur',
		name: 'Laporan_Arsitektur_v1.pdf',
		kind: 'document',
		sizeBytes: 2_516_582,
		modifiedAt: '2 jam lalu'
	},
	{
		id: 'tr-9',
		name: 'Old Banner.png',
		kind: 'image',
		sizeBytes: 2_200_000,
		modifiedAt: '1 bulan lalu'
	},
	{
		id: 'ms-file-intro',
		name: 'Intro Audio.mp3',
		kind: 'audio',
		sizeBytes: 4_403_200,
		modifiedAt: '5 hari lalu'
	},
	{
		id: 'tr-10',
		name: 'Logs Minggu Lalu.zip',
		kind: 'archive',
		sizeBytes: 6_500_000,
		modifiedAt: '3 minggu lalu'
	},
	{
		id: 'ms-file-backup',
		name: 'Backup Config.zip',
		kind: 'archive',
		sizeBytes: 1_048_576,
		modifiedAt: '5 hari lalu'
	},
	{
		id: 'ms-file-wireframe',
		name: 'Wireframe.png',
		kind: 'image',
		sizeBytes: 890_000,
		modifiedAt: 'Kemarin'
	},
	{
		id: 'ms-file-brief',
		name: 'Brief Produk.pdf',
		kind: 'document',
		sizeBytes: 512_000,
		modifiedAt: '4 jam lalu'
	}
];
