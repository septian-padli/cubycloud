import type { FileEntry, ShareLinkRecord } from '$lib/types/drive.js';
import { MOCK_DRIVE_USER } from '$lib/mocks/drive.js';
import { MOCK_ADDONS } from '$lib/mocks/addons.js';
import {
	MOCK_MY_STORAGE_TREE,
	MOCK_ORG_TREE,
	findEntry,
	getChildren
} from '$lib/mocks/file-tree.js';
import { MOCK_SHARED_FILES } from '$lib/mocks/shared-files.js';

const ALL_TREES: FileEntry[] = [...MOCK_MY_STORAGE_TREE, ...MOCK_ORG_TREE, ...MOCK_SHARED_FILES];

function daysFromNow(days: number): string {
	return new Date(Date.now() + days * 86_400_000).toISOString();
}

function daysAgo(days: number): string {
	return new Date(Date.now() - days * 86_400_000).toISOString();
}

/** Demo tokens for QA. Password mock: `demo123`. */
const linksByToken = new Map<string, ShareLinkRecord>([
	[
		'demo-public',
		{
			token: 'demo-public',
			entryId: 'ms-file-demo',
			entryName: 'Demo Setup.mp4',
			entryKind: 'video',
			access: 'public',
			password: null,
			expiresAt: daysFromNow(7),
			ownerId: MOCK_DRIVE_USER.id,
			ownerName: MOCK_DRIVE_USER.displayName
		}
	],
	[
		'demo-folder',
		{
			token: 'demo-folder',
			entryId: 'ms-folder-proyek',
			entryName: 'Proyek Q3',
			entryKind: 'folder',
			access: 'public',
			password: null,
			expiresAt: daysFromNow(30),
			ownerId: MOCK_DRIVE_USER.id,
			ownerName: MOCK_DRIVE_USER.displayName
		}
	],
	[
		'demo-password',
		{
			token: 'demo-password',
			entryId: 'ms-file-brief',
			entryName: 'Brief Produk.pdf',
			entryKind: 'document',
			access: 'public',
			password: 'demo123',
			expiresAt: daysFromNow(7),
			ownerId: MOCK_DRIVE_USER.id,
			ownerName: MOCK_DRIVE_USER.displayName
		}
	],
	[
		'demo-restricted',
		{
			token: 'demo-restricted',
			entryId: 'org-file-handbook',
			entryName: 'Employee Handbook.pdf',
			entryKind: 'document',
			access: 'restricted',
			password: null,
			expiresAt: daysFromNow(14),
			ownerId: MOCK_DRIVE_USER.id,
			ownerName: MOCK_DRIVE_USER.displayName
		}
	],
	[
		'demo-expired',
		{
			token: 'demo-expired',
			entryId: 'ms-file-rencana',
			entryName: 'Rencana Q3.md',
			entryKind: 'document',
			access: 'public',
			password: null,
			expiresAt: daysAgo(1),
			ownerId: MOCK_DRIVE_USER.id,
			ownerName: MOCK_DRIVE_USER.displayName
		}
	]
]);

export function findShareLink(token: string): ShareLinkRecord | undefined {
	const link = linksByToken.get(token);
	return link ? { ...link } : undefined;
}

export function isShareExpired(link: ShareLinkRecord): boolean {
	if (!link.expiresAt) return false;
	return Date.parse(link.expiresAt) <= Date.now();
}

export function findSharedEntry(entryId: string): FileEntry | undefined {
	return findEntry(ALL_TREES, entryId);
}

export function getShareChildren(folderId: string): FileEntry[] {
	const fromMy = getChildren(MOCK_MY_STORAGE_TREE, folderId);
	if (fromMy.length > 0 || findEntry(MOCK_MY_STORAGE_TREE, folderId)) return fromMy;
	const fromOrg = getChildren(MOCK_ORG_TREE, folderId);
	if (fromOrg.length > 0 || findEntry(MOCK_ORG_TREE, folderId)) return fromOrg;
	return getChildren(MOCK_SHARED_FILES, folderId);
}

export function createShareLink(
	entry: FileEntry,
	input: Pick<ShareLinkRecord, 'access' | 'password' | 'expiresAt'>
): ShareLinkRecord {
	const token = `lnk-${Date.now().toString(36)}`;
	const record: ShareLinkRecord = {
		token,
		entryId: entry.id,
		entryName: entry.name,
		entryKind: entry.kind,
		access: input.access,
		password: input.password,
		expiresAt: input.expiresAt,
		ownerId: MOCK_DRIVE_USER.id,
		ownerName: MOCK_DRIVE_USER.displayName
	};
	linksByToken.set(token, record);
	return { ...record };
}

export function shareUrl(token: string): string {
	return `/s/${token}`;
}

export function isDownloadZipEnabled(): boolean {
	return MOCK_ADDONS.find((a) => a.id === 'download_zip')?.enabled === true;
}

export const DEMO_SHARE_PASSWORD = 'demo123';
