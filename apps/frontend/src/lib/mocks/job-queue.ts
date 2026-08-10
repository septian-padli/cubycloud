import type { JobQueueItem } from '$lib/types/admin.js';

/** Sample SQLite worker queue for the admin drawer. */
export const MOCK_JOB_QUEUE: JobQueueItem[] = [
	{
		id: 'job-1',
		type: 'compress_zip',
		status: 'PROCESSING',
		fileLabel: 'Backup Proyek Q3',
		createdAt: '2 menit lalu'
	},
	{
		id: 'job-2',
		type: 'image_converter',
		status: 'PENDING',
		fileLabel: 'Foto Lab.jpg → PNG',
		createdAt: '5 menit lalu'
	},
	{
		id: 'job-3',
		type: 'download_zip',
		status: 'COMPLETED',
		fileLabel: 'Dokumen Rapat (4 berkas)',
		createdAt: '20 menit lalu'
	},
	{
		id: 'job-4',
		type: 'pdf_previewer',
		status: 'FAILED',
		fileLabel: 'Laporan_Arsitektur_v1.pdf',
		createdAt: '1 jam lalu'
	},
	{
		id: 'job-5',
		type: 'download_zip',
		status: 'PENDING',
		fileLabel: 'Aset Kampanye',
		createdAt: '1 jam lalu'
	},
	{
		id: 'job-6',
		type: 'compress_zip',
		status: 'COMPLETED',
		fileLabel: 'Logs Minggu Lalu',
		createdAt: '3 jam lalu'
	}
];

export function cloneMockJobQueue(): JobQueueItem[] {
	return MOCK_JOB_QUEUE.map((j) => ({ ...j }));
}
