export type PlacementStrategy = 'FILL_FIRST' | 'ROUND_ROBIN' | 'MOST_FREE_SPACE';

export type CloudProvider = 'R2' | 'S3' | 'S3_COMPATIBLE';

export type SetupStepId = 1 | 2 | 3 | 4 | 5;

export interface StorageNode {
	id: string;
	name: string;
	path: string;
	kind: 'SSD' | 'HDD' | 'S3' | 'UNKNOWN';
	type?: 'LOCAL' | 'S3';
	endpoint?: string;
	bucket?: string;
	region?: string;
	totalGb: number;
	freeGb: number;
	usedBytes: number;
	enabled: boolean;
	capacityCapGb: number;
	safeToDisconnect?: boolean;
	migrationStatus?: 'MIGRATING' | 'COMPLETED' | 'FAILED' | 'IDLE';
	migrationJobId?: string;
}

export interface LinuxDevice {
	id: string;
	path: string;
	sizeGb: number;
	model: string;
	selected: boolean;
}

export interface AdminForm {
	username: string;
	email: string;
	password: string;
	confirmPassword: string;
}

export interface CloudForm {
	enabled: boolean;
	provider: CloudProvider;
	nodeName: string;
	endpoint: string;
	region: string;
	bucket: string;
	accessKey: string;
	secretKey: string;
	capacityCapGb?: number | string | null;
}

export interface SetupWizardState {
	admin: AdminForm;
	nodes: StorageNode[];
	devices: LinuxDevice[];
	strategy: PlacementStrategy;
	cloud: CloudForm;
	recoveryKey: string;
	recoveryAcknowledged: boolean;
}

export interface SetupStepMeta {
	id: SetupStepId;
	title: string;
	description: string;
}

export const SETUP_STEPS: SetupStepMeta[] = [
	{ id: 1, title: 'Akun Super Admin', description: 'Buat akun administrator pertama' },
	{ id: 2, title: 'Penyimpanan Lokal', description: 'Pilih node penyimpanan' },
	{ id: 3, title: 'Cloud', description: 'Hubungkan penyimpanan S3 Compatible' },
	{ id: 4, title: 'Strategi Penempatan', description: 'Atur alokasi berkas' },
	{ id: 5, title: 'Konfirmasi', description: 'Tinjau konfigurasi dan selesaikan setup' }
];

export const STRATEGY_OPTIONS: {
	value: PlacementStrategy;
	label: string;
	description: string;
}[] = [
	{
		value: 'FILL_FIRST',
		label: 'Fill First',
		description: 'Isi disk pertama sampai penuh sebelum beralih ke disk berikutnya.'
	},
	{
		value: 'ROUND_ROBIN',
		label: 'Round Robin',
		description: 'Bagagi alokasi berkas secara bergiliran ke seluruh disk aktif.'
	},
	{
		value: 'MOST_FREE_SPACE',
		label: 'Most Free Space',
		description: 'Selalu pilih disk dengan sisa kapasitas fisik terbesar.'
	}
];

export const PROVIDER_OPTIONS: {
	value: CloudProvider;
	label: string;
	description: string;
	endpointPlaceholder: string;
}[] = [
	{
		value: 'S3_COMPATIBLE',
		label: 'S3 Compatible',
		description: 'MinIO, Wasabi, Backblaze B2, Cloudflare R2, AWS S3, dan penyedia S3 API lain.',
		endpointPlaceholder: 'https://s3.example.com'
	}
];
