import errorIllustration from '$lib/assets/illustration/404.webp';

export type ErrorPageContent = {
	title: string;
	description: string;
	illustration: string;
};

/** ponytail: swap per-status illustration paths when assets are ready */
export function errorIllustrationFor(_status: number): string {
	return errorIllustration;
}

const COPY: Record<number, Omit<ErrorPageContent, 'illustration'>> = {
	401: {
		title: 'Perlu masuk',
		description: 'Silakan masuk ke akun CubyCloud untuk melanjutkan.'
	},
	403: {
		title: 'Akses ditolak',
		description: 'Anda tidak memiliki izin untuk melihat halaman ini.'
	},
	404: {
		title: 'Halaman tidak ditemukan',
		description: 'Tautan mungkin sudah tidak valid atau halaman telah dipindahkan.'
	},
	500: {
		title: 'Kesalahan server',
		description: 'Terjadi gangguan sementara. Coba lagi beberapa saat.'
	},
	503: {
		title: 'Layanan tidak tersedia',
		description: 'Server sedang sibuk atau dalam pemeliharaan. Coba lagi nanti.'
	}
};

export function errorPageContent(status: number, fallbackMessage?: string): ErrorPageContent {
	const known = COPY[status];
	if (known) {
		return { ...known, illustration: errorIllustrationFor(status) };
	}

	return {
		title: 'Terjadi kesalahan',
		description:
			fallbackMessage?.trim() || 'Terjadi kesalahan saat memuat halaman. Silakan coba lagi.',
		illustration: errorIllustrationFor(status)
	};
}
