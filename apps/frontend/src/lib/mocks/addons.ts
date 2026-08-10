import type { AddonConfig } from '$lib/types/admin.js';

/** Phase-1 add-ons — PDF Previewer OFF by default (SITEMAP §D). */
export const MOCK_ADDONS: AddonConfig[] = [
	{
		id: 'image_converter',
		name: 'Image Converter on Download',
		description: 'Konversi format gambar saat unduh (mis. WebP → JPEG/PNG).',
		enabled: true
	},
	{
		id: 'compress_zip',
		name: 'Compress to ZIP',
		description: 'Kompres berkas atau folder menjadi arsip ZIP di latar belakang.',
		enabled: true
	},
	{
		id: 'download_zip',
		name: 'Download as ZIP',
		description: 'Unduh beberapa berkas sekaligus sebagai satu arsip ZIP.',
		enabled: true
	},
	{
		id: 'pdf_previewer',
		name: 'PDF Previewer',
		description: 'Pratinjau PDF di lightbox (memakai worker, default nonaktif).',
		enabled: false
	}
];

export function cloneMockAddons(): AddonConfig[] {
	return MOCK_ADDONS.map((a) => ({ ...a }));
}
