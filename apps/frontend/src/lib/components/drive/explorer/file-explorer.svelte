<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { onDestroy, onMount, untrack } from 'svelte';
	import { fly } from 'svelte/transition';
	import { locale } from '$lib/locale.svelte.js';
	const tx = (id: string, en: string) => (locale.language === 'id' ? id : en);
	const formatModifiedAt = (value: string) =>
		value === 'Baru saja' ? tx('Baru saja', 'Just now') : value;
	import { toast } from 'svelte-sonner';
	import type { ExplorerVariant, FileCategory, FileEntry, FileKind } from '$lib/types/drive.js';
	import { getDriveChrome } from '$lib/drive-context.svelte.js';
	import {
		cloneTree,
		findEntry,
		formatSize,
		getBreadcrumbPath,
		getChildren,
		getFolderOptions,
		isAncestor,
		nextId
	} from '$lib/mocks/file-tree.js';
	import { myStorageAPI } from '$lib/api/my-storage.js';
	import { organizationAPI } from '$lib/api/organization.js';
	import { recordItemAccess } from '$lib/api/drive-home.js';
	import {
		cancelTransfer,
		enqueueTransfers,
		failPendingTransfers,
		getInMemoryTransferFile,
		isTransferQuotaError,
		loadTransferQueue,
		pauseTransfer,
		restartTransfer,
		resumeFailedTransfer,
		runTransfer,
		transferQueueState,
		type TransferCancelDriver,
		type TransferDriver,
		type TransferRecord,
		type TransferState
	} from '$lib/transfer-queue.svelte.js';
	import { cn } from '$lib/utils.js';
	import { extractDroppedEntries } from '$lib/utils/drag-drop.js';
	import FileExplorerBreadcrumbs from './file-explorer-breadcrumbs.svelte';
	import FileExplorerToolbar from './file-explorer-toolbar.svelte';
	import FileExplorerRow from './file-explorer-row.svelte';
	import FileExplorerDropzone from './file-explorer-dropzone.svelte';
	import FileExplorerActionsMenu from './file-explorer-actions-menu.svelte';
	import FileExplorerContextMenu from './file-explorer-context-menu.svelte';
	import NewFolderDialog from './new-folder-dialog.svelte';
	import RenameDialog from './rename-dialog.svelte';
	import MoveDialog from './move-dialog.svelte';
	import ConflictResolutionDialog from './conflict-resolution-dialog.svelte';
	import { ConfirmDialog } from '$lib/components/ui/confirm-dialog/index.js';
	import ShareSheet from './share-sheet.svelte';
	import FilePreviewDialog from './file-preview-dialog.svelte';
	import { previewUrl } from '$lib/api/preview.js';
	import DriveFileGrid from '$lib/components/drive/drive-file-grid.svelte';
	import FileGridCard from '$lib/components/drive/file-grid-card.svelte';
	import OfflineFileNotice from './offline-file-notice.svelte';
	import { driveView } from '$lib/drive-view.svelte.js';
	import {
		ChevronDown,
		ChevronUp,
		Download,
		FolderInput,
		MoreVertical,
		Star,
		Trash2,
		X
	} from '@lucide/svelte';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Checkbox } from '$lib/components/ui/checkbox/index.js';

	type DownloadQueueItem = {
		name: string;
		size: number;
		progress: number;
		state: 'uploading' | 'started' | 'failed';
		error?: string;
	};

	type QueueDisplayItem = {
		id: string;
		kind: 'upload' | 'download';
		name: string;
		size: number;
		progress: number;
		state: TransferState | 'done' | 'started';
		error?: string;
		transfer?: TransferRecord;
	};
	let {
		variant,
		initialTree,
		rootLabel
	}: {
		variant: ExplorerVariant;
		initialTree: FileEntry[];
		rootLabel: string;
	} = $props();

	const chrome = getDriveChrome();
	const isApiBacked = $derived(variant === 'my-storage' || variant === 'organization');
	const api = $derived(variant === 'organization' ? organizationAPI : myStorageAPI);
	const transferDriver: TransferDriver = (file, record, options) =>
		api.uploadResumable(file, record, options);
	const transferCancelDriver: TransferCancelDriver = (record) => api.cancelResumable(record);

	let tree = $state<FileEntry[]>(untrack(() => cloneTree(initialTree)));
	let currentItems = $state<FileEntry[]>([]);
	let apiBreadcrumbs = $state<{ id: string; name: string }[]>([]);
	let allFolders = $state<FileEntry[]>([]);
	let storageLoading = $state(false);
	let downloadQueue = $state<DownloadQueueItem[]>([]);
	const uploadQueue = $derived(
		transferQueueState.items.filter(
			(item) => item.ownerId === chrome.user.id && item.area === variant
		)
	);
	let queueOpen = $state(false);
	let queueDismissed = $state(false);
	let queueFileInput: HTMLInputElement | undefined = $state();
	let pendingQueueTransferId = $state<string | null>(null);
	let queueStartX = $state<number | null>(null);
	let queueSwipeOffset = $state(0);
	let isSwipingQueue = $state(false);
	const queueEntries = $derived<QueueDisplayItem[]>([
		...uploadQueue.map((item) => ({
			id: item.id,
			kind: 'upload' as const,
			name: item.relativePath || item.name,
			size: item.size,
			progress: item.progress,
			state: item.state,
			error: item.error,
			transfer: item
		})),
		...downloadQueue.map((item, index) => ({
			id: `zip-${index}`,
			kind: 'download' as const,
			name: item.name,
			size: item.size,
			progress: item.progress,
			state: item.state,
			error: item.error
		}))
	]);
	const queueHasActive = $derived(
		queueEntries.some((item) =>
			['queued', 'uploading', 'retrying', 'verifying', 'committing'].includes(item.state)
		)
	);
	const queueCanDismiss = $derived(
		queueEntries.length > 0 &&
			queueEntries.every((item) =>
				['completed', 'done', 'started', 'failed', 'expired'].includes(item.state)
			)
	);
	const queueHasProgress = $derived(
		uploadQueue.some((item) =>
			[
				'queued',
				'uploading',
				'retrying',
				'paused',
				'waiting-for-file',
				'verifying',
				'committing',
				'completed'
			].includes(item.state)
		)
	);
	const queueCompletedCount = $derived(
		queueEntries.filter((item) => ['completed', 'done', 'started'].includes(item.state)).length
	);
	const queueHeading = $derived.by(() => {
		const states = uploadQueue.map((item) => item.state);
		if (states.includes('committing')) return tx('Menyimpan', 'Saving');
		if (states.includes('verifying')) return tx('Memverifikasi', 'Verifying');
		if (states.includes('uploading')) return tx('Mengunggah', 'Uploading');
		if (states.includes('retrying')) return tx('Mencoba kembali', 'Retrying');
		if (states.includes('queued')) return tx('Menunggu giliran', 'Queued');
		if (states.includes('paused') || states.includes('waiting-for-file'))
			return tx('Perlu tindakan', 'Action required');
		if (states.includes('failed') || states.includes('expired'))
			return tx('Ada masalah', 'Something went wrong');
		if (downloadQueue.some((item) => item.state === 'uploading'))
			return tx('Memproses', 'Processing');
		if (downloadQueue.some((item) => item.state === 'started'))
			return tx('Unduhan dimulai', 'Download started');
		return tx('Selesai', 'Complete');
	});
	const uploadProgress = $derived.by(() => {
		let totalWeight = 0;
		let transferredWeight = 0;
		for (const item of uploadQueue) {
			const weight = Math.max(item.size, 1);
			totalWeight += weight;
			transferredWeight += (weight * item.progress) / 100;
		}
		if (!uploadQueue.length) return 0;
		const settled = uploadQueue.every((item) =>
			['completed', 'failed', 'expired'].includes(item.state)
		);
		const successful = uploadQueue.every((item) => item.state === 'completed');
		if (settled && successful) return 100;
		if (!totalWeight) return 0;
		return Math.min(Math.floor((transferredWeight / totalWeight) * 100), 99);
	});
	let category = $state<FileCategory>('all');
	let deleteConfirmOpen = $state(false);
	let deleteTargetEntry = $state<FileEntry | null>(null);
	let conflictConfirmOpen = $state(false);
	let conflictTitle = $state({ id: 'Folder Sudah Ada', en: 'Folder Already Exists' });
	let conflictDescription = $state({ id: '', en: '' });
	const conflictTitleText = $derived(tx(conflictTitle.id, conflictTitle.en));
	const conflictDescriptionText = $derived(tx(conflictDescription.id, conflictDescription.en));
	let conflictAction = $state<(() => Promise<void>) | null>(null);
	let bulkDeleteEntries = $state<FileEntry[]>([]);
	let moveMultipleEntries = $state<FileEntry[]>([]);
	let selectedIds = $state<string[]>([]);

	let dragActive = $state(false);
	let dragDepth = 0;

	let newFolderOpen = $state(false);
	let renameOpen = $state(false);
	let moveOpen = $state(false);
	let conflictOpen = $state(false);
	let shareOpen = $state(false);

	let moveConflictOpen = $state(false);
	let moveConflictEntry = $state<FileEntry | null>(null);
	let moveConflictTargetFolder = $state<FileEntry | null>(null);

	let actionEntry = $state<FileEntry | null>(null);
	let pendingUpload = $state<{ name: string; size: number; kind: FileKind } | null>(null);
	let fileInput: HTMLInputElement | undefined = $state();
	let folderInput: HTMLInputElement | undefined = $state();

	const folderId = $derived(page.url.searchParams.get('folder'));
	const previewId = $derived(page.url.searchParams.get('preview'));

	const breadcrumbs = $derived(
		isApiBacked
			? [
					{ id: null, label: rootLabel },
					...apiBreadcrumbs.map((item) => ({ id: item.id, label: item.name }))
				]
			: getBreadcrumbPath(tree, folderId, rootLabel)
	);
	const children = $derived(isApiBacked ? currentItems : getChildren(tree, folderId));
	const visible = $derived(
		category === 'all' ? children : children.filter((e) => e.kind === category)
	);
	const selectedCount = $derived(selectedIds.length);
	const selectableItems = $derived(
		visible.filter((item) => item.kind === 'folder' || item.storageAvailable !== false)
	);
	const hasUnavailableSelection = $derived(
		visible.some(
			(entry) =>
				selectedIds.includes(entry.id) &&
				entry.kind !== 'folder' &&
				entry.storageAvailable === false
		)
	);
	const isAllSelected = $derived(
		selectableItems.length > 0 && selectableItems.every((item) => selectedIds.includes(item.id))
	);
	const isPartiallySelected = $derived(
		selectableItems.some((item) => selectedIds.includes(item.id)) && !isAllSelected
	);

	function toggleSelectEntry(id: string) {
		const entry = visible.find((item) => item.id === id);
		if (entry?.kind !== 'folder' && entry?.storageAvailable === false) return;
		if (selectedIds.includes(id)) {
			selectedIds = selectedIds.filter((item) => item !== id);
		} else {
			selectedIds = [...selectedIds, id];
		}
	}

	function toggleSelectAll() {
		if (isAllSelected) {
			const selectableIds = new Set(selectableItems.map((item) => item.id));
			selectedIds = selectedIds.filter((id) => !selectableIds.has(id));
		} else {
			selectedIds = [...new Set([...selectedIds, ...selectableItems.map((item) => item.id)])];
		}
	}

	function handleBulkMove() {
		const selected = visible.filter((e) => selectedIds.includes(e.id));
		if (
			selected.length === 0 ||
			selected.some((e) => e.kind !== 'folder' && e.storageAvailable === false)
		)
			return;
		actionEntry = null;
		moveMultipleEntries = selected;
		moveOpen = true;
	}

	function handleBulkDelete() {
		const selected = visible.filter((e) => selectedIds.includes(e.id));
		if (
			selected.length === 0 ||
			selected.some((e) => e.kind !== 'folder' && e.storageAvailable === false)
		)
			return;
		deleteTargetEntry = null;
		bulkDeleteEntries = selected;
		deleteConfirmOpen = true;
	}

	async function handleBulkDownload() {
		const selected = visible.filter((e) => selectedIds.includes(e.id));
		if (
			selected.length === 0 ||
			selected.some((e) => e.kind !== 'folder' && e.storageAvailable === false)
		)
			return;
		if (!isApiBacked) {
			toast.info(
				tx(
					`Mengunduh ${selected.length} item sebagai ZIP (mock)`,
					`Downloading ${selected.length} items as ZIP (mock)`
				)
			);
			return;
		}
		const queueIndex = downloadQueue.length;
		const totalSize = selected.reduce((acc, e) => acc + (e.sizeBytes || 1), 0);
		downloadQueue = [
			...downloadQueue,
			{
				name:
					selected.length === 1
						? tx(`Men-zip-kan ${selected[0].name}`, `Zipping ${selected[0].name}`)
						: tx(`Men-zip-kan ${selected.length} item`, `Zipping ${selected.length} items`),
				size: totalSize,
				progress: 0,
				state: 'uploading'
			}
		];
		queueDismissed = false;
		queueOpen = true;

		try {
			api.downloadBatchZip(
				selected.map((e) => e.id),
				(message) => {
					downloadQueue[queueIndex] = {
						...downloadQueue[queueIndex],
						state: 'failed',
						error: message
					};
					toast.error(message);
				}
			);
			downloadQueue[queueIndex] = {
				...downloadQueue[queueIndex],
				state: 'started'
			};
			toast.info(tx('Pengunduhan berkas ZIP dimulai', 'ZIP file download started'));
		} catch (error) {
			downloadQueue[queueIndex] = {
				...downloadQueue[queueIndex],
				state: 'failed',
				error:
					error instanceof Error ? error.message : tx('Gagal men-zip berkas', 'Failed to zip files')
			};
			toast.error(
				error instanceof Error ? error.message : tx('Gagal mengunduh ZIP', 'Failed to download ZIP')
			);
		}
	}
	let fetchedPreviewEntry = $state<FileEntry | null>(null);
	let lastFetchedPreviewId = $state<string | null>(null);

	$effect(() => {
		const targetId = previewId;
		if (!targetId) {
			fetchedPreviewEntry = null;
			lastFetchedPreviewId = null;
			return;
		}
		if (!isApiBacked) return;

		const inCurrent = currentItems.find((entry) => entry.id === targetId);
		if (inCurrent) {
			if (fetchedPreviewEntry) fetchedPreviewEntry = null;
			return;
		}

		if (fetchedPreviewEntry?.id !== targetId && lastFetchedPreviewId !== targetId) {
			lastFetchedPreviewId = targetId;
			api
				.getItem(targetId)
				.then((item) => {
					if (previewId === targetId && item) {
						fetchedPreviewEntry = api.toEntry(item);
					}
				})
				.catch(() => {
					// Item tidak ditemukan atau link tidak valid
				});
		}
	});

	const previewEntry = $derived(
		previewId
			? isApiBacked
				? (currentItems.find((entry) => entry.id === previewId) ?? fetchedPreviewEntry)
				: (findEntry(tree, previewId) ?? null)
			: null
	);
	const previewOpen = $derived(
		!!previewEntry && (previewEntry.kind === 'folder' || previewEntry.storageAvailable !== false)
	);
	const moveFolders = $derived(
		isApiBacked
			? allFolders.filter((f) => f.id !== actionEntry?.id)
			: getFolderOptions(tree).filter((f) => f.id !== actionEntry?.id)
	);

	async function refreshStorage() {
		storageLoading = true;
		try {
			const [listing, folders] = await Promise.all([api.list(folderId), api.folders()]);
			currentItems = listing.items.map((item) => api.toEntry(item));
			selectedIds = selectedIds.filter((id) =>
				currentItems.some(
					(item) => item.id === id && (item.kind === 'folder' || item.storageAvailable !== false)
				)
			);
			apiBreadcrumbs = listing.breadcrumbs;
			allFolders = folders.items.map((item) => ({
				id: item.id,
				name: item.name,
				kind: 'folder' as const,
				sizeBytes: 0,
				parentId: item.parent_id,
				modifiedAt: ''
			}));
		} catch (error) {
			toast.error(
				error instanceof Error
					? error.message
					: tx(`Gagal memuat ${rootLabel}`, `Failed to load ${rootLabel}`)
			);
		} finally {
			storageLoading = false;
		}
	}

	$effect(() => {
		if (isApiBacked) void refreshStorage();
	});
	function syncQuery(next: { folder?: string | null; preview?: string | null }) {
		const params = new URLSearchParams(page.url.searchParams);
		if ('folder' in next) {
			if (next.folder) params.set('folder', next.folder);
			else params.delete('folder');
		}
		if ('preview' in next) {
			if (next.preview) params.set('preview', next.preview);
			else params.delete('preview');
		}
		const qs = params.toString();
		void goto(`${page.url.pathname}${qs ? `?${qs}` : ''}`, {
			replaceState: true,
			keepFocus: true,
			noScroll: true
		});
	}

	function navigateFolder(id: string | null) {
		selectedIds = [];
		syncQuery({ folder: id, preview: null });
	}

	function openEntry(entry: FileEntry) {
		if (entry.kind !== 'folder' && entry.storageAvailable === false) return;
		const source = variant === 'organization' ? 'organization' : 'my-storage';
		void recordItemAccess(entry.id, source);
		if (entry.kind === 'folder') {
			navigateFolder(entry.id);
			return;
		}
		syncQuery({ preview: entry.id });
	}

	function closePreview() {
		syncQuery({ preview: null });
	}

	function guessKind(name: string): FileKind {
		const ext = name.split('.').pop()?.toLowerCase() ?? '';
		if (['png', 'jpg', 'jpeg', 'gif', 'webp', 'svg', 'heic'].includes(ext)) return 'image';
		if (['mp4', 'webm', 'mov', 'mkv'].includes(ext)) return 'video';
		if (['mp3', 'wav', 'ogg', 'm4a'].includes(ext)) return 'audio';
		if (['zip', 'rar', '7z', 'tar', 'gz'].includes(ext)) return 'archive';
		if (['pdf', 'doc', 'docx', 'md'].includes(ext)) return 'document';
		if (['txt', 'csv', 'json', 'sql'].includes(ext)) return 'text';
		return 'document';
	}

	function findSiblingByName(name: string): FileEntry | undefined {
		return tree.find((e) => e.parentId === folderId && e.name === name);
	}

	function addFile(name: string, size: number, kind: FileKind, replaceId?: string) {
		if (replaceId) {
			tree = tree.map((e) =>
				e.id === replaceId ? { ...e, name, sizeBytes: size, kind, modifiedAt: 'Baru saja' } : e
			);
		} else {
			tree = [
				...tree,
				{
					id: nextId(variant === 'my-storage' ? 'ms' : 'org'),
					name,
					kind,
					sizeBytes: size,
					modifiedAt: 'Baru saja',
					parentId: folderId,
					storageNode: variant === 'my-storage' ? 'ssd' : 'ssd'
				}
			];
		}
		toast.success(tx(`“${name}” diunggah (mock)`, `“${name}” uploaded (mock)`));
	}

	async function handleUploadFiles(
		files: FileList | File[],
		paths?: string[],
		targetParentId?: string | null,
		emptyFolders?: string[]
	) {
		const list = Array.from(files);
		const effectiveParentId = targetParentId !== undefined ? targetParentId : folderId;

		if (!list.length && (!emptyFolders || !emptyFolders.length)) return;

		if (!isApiBacked) {
			if (list.length > 0) {
				const file = list[0];
				const existing = findSiblingByName(file.name);
				if (existing) {
					pendingUpload = { name: file.name, size: file.size, kind: guessKind(file.name) };
					actionEntry = existing;
					conflictOpen = true;
					return;
				}
				addFile(file.name, file.size, guessKind(file.name));
			}
			return;
		}

		const folderPaths = new Map<string, string>();
		const pathErrors: string[] = [];
		if (effectiveParentId) folderPaths.set('', effectiveParentId);

		async function ensureFolderPath(relPath: string): Promise<string | null> {
			const parts = relPath.replace(/\\/g, '/').split('/').filter(Boolean);
			let parent = effectiveParentId;
			let accumulated = '';
			for (const part of parts) {
				accumulated = accumulated ? `${accumulated}/${part}` : part;
				if (!folderPaths.has(accumulated)) {
					const folder = await api.findOrCreateFolder(part, parent, allFolders);
					folderPaths.set(accumulated, folder.id);
					if (!allFolders.some((f) => f.id === folder.id)) {
						allFolders = [
							...allFolders,
							{
								id: folder.id,
								name: folder.name,
								kind: 'folder',
								sizeBytes: 0,
								modifiedAt: folder.modified_at || 'Baru saja',
								parentId: folder.parent_id
							}
						];
					}
				}
				parent = folderPaths.get(accumulated)!;
			}
			return parent;
		}

		// 1. Process any empty folders
		if (emptyFolders && emptyFolders.length > 0) {
			for (const emptyRelPath of emptyFolders) {
				try {
					await ensureFolderPath(emptyRelPath);
				} catch (error) {
					pathErrors.push(
						`${emptyRelPath}: ${error instanceof Error ? error.message : tx('Gagal membuat folder', 'Failed to create folder')}`
					);
				}
			}
		}

		// If there were only empty folders without files:
		if (!list.length) {
			await refreshStorage();
			if (pathErrors.length) {
				toast.error(
					tx(
						`${pathErrors.length} folder gagal dibuat`,
						`${pathErrors.length} folders failed to create`
					),
					{ description: pathErrors[0] }
				);
			} else {
				toast.success(tx('Folder berhasil dibuat', 'Folder created successfully'), {
					description: tx(
						`${emptyFolders!.length} folder dibuat.`,
						`${emptyFolders!.length} folders created.`
					)
				});
			}
			return;
		}

		if (!chrome.user.id) {
			toast.error(
				tx(
					'Sesi pengguna tidak tersedia. Silakan masuk kembali.',
					'User session is unavailable. Please sign in again.'
				)
			);
			return;
		}

		const prepared: { name: string; file: File; parentId: string | null; relativePath: string }[] =
			[];
		for (let index = 0; index < list.length; index += 1) {
			const file = list[index];
			try {
				let parentId = effectiveParentId;
				const relativePath = paths?.[index]?.replace(/\\/g, '/') || file.name;
				if (paths?.[index]) {
					const parts = relativePath.split('/').filter(Boolean);
					parts.pop();
					if (parts.length > 0) parentId = await ensureFolderPath(parts.join('/'));
				}
				prepared.push({ name: file.name, file, parentId, relativePath });
			} catch (error) {
				pathErrors.push(
					`${paths?.[index] || file.name}: ${error instanceof Error ? error.message : tx('Gagal menyiapkan folder tujuan', 'Failed to prepare destination folder')}`
				);
			}
		}

		if (!prepared.length) {
			if (pathErrors.length) {
				toast.error(
					tx(
						`${pathErrors.length} item tidak dapat disiapkan`,
						`${pathErrors.length} items could not be prepared`
					),
					{
						description: pathErrors[0]
					}
				);
			}
			return;
		}

		let transferIds: string[];
		try {
			transferIds = await enqueueTransfers(chrome.user.id, variant, prepared);
		} catch (error) {
			toast.error(
				error instanceof Error
					? error.message
					: tx('Gagal menyimpan antrean unggahan', 'Failed to save upload queue')
			);
			return;
		}
		queueDismissed = false;

		let quotaAborted = false;
		for (let index = 0; index < transferIds.length; index += 1) {
			const id = transferIds[index];
			await runTransfer(id, prepared[index].file, transferDriver, transferCancelDriver);
			const current = transferQueueState.items.find((item) => item.id === id);
			if (current && isTransferQuotaError(current)) {
				quotaAborted = true;
				await failPendingTransfers(
					transferIds.slice(index + 1),
					tx('Dibatalkan: kuota penyimpanan tidak cukup.', 'Cancelled: insufficient storage quota.')
				);
				break;
			}
		}

		await refreshStorage();
		if (variant === 'my-storage') await chrome.refreshQuota();
		const failed = transferIds.filter((id) => {
			const current = transferQueueState.items.find((item) => item.id === id);
			return current?.state === 'failed' || current?.state === 'expired';
		}).length;
		if (quotaAborted) {
			toast.error(
				tx('Unggahan dihentikan: kuota penyimpanan penuh', 'Upload stopped: storage quota is full'),
				{
					description: tx(
						'Sebagian berkas gagal diunggah karena batas kuota terlampaui.',
						'Some files could not be uploaded because the quota limit was exceeded.'
					)
				}
			);
		} else {
			const failedTotal = failed + pathErrors.length;
			toast[failedTotal ? 'warning' : 'success'](
				failedTotal
					? tx(`${failedTotal} unggahan gagal`, `${failedTotal} uploads failed`)
					: tx('Unggahan selesai', 'Upload complete'),
				{
					description: failedTotal
						? tx('Periksa daftar unggahan untuk detail.', 'Check the upload list for details.')
						: tx(
								`${prepared.length} item berhasil diunggah.`,
								`${prepared.length} items uploaded successfully.`
							)
				}
			);
		}
	}

	async function executeQueuedUpload(id: string, file: File) {
		const succeeded = await runTransfer(id, file, transferDriver, transferCancelDriver, true);
		const current = transferQueueState.items.find((item) => item.id === id);
		if (succeeded) {
			await refreshStorage();
			if (variant === 'my-storage') await chrome.refreshQuota();
		} else if (current?.state === 'waiting-for-file' && current.error) {
			toast.error(current.error);
		} else if (
			current &&
			['uploading', 'retrying', 'verifying', 'committing'].includes(current.state) &&
			!getInMemoryTransferFile(id)
		) {
			toast.info(
				tx(
					'Unggahan ini masih berjalan di tab lain. Coba lagi setelah transfer tersebut berhenti.',
					'This upload is still running in another tab. Try again after that transfer stops.'
				)
			);
		}
	}

	function chooseQueuedFile(id: string) {
		pendingQueueTransferId = id;
		queueFileInput?.click();
	}

	async function handleQueuedFileSelection(event: Event) {
		const input = event.currentTarget as HTMLInputElement;
		const file = input.files?.[0];
		const id = pendingQueueTransferId;
		input.value = '';
		pendingQueueTransferId = null;
		if (id && file) await executeQueuedUpload(id, file);
	}

	async function resumeQueuedUpload(id: string) {
		const record = transferQueueState.items.find((item) => item.id === id);
		if (!record) return;
		if (record.state === 'expired' || (record.state === 'failed' && !record.session)) {
			await restartQueuedUpload(id);
			return;
		}
		if (record.state === 'failed' && !(await resumeFailedTransfer(id))) {
			toast.error(
				tx(
					'Sesi unggahan tidak dapat dilanjutkan. Mulai ulang dari nol.',
					'The upload session cannot be resumed. Start over from scratch.'
				)
			);
			return;
		}
		const file = getInMemoryTransferFile(id);
		if (file) await executeQueuedUpload(id, file);
		else chooseQueuedFile(id);
	}

	async function restartQueuedUpload(id: string) {
		if (!(await restartTransfer(id, transferCancelDriver))) {
			toast.error(
				tx(
					'Sesi unggahan tidak dapat dihapus dengan aman. Coba lagi sebelum memulai ulang.',
					'The upload session could not be safely removed. Try again before restarting.'
				)
			);
			return;
		}
		chooseQueuedFile(id);
	}

	async function pauseQueuedUpload(id: string) {
		if (!(await pauseTransfer(id)))
			toast.error(
				tx('Unggahan tidak dapat dijeda sekarang.', 'The upload cannot be paused right now.')
			);
	}

	async function cancelQueuedUpload(id: string) {
		if (!(await cancelTransfer(id, transferCancelDriver))) {
			toast.error(
				tx(
					'Unggahan belum dapat dibatalkan dengan aman.',
					'The upload cannot be safely cancelled yet.'
				)
			);
		}
	}

	function queueStateLabel(item: QueueDisplayItem): string {
		if (item.kind === 'download') {
			if (item.state === 'uploading') return tx('Memproses', 'Processing');
			if (item.state === 'failed') return tx('Gagal', 'Failed');
			return tx('Dimulai', 'Started');
		}
		switch (item.state) {
			case 'queued':
				return tx('Menunggu giliran', 'Queued');
			case 'uploading':
				return tx('Mengunggah', 'Uploading');
			case 'retrying':
				return tx('Mencoba kembali', 'Retrying');
			case 'paused':
				return tx('Dijeda', 'Paused');
			case 'waiting-for-file':
				return tx('Menunggu file', 'Waiting for file');
			case 'verifying':
				return tx('Memverifikasi', 'Verifying');
			case 'committing':
				return tx('Menyimpan', 'Saving');
			case 'completed':
				return tx('Selesai', 'Completed');
			case 'failed':
				return tx('Gagal', 'Failed');
			case 'expired':
				return tx('Kedaluwarsa', 'Expired');
			case 'done':
				return tx('Selesai', 'Completed');
			case 'started':
				return tx('Dimulai', 'Started');
		}
	}

	function transferHasSession(item: QueueDisplayItem): boolean {
		return Boolean(item.transfer?.session);
	}

	function transferCanPause(item: QueueDisplayItem): boolean {
		return item.kind === 'upload' && ['uploading', 'retrying', 'verifying'].includes(item.state);
	}

	function transferCanCancel(item: QueueDisplayItem): boolean {
		return item.kind === 'upload' && !['completed', 'committing'].includes(item.state);
	}

	function dismissQueue() {
		if (queueCanDismiss) {
			queueDismissed = true;
			queueOpen = false;
		}
	}

	function transferActionLabel(item: QueueDisplayItem): string {
		if (item.state === 'expired' || (item.state === 'failed' && !transferHasSession(item))) {
			return tx('Mulai ulang dari nol', 'Start over from scratch');
		}
		if (item.state === 'failed' && transferHasSession(item))
			return tx('Pilih file untuk melanjutkan', 'Choose a file to resume');
		if (
			item.state === 'waiting-for-file' ||
			(['uploading', 'retrying', 'verifying', 'committing'].includes(item.state) &&
				!getInMemoryTransferFile(item.id))
		)
			return tx('Pilih file', 'Choose file');
		return tx('Lanjutkan', 'Resume');
	}

	function triggerUpload() {
		fileInput?.click();
	}

	function triggerFolderUpload() {
		folderInput?.click();
	}

	onMount(() => {
		if (isApiBacked && chrome.user.id) {
			void loadTransferQueue(chrome.user.id, variant).catch((error) => {
				toast.error(
					error instanceof Error
						? error.message
						: tx('Gagal memuat antrean transfer tersimpan', 'Failed to load saved transfer queue')
				);
			});
		}
		chrome.registerUploadHandler(triggerUpload);
		chrome.registerStorageChangeHandler?.(refreshStorage);
		function handleDragEnter(e: DragEvent) {
			const types = e.dataTransfer?.types || [];
			if (!types.includes('Files')) return;
			e.preventDefault();
			dragDepth += 1;
			dragActive = true;
		}

		function handleDragOver(e: DragEvent) {
			const types = e.dataTransfer?.types || [];
			if (!types.includes('Files')) return;
			e.preventDefault();
			if (e.dataTransfer) e.dataTransfer.dropEffect = 'copy';
		}

		function handleDragLeave(e: DragEvent) {
			const types = e.dataTransfer?.types || [];
			if (!types.includes('Files')) return;
			e.preventDefault();
			dragDepth = Math.max(0, dragDepth - 1);
			if (dragDepth === 0) dragActive = false;
		}

		async function handleDrop(e: DragEvent) {
			const types = e.dataTransfer?.types || [];
			if (!types.includes('Files')) return;
			e.preventDefault();
			dragDepth = 0;
			dragActive = false;
			if (e.dataTransfer) {
				const result = await extractDroppedEntries(e.dataTransfer);
				if (result.files.length > 0 || result.emptyFolders.length > 0) {
					void handleUploadFiles(result.files, result.paths, folderId, result.emptyFolders);
				}
			}
		}

		window.addEventListener('dragenter', handleDragEnter);
		window.addEventListener('dragover', handleDragOver);
		window.addEventListener('dragleave', handleDragLeave);
		window.addEventListener('drop', handleDrop);

		return () => {
			window.removeEventListener('dragenter', handleDragEnter);
			window.removeEventListener('dragover', handleDragOver);
			window.removeEventListener('dragleave', handleDragLeave);
			window.removeEventListener('drop', handleDrop);
		};
	});

	onDestroy(() => {
		chrome.registerUploadHandler(null);
		chrome.registerStorageChangeHandler?.(null);
		draggedEntryId = null;
		if (longPressTimer) {
			clearTimeout(longPressTimer);
			longPressTimer = null;
		}
	});
	async function onNewFolder(name: string) {
		if (isApiBacked) {
			try {
				await api.createFolder(name, folderId);
				await refreshStorage();
				toast.success(tx(`Folder “${name}” dibuat`, `Folder “${name}” created`));
			} catch (error) {
				const conflict = error as Error & { status?: number; suggested_name?: string };
				if (conflict.status === 409 && conflict.suggested_name) {
					conflictTitle = { id: 'Folder Sudah Ada', en: 'Folder Already Exists' };
					conflictDescription = {
						id: `Folder “${name}” sudah ada. Gunakan nama yang disarankan “${conflict.suggested_name}”?`,
						en: `Folder “${name}” already exists. Use the suggested name “${conflict.suggested_name}”?`
					};
					conflictAction = async () => {
						try {
							await api.createFolder(conflict.suggested_name!, folderId);
							await refreshStorage();
							toast.success(
								tx(
									`Folder “${conflict.suggested_name}” dibuat`,
									`Folder “${conflict.suggested_name}” created`
								)
							);
						} catch (retryError) {
							toast.error(
								retryError instanceof Error
									? retryError.message
									: tx('Gagal membuat folder', 'Failed to create folder')
							);
						}
					};
					conflictConfirmOpen = true;
				} else if (conflict.status !== 409) {
					toast.error(conflict.message);
				}
			}
			return;
		}
		if (findSiblingByName(name)) {
			toast.error(tx(`Folder “${name}” sudah ada.`, `Folder “${name}” already exists.`));
			return;
		}
		tree = [
			...tree,
			{
				id: nextId('folder'),
				name,
				kind: 'folder',
				sizeBytes: 0,
				modifiedAt: 'Baru saja',
				parentId: folderId,
				storageNode: 'ssd'
			}
		];
		toast.success(tx(`Folder “${name}” dibuat`, `Folder “${name}” created`));
	}

	async function runMutation(update: { name?: string; parent_id?: string | null }) {
		if (!actionEntry || (actionEntry.kind !== 'folder' && actionEntry.storageAvailable === false))
			return;
		try {
			await api.mutate(actionEntry.id, update);
		} catch (error) {
			const conflict = error as Error & { status?: number; suggested_name?: string };
			if (conflict.status === 409 && conflict.suggested_name) {
				conflictTitle = { id: 'Nama Sudah Digunakan', en: 'Name Already in Use' };
				conflictDescription = {
					id: `Nama sudah digunakan. Gunakan nama yang disarankan “${conflict.suggested_name}”?`,
					en: `This name is already in use. Use the suggested name “${conflict.suggested_name}”?`
				};
				conflictAction = async () => {
					try {
						await api.mutate(actionEntry!.id, { ...update, name: conflict.suggested_name });
						await refreshStorage();
						toast.success(
							update.name !== undefined
								? tx('Nama diperbarui', 'Name updated')
								: tx(`“${actionEntry!.name}” dipindahkan`, `“${actionEntry!.name}” moved`)
						);
					} catch (retryError) {
						toast.error(
							retryError instanceof Error
								? retryError.message
								: tx('Gagal memperbarui item', 'Failed to update item')
						);
					}
				};
				conflictConfirmOpen = true;
				return;
			} else {
				if (conflict.status !== 409) toast.error(conflict.message);
				return;
			}
		}
		await refreshStorage();
		toast.success(
			update.name !== undefined
				? tx('Nama diperbarui', 'Name updated')
				: tx(`“${actionEntry.name}” dipindahkan`, `“${actionEntry.name}” moved`)
		);
	}

	function onRename(name: string) {
		if (actionEntry?.kind !== 'folder' && actionEntry?.storageAvailable === false) return;
		if (isApiBacked) {
			void runMutation({ name });
			return;
		}
		if (!actionEntry) return;
		tree = tree.map((e) =>
			e.id === actionEntry!.id ? { ...e, name, modifiedAt: 'Baru saja' } : e
		);
		toast.success(tx('Nama diperbarui', 'Name updated'));
	}

	function onMove(targetFolderId: string | null) {
		if (moveMultipleEntries.length > 0) {
			if (moveMultipleEntries.some((e) => e.kind !== 'folder' && e.storageAvailable === false))
				return;
			const entriesToMove = [...moveMultipleEntries];
			moveMultipleEntries = [];
			if (isApiBacked) {
				void (async () => {
					let movedCount = 0;
					for (const entryToMove of entriesToMove) {
						if (entryToMove.kind === 'folder' && targetFolderId === entryToMove.id) continue;
						if (
							entryToMove.kind === 'folder' &&
							targetFolderId &&
							isAncestor(allFolders, entryToMove.id, targetFolderId)
						)
							continue;
						try {
							await api.moveItem(entryToMove.id, targetFolderId);
							movedCount++;
						} catch {
							// skip conflict/error for batch
						}
					}
					selectedIds = [];
					await refreshStorage();
					const targetName = targetFolderId
						? (allFolders.find((f) => f.id === targetFolderId)?.name ?? tx('Folder', 'Folder'))
						: rootLabel;
					toast.success(
						movedCount === 1
							? tx(
									`“${entriesToMove[0].name}” dipindahkan ke “${targetName}”`,
									`“${entriesToMove[0].name}” moved to “${targetName}”`
								)
							: tx(
									`${movedCount} item dipindahkan ke “${targetName}”`,
									`${movedCount} items moved to “${targetName}”`
								)
					);
				})();
				return;
			}
			const movedSet = new Set(entriesToMove.map((e) => e.id));
			tree = tree.map((e) =>
				movedSet.has(e.id) ? { ...e, parentId: targetFolderId, modifiedAt: 'Baru saja' } : e
			);
			selectedIds = [];
			toast.success(
				tx(`${entriesToMove.length} item dipindahkan`, `${entriesToMove.length} items moved`)
			);
			return;
		}
		if (!actionEntry || (actionEntry.kind !== 'folder' && actionEntry.storageAvailable === false))
			return;
		if (actionEntry.kind === 'folder' && targetFolderId === actionEntry.id) {
			toast.error(
				tx(
					'Tidak dapat memindahkan folder ke dirinya sendiri.',
					'A folder cannot be moved into itself.'
				)
			);
			return;
		}
		if (isApiBacked) {
			const entryToMove = actionEntry;
			void (async () => {
				try {
					await api.moveItem(entryToMove.id, targetFolderId);
					await refreshStorage();
					toast.success(tx(`“${entryToMove.name}” dipindahkan`, `“${entryToMove.name}” moved`));
				} catch (error) {
					const conflict = error as Error & { status?: number; suggested_name?: string };
					if (conflict.status === 409 && conflict.suggested_name) {
						conflictTitle = { id: 'Nama Sudah Digunakan', en: 'Name Already in Use' };
						conflictDescription = {
							id: `Nama sudah digunakan. Gunakan nama yang disarankan “${conflict.suggested_name}”?`,
							en: `This name is already in use. Use the suggested name “${conflict.suggested_name}”?`
						};
						conflictAction = async () => {
							try {
								await api.mutate(entryToMove.id, {
									parent_id: targetFolderId,
									name: conflict.suggested_name
								});
								await refreshStorage();
								toast.success(
									tx(`“${entryToMove.name}” dipindahkan`, `“${entryToMove.name}” moved`)
								);
							} catch (retryError) {
								toast.error(
									retryError instanceof Error
										? retryError.message
										: tx('Gagal memindahkan', 'Failed to move')
								);
							}
						};
						conflictConfirmOpen = true;
					} else if (conflict.status === 409) {
						moveConflictEntry = entryToMove;
						moveConflictOpen = true;
					} else {
						toast.error(
							error instanceof Error
								? error.message
								: tx('Gagal memindahkan berkas', 'Failed to move file')
						);
					}
				}
			})();
			return;
		}
		tree = tree.map((e) =>
			e.id === actionEntry!.id ? { ...e, parentId: targetFolderId, modifiedAt: 'Baru saja' } : e
		);
		toast.success(tx(`“${actionEntry.name}” dipindahkan`, `“${actionEntry.name}” moved`));
	}

	async function onCopy(entry: FileEntry) {
		if (entry.kind !== 'folder' && entry.storageAvailable === false) return;
		if (isApiBacked) {
			try {
				await api.duplicate(entry.id);
				await refreshStorage();
				if (variant === 'my-storage') void chrome?.refreshQuota();
				toast.success(tx(`“${entry.name}” diduplikasi`, `“${entry.name}” duplicated`));
			} catch (error) {
				toast.error(
					error instanceof Error ? error.message : tx('Gagal menduplikasi', 'Failed to duplicate')
				);
			}
			return;
		}
		const base = entry.name.replace(/(\.[^.]+)?$/, (m) => ` (salinan)${m}`);
		let name = base;
		let n = 2;
		while (findSiblingByName(name)) {
			name = entry.name.replace(/(\.[^.]+)?$/, (m) => ` (salinan ${n})${m}`);
			n += 1;
		}
		tree = [
			...tree,
			{
				...entry,
				id: nextId('copy'),
				name,
				sizeBytes: 0,
				modifiedAt: 'Baru saja',
				parentId: folderId,
				starred: false
			}
		];
		toast.success(tx(`Salinan zero-byte “${name}” dibuat`, `Zero-byte copy “${name}” created`));
	}
	async function onStar(entry: FileEntry) {
		if (entry.kind !== 'folder' && entry.storageAvailable === false) return;
		const nextStarred = !entry.starred;
		if (isApiBacked) {
			currentItems = currentItems.map((item) =>
				item.id === entry.id ? { ...item, starred: nextStarred } : item
			);
			try {
				const result = await api.toggleStar(entry.id);
				currentItems = currentItems.map((item) =>
					item.id === entry.id ? { ...item, starred: result.is_starred } : item
				);
				toast.success(
					result.is_starred
						? tx('Ditandai berbintang', 'Added to starred')
						: tx('Bintang dihapus', 'Removed from starred')
				);
			} catch (error) {
				currentItems = currentItems.map((item) =>
					item.id === entry.id ? { ...item, starred: !nextStarred } : item
				);
				toast.error(
					error instanceof Error
						? error.message
						: tx('Gagal mengubah bintang', 'Failed to update star')
				);
			}
		} else {
			tree = tree.map((item) => (item.id === entry.id ? { ...item, starred: nextStarred } : item));
			toast.success(
				nextStarred
					? tx('Ditandai berbintang', 'Added to starred')
					: tx('Bintang dihapus', 'Removed from starred')
			);
		}
	}

	function onDelete(entry: FileEntry) {
		if (entry.kind !== 'folder' && entry.storageAvailable === false) return;
		deleteTargetEntry = entry;
		deleteConfirmOpen = true;
	}

	async function executeDelete() {
		if (bulkDeleteEntries.length > 0) {
			const toDelete = [...bulkDeleteEntries];
			if (toDelete.some((e) => e.kind !== 'folder' && e.storageAvailable === false)) return;
			bulkDeleteEntries = [];
			deleteTargetEntry = null;
			if (isApiBacked) {
				let deletedCount = 0;
				for (const entry of toDelete) {
					try {
						await api.remove(entry.id);
						deletedCount++;
					} catch {
						// continue
					}
				}
				selectedIds = [];
				await refreshStorage();
				if (variant === 'my-storage') void chrome?.refreshQuota();
				toast.success(
					variant === 'organization'
						? tx(`${deletedCount} item dihapus`, `${deletedCount} items deleted`)
						: tx(
								`${deletedCount} item dipindahkan ke Trash`,
								`${deletedCount} items moved to Trash`
							)
				);
				return;
			}
			const removeIds = new Set<string>(toDelete.map((e) => e.id));
			tree = tree.filter((e) => !removeIds.has(e.id));
			toast.success(
				tx(
					`${toDelete.length} item dipindahkan ke Trash (mock)`,
					`${toDelete.length} items moved to Trash (mock)`
				)
			);
			return;
		}
		if (!deleteTargetEntry) return;
		const entry = deleteTargetEntry;
		if (entry.kind !== 'folder' && entry.storageAvailable === false) return;
		if (isApiBacked) {
			try {
				await api.remove(entry.id);
				const deletedBreadcrumb = apiBreadcrumbs.findIndex((item) => item.id === entry.id);
				if (
					deletedBreadcrumb >= 0 &&
					folderId &&
					apiBreadcrumbs.slice(deletedBreadcrumb).some((item) => item.id === folderId)
				) {
					navigateFolder(deletedBreadcrumb > 0 ? apiBreadcrumbs[deletedBreadcrumb - 1].id : null);
				}
				await refreshStorage();
				if (variant === 'my-storage') void chrome?.refreshQuota();
				toast.success(
					variant === 'organization'
						? tx(`“${entry.name}” dihapus`, `“${entry.name}” deleted`)
						: tx(`“${entry.name}” dipindahkan ke Trash`, `“${entry.name}” moved to Trash`)
				);
			} catch (error) {
				toast.error(
					error instanceof Error
						? error.message
						: tx('Gagal menghapus item', 'Failed to delete item')
				);
			}
			return;
		}
		const removeIds = new Set<string>([entry.id]);
		if (entry.kind === 'folder') {
			let changed = true;
			while (changed) {
				changed = false;
				for (const e of tree) {
					if (e.parentId && removeIds.has(e.parentId) && !removeIds.has(e.id)) {
						removeIds.add(e.id);
						changed = true;
					}
				}
			}
		}
		tree = tree.filter((e) => !removeIds.has(e.id));
		selectedIds = selectedIds.filter((id) => !removeIds.has(id));
		if (folderId && removeIds.has(folderId)) navigateFolder(entry.parentId);
		toast.success(
			tx(`“${entry.name}” dipindahkan ke Trash (mock)`, `“${entry.name}” moved to Trash (mock)`)
		);
	}

	async function onDownload(entry: FileEntry) {
		if (entry.kind !== 'folder' && entry.storageAvailable === false) return;
		if (isApiBacked) {
			try {
				await api.download(entry.id, entry.kind === 'folder' ? `${entry.name}.zip` : entry.name);
			} catch (error) {
				toast.error(
					error instanceof Error ? error.message : tx('Gagal mengunduh', 'Failed to download')
				);
			}
			return;
		}
		toast.info(tx(`Mengunduh “${entry.name}” (mock)`, `Downloading “${entry.name}” (mock)`), {
			description: formatSize(entry.sizeBytes)
		});
	}

	let draggedEntryId = $state<string | null>(null);
	let dragOverFolderId = $state<string | null>(null);
	let longPressTimer: number | null = null;
	let isDraggingViaTouch = $state(false);
	let lastClickTime = 0;
	let dragStartAllowed = true;

	function handleItemClick(entry: FileEntry) {
		const now = Date.now();
		if (now - lastClickTime < 300) {
			// Double-click detected
			dragStartAllowed = false;
			openEntry(entry);
			setTimeout(() => {
				dragStartAllowed = true;
			}, 100);
		}
		lastClickTime = now;
	}

	function handleItemDragStart(entry: FileEntry, e: DragEvent) {
		if (!dragStartAllowed || (entry.kind !== 'folder' && entry.storageAvailable === false)) {
			e.preventDefault();
			return;
		}
		if (!e.dataTransfer) return;
		draggedEntryId = entry.id;
		e.dataTransfer.effectAllowed = 'move';
		e.dataTransfer.setData('text/plain', entry.id);
	}

	function handleItemDragEnd() {
		draggedEntryId = null;
		dragOverFolderId = null;
		isDraggingViaTouch = false;
	}

	function handleTouchStart(entry: FileEntry, e: TouchEvent) {
		if (entry.kind !== 'folder' && entry.storageAvailable === false) return;
		longPressTimer = window.setTimeout(() => {
			isDraggingViaTouch = true;
			draggedEntryId = entry.id;
		}, 200);
	}

	async function handleTouchEnd(e: TouchEvent) {
		if (longPressTimer) {
			clearTimeout(longPressTimer);
			longPressTimer = null;
		}

		// Mobile drop: complete move if dragging
		if (isDraggingViaTouch && draggedEntryId) {
			const touch = e.changedTouches[0];
			const dropTarget = document.elementFromPoint(touch.clientX, touch.clientY);
			const folderWrapper = dropTarget?.closest('[data-folder-id]');

			if (folderWrapper) {
				const targetFolderId = folderWrapper.getAttribute('data-folder-id');
				const targetFolder =
					(variant === 'my-storage' ? currentItems.find((e) => e.id === targetFolderId) : null) ??
					findEntry(tree, targetFolderId || '');

				if (targetFolder && targetFolder.kind === 'folder') {
					const isMulti = selectedIds.includes(draggedEntryId) && selectedIds.length > 1;
					const entriesToMove = isMulti
						? visible.filter((ent) => selectedIds.includes(ent.id) && ent.id !== targetFolder.id)
						: ([
								(isApiBacked ? currentItems.find((ent) => ent.id === draggedEntryId) : null) ??
									findEntry(tree, draggedEntryId)
							].filter(Boolean) as FileEntry[]);

					if (
						entriesToMove.some((item) => item.kind !== 'folder' && item.storageAvailable === false)
					) {
						isDraggingViaTouch = false;
						draggedEntryId = null;
						return;
					}

					if (
						entriesToMove.length > 0 &&
						!(entriesToMove.length === 1 && entriesToMove[0].id === targetFolder.id)
					) {
						if (isMulti) {
							if (isApiBacked) {
								void (async () => {
									let movedCount = 0;
									for (const movedEntry of entriesToMove) {
										if (
											movedEntry.kind === 'folder' &&
											isAncestor(isApiBacked ? allFolders : tree, movedEntry.id, targetFolder.id)
										)
											continue;
										try {
											await api.moveItem(movedEntry.id, targetFolder.id);
											movedCount++;
										} catch {
											// skip
										}
									}
									selectedIds = [];
									await refreshStorage();
									toast.success(
										movedCount === 1
											? tx(
													`“${entriesToMove[0].name}” dipindahkan ke “${targetFolder.name}”`,
													`“${entriesToMove[0].name}” moved to “${targetFolder.name}”`
												)
											: tx(
													`${movedCount} item dipindahkan ke “${targetFolder.name}”`,
													`${movedCount} items moved to “${targetFolder.name}”`
												)
									);
								})();
							} else {
								const movedIds = new Set(entriesToMove.map((ent) => ent.id));
								tree = tree.map((ent) =>
									movedIds.has(ent.id)
										? { ...ent, parentId: targetFolder.id, modifiedAt: 'Baru saja' }
										: ent
								);
								selectedIds = [];
								toast.success(
									tx(
										`${entriesToMove.length} item dipindahkan ke “${targetFolder.name}”`,
										`${entriesToMove.length} items moved to “${targetFolder.name}”`
									)
								);
							}
							isDraggingViaTouch = false;
							draggedEntryId = null;
							return;
						}
						const movedEntry = entriesToMove[0];
						// Circular move check
						if (movedEntry.kind === 'folder' && isAncestor(tree, movedEntry.id, targetFolder.id)) {
							toast.error(
								tx(
									'Tidak bisa memindahkan folder ke dalam subfolder-nya sendiri',
									'A folder cannot be moved into its own subfolder'
								)
							);
						} else if (isApiBacked) {
							try {
								await api.moveItem(movedEntry.id, targetFolder.id);
								await refreshStorage();
								toast.success(
									tx(
										`“${movedEntry.name}” dipindahkan ke “${targetFolder.name}”`,
										`“${movedEntry.name}” moved to “${targetFolder.name}”`
									)
								);
							} catch (error) {
								const conflict = error as Error & { status?: number; suggested_name?: string };
								if (conflict.status === 409 && conflict.suggested_name) {
									conflictTitle = { id: 'Nama Sudah Digunakan', en: 'Name Already in Use' };
									conflictDescription = {
										id: `Nama sudah digunakan. Gunakan nama yang disarankan “${conflict.suggested_name}”?`,
										en: `This name is already in use. Use the suggested name “${conflict.suggested_name}”?`
									};
									conflictAction = async () => {
										try {
											await api.mutate(movedEntry.id, {
												parent_id: targetFolder.id,
												name: conflict.suggested_name
											});
											await refreshStorage();
											toast.success(
												tx(
													`“${movedEntry.name}” dipindahkan ke “${targetFolder.name}”`,
													`“${movedEntry.name}” moved to “${targetFolder.name}”`
												)
											);
										} catch (retryError) {
											toast.error(
												retryError instanceof Error
													? retryError.message
													: tx('Gagal memindahkan', 'Failed to move')
											);
										}
									};
									conflictConfirmOpen = true;
								} else if (conflict.status === 409) {
									moveConflictEntry = movedEntry;
									moveConflictTargetFolder = targetFolder;
									moveConflictOpen = true;
								} else {
									toast.error(
										error instanceof Error
											? error.message
											: tx('Gagal memindahkan berkas', 'Failed to move file')
									);
								}
							}
						} else {
							// Complete move
							tree = tree.map((ent) =>
								ent.id === draggedEntryId
									? { ...ent, parentId: targetFolder.id, modifiedAt: 'Baru saja' }
									: ent
							);
							toast.success(
								tx(
									`“${movedEntry.name}” dipindahkan ke “${targetFolder.name}”`,
									`“${movedEntry.name}” moved to “${targetFolder.name}”`
								)
							);
						}
					}
				}
			}
		}

		isDraggingViaTouch = false;
		draggedEntryId = null;
	}

	function handleFolderDragOver(entry: FileEntry, e: DragEvent) {
		e.preventDefault();
		e.stopPropagation();
		if (e.dataTransfer) {
			e.dataTransfer.dropEffect = draggedEntryId ? 'move' : 'copy';
		}
		dragOverFolderId = entry.id;
	}

	function handleFolderDragLeave(entry: FileEntry, e: DragEvent) {
		const related = e.relatedTarget as HTMLElement | null;
		if (!related || !related.closest(`[data-folder-id="${entry.id}"]`)) {
			if (dragOverFolderId === entry.id) {
				dragOverFolderId = null;
			}
		}
	}

	async function handleFolderDrop(targetFolder: FileEntry, e: DragEvent) {
		e.preventDefault();
		e.stopPropagation();
		dragOverFolderId = null;
		dragActive = false;
		dragDepth = 0;

		// 1. Internal item move
		if (draggedEntryId) {
			const isMulti = selectedIds.includes(draggedEntryId) && selectedIds.length > 1;
			const entriesToMove = isMulti
				? visible.filter((ent) => selectedIds.includes(ent.id) && ent.id !== targetFolder.id)
				: ([
						(isApiBacked ? currentItems.find((ent) => ent.id === draggedEntryId) : null) ??
							findEntry(tree, draggedEntryId)
					].filter(Boolean) as FileEntry[]);

			if (entriesToMove.some((item) => item.kind !== 'folder' && item.storageAvailable === false)) {
				draggedEntryId = null;
				return;
			}

			if (
				entriesToMove.length === 0 ||
				(entriesToMove.length === 1 && entriesToMove[0].id === targetFolder.id)
			) {
				draggedEntryId = null;
				return;
			}

			if (isMulti) {
				if (isApiBacked) {
					void (async () => {
						let movedCount = 0;
						for (const movedEntry of entriesToMove) {
							if (
								movedEntry.kind === 'folder' &&
								isAncestor(isApiBacked ? allFolders : tree, movedEntry.id, targetFolder.id)
							) {
								toast.error(
									tx(
										`Tidak bisa memindahkan folder “${movedEntry.name}” ke dalam subfolder-nya sendiri`,
										`Cannot move folder “${movedEntry.name}” into its own subfolder`
									)
								);
								continue;
							}
							try {
								await api.moveItem(movedEntry.id, targetFolder.id);
								movedCount++;
							} catch {
								// skip
							}
						}
						selectedIds = [];
						await refreshStorage();
						toast.success(
							movedCount === 1
								? tx(
										`“${entriesToMove[0].name}” dipindahkan ke “${targetFolder.name}”`,
										`“${entriesToMove[0].name}” moved to “${targetFolder.name}”`
									)
								: tx(
										`${movedCount} item dipindahkan ke “${targetFolder.name}”`,
										`${movedCount} items moved to “${targetFolder.name}”`
									)
						);
					})();
					draggedEntryId = null;
					return;
				}
				const movedIds = new Set(entriesToMove.map((ent) => ent.id));
				tree = tree.map((ent) =>
					movedIds.has(ent.id)
						? { ...ent, parentId: targetFolder.id, modifiedAt: 'Baru saja' }
						: ent
				);
				selectedIds = [];
				toast.success(
					tx(
						`${entriesToMove.length} item dipindahkan ke “${targetFolder.name}”`,
						`${entriesToMove.length} items moved to “${targetFolder.name}”`
					)
				);
				draggedEntryId = null;
				return;
			}

			const movedEntry = entriesToMove[0];
			// Prevent circular move: folder cannot be moved into its own descendant
			if (movedEntry.kind === 'folder' && isAncestor(tree, movedEntry.id, targetFolder.id)) {
				toast.error(
					tx(
						'Tidak bisa memindahkan folder ke dalam subfolder-nya sendiri',
						'A folder cannot be moved into its own subfolder'
					)
				);
				draggedEntryId = null;
				return;
			}

			if (isApiBacked) {
				try {
					await api.moveItem(movedEntry.id, targetFolder.id);
					await refreshStorage();
					toast.success(
						tx(
							`“${movedEntry.name}” dipindahkan ke “${targetFolder.name}”`,
							`“${movedEntry.name}” moved to “${targetFolder.name}”`
						)
					);
				} catch (error) {
					const conflict = error as Error & { status?: number; suggested_name?: string };
					if (conflict.status === 409 && conflict.suggested_name) {
						conflictTitle = { id: 'Nama Sudah Digunakan', en: 'Name Already in Use' };
						conflictDescription = {
							id: `Nama sudah digunakan. Gunakan nama yang disarankan “${conflict.suggested_name}”?`,
							en: `This name is already in use. Use the suggested name “${conflict.suggested_name}”?`
						};
						conflictAction = async () => {
							try {
								await api.mutate(movedEntry.id, {
									parent_id: targetFolder.id,
									name: conflict.suggested_name
								});
								await refreshStorage();
								toast.success(
									tx(
										`“${movedEntry.name}” dipindahkan ke “${targetFolder.name}”`,
										`“${movedEntry.name}” moved to “${targetFolder.name}”`
									)
								);
							} catch (retryError) {
								toast.error(
									retryError instanceof Error
										? retryError.message
										: tx('Gagal memindahkan', 'Failed to move')
								);
							}
						};
						conflictConfirmOpen = true;
					} else if (conflict.status === 409) {
						moveConflictEntry = movedEntry;
						moveConflictTargetFolder = targetFolder;
						moveConflictOpen = true;
					} else {
						toast.error(
							error instanceof Error
								? error.message
								: tx('Gagal memindahkan berkas', 'Failed to move file')
						);
					}
				}
			} else {
				tree = tree.map((ent) =>
					ent.id === draggedEntryId
						? { ...ent, parentId: targetFolder.id, modifiedAt: 'Baru saja' }
						: ent
				);
				toast.success(
					tx(
						`“${movedEntry.name}” dipindahkan ke “${targetFolder.name}”`,
						`“${movedEntry.name}” moved to “${targetFolder.name}”`
					)
				);
			}
			draggedEntryId = null;
			return;
		}

		// 2. External files or folders dropped directly into this folder row
		if (e.dataTransfer) {
			const result = await extractDroppedEntries(e.dataTransfer);
			if (result.files.length > 0 || result.emptyFolders.length > 0) {
				void handleUploadFiles(result.files, result.paths, targetFolder.id, result.emptyFolders);
			}
		}
	}
</script>

<svelte:window
	onpointermove={(event) => {
		if (queueStartX !== null) {
			const diff = event.clientX - queueStartX;
			queueSwipeOffset = Math.max(0, diff);
			if (queueCanDismiss && queueSwipeOffset > 80) {
				queueDismissed = true;
				queueStartX = null;
				queueSwipeOffset = 0;
				isSwipingQueue = false;
			}
		}
	}}
	onpointerup={() => {
		if (queueCanDismiss && queueStartX !== null && queueSwipeOffset > 60) {
			queueDismissed = true;
		}
		queueStartX = null;
		queueSwipeOffset = 0;
		isSwipingQueue = false;
	}}
	onpointercancel={() => {
		queueStartX = null;
		queueSwipeOffset = 0;
		isSwipingQueue = false;
	}}
/>

<input
	bind:this={fileInput}
	type="file"
	class="hidden"
	aria-label={tx('Pilih berkas untuk diunggah', 'Choose files to upload')}
	data-testid="upload-file-input"
	multiple
	onchange={(e) => {
		const input = e.currentTarget as HTMLInputElement;
		if (input.files) void handleUploadFiles(input.files);
		input.value = '';
	}}
/>
<input
	bind:this={folderInput}
	type="file"
	class="hidden"
	aria-label={tx('Pilih folder untuk diunggah', 'Choose folders to upload')}
	data-testid="upload-folder-input"
	multiple
	webkitdirectory
	onchange={(e) => {
		const input = e.currentTarget as HTMLInputElement;
		if (input.files) {
			const files = Array.from(input.files);
			void handleUploadFiles(
				files,
				files.map(
					(file) => (file as File & { webkitRelativePath?: string }).webkitRelativePath ?? file.name
				)
			);
		}
		input.value = '';
	}}
/>
<input
	bind:this={queueFileInput}
	type="file"
	class="hidden"
	aria-label={tx(
		'Pilih kembali file untuk melanjutkan unggahan',
		'Choose the file again to resume the upload'
	)}
	data-testid="resume-upload-file-input"
	onchange={handleQueuedFileSelection}
/>

<div class="flex flex-col gap-4">
	<FileExplorerBreadcrumbs segments={breadcrumbs} onNavigate={navigateFolder} />
	{#if storageLoading}
		<p class="text-sm text-muted" role="status">
			{tx(`Memuat ${rootLabel}…`, `Loading ${rootLabel}…`)}
		</p>
	{/if}
	<FileExplorerToolbar
		bind:category
		onNewFolder={() => (newFolderOpen = true)}
		onUpload={triggerUpload}
		onUploadFolder={triggerFolderUpload}
		showFolderUpload={isApiBacked}
	/>

	{#if selectedCount > 0}
		<div
			class="flex items-center justify-between rounded-xl border border-subtle bg-surface px-4 py-2.5 shadow-none transition-all"
			data-testid="bulk-action-bar"
		>
			<div class="flex items-center gap-3">
				<label class="flex cursor-pointer items-center gap-2 select-none">
					<Checkbox
						checked={isAllSelected}
						indeterminate={isPartiallySelected}
						onCheckedChange={toggleSelectAll}
						aria-label={tx('Pilih semua berkas', 'Select all files')}
						data-testid="select-all-checkbox"
					/>
					<span class="text-sm font-medium text-main" data-testid="selected-count-label">
						{tx(`${selectedCount} item dipilih`, `${selectedCount} items selected`)}
					</span>
				</label>
			</div>

			<DropdownMenu.Root>
				<DropdownMenu.Trigger>
					{#snippet child({ props })}
						<Button
							{...props}
							type="button"
							variant="ghost"
							size="icon"
							class="rounded-full bg-surface text-muted hover:bg-primary hover:text-white aria-expanded:bg-primary aria-expanded:text-white"
							aria-label={tx('Aksi massal', 'Bulk actions')}
							data-testid="bulk-actions-menu-trigger"
						>
							<MoreVertical class="size-4 stroke-2" />
						</Button>
					{/snippet}
				</DropdownMenu.Trigger>
				<DropdownMenu.Content align="end" class="min-w-48">
					<DropdownMenu.Item
						disabled={hasUnavailableSelection}
						class="cursor-pointer gap-2"
						onclick={handleBulkDownload}
						data-testid="bulk-action-download"
					>
						<Download class="size-4 stroke-2" />
						<span>{tx('Unduh semua', 'Download selected')}</span>
					</DropdownMenu.Item>
					<DropdownMenu.Item
						disabled={hasUnavailableSelection}
						class="cursor-pointer gap-2"
						onclick={handleBulkMove}
						data-testid="bulk-action-move"
					>
						<FolderInput class="size-4 stroke-2" />
						<span>{tx('Pindahkan', 'Move')}</span>
					</DropdownMenu.Item>
					<DropdownMenu.Separator />
					<DropdownMenu.Item
						disabled={hasUnavailableSelection}
						class="cursor-pointer gap-2 text-destructive focus:text-destructive"
						onclick={handleBulkDelete}
						data-testid="bulk-action-delete"
					>
						<Trash2 class="size-4 stroke-2 text-destructive" />
						<span>{tx('Hapus', 'Delete')}</span>
					</DropdownMenu.Item>
				</DropdownMenu.Content>
			</DropdownMenu.Root>
		</div>
	{/if}
	<FileExplorerDropzone active={dragActive}>
		{#if visible.length === 0}
			<div
				class="overflow-hidden rounded-xl border border-subtle bg-surface px-4 py-12 text-center"
			>
				<p class="text-sm font-medium text-main">{tx('Folder kosong', 'Folder is empty')}</p>
				<p class="mt-1 text-xs text-muted">
					{tx(
						'Seret berkas ke sini atau gunakan tombol Unggah.',
						'Drag files here or use the Upload button.'
					)}
				</p>
			</div>
		{:else if driveView.mode === 'grid'}
			<DriveFileGrid>
				{#each visible as entry (entry.id)}
					{@const menuHandlers = {
						onOpen: () => openEntry(entry),
						onDownload,
						onRename: (e: FileEntry) => {
							actionEntry = e;
							renameOpen = true;
						},
						onMove: (e: FileEntry) => {
							actionEntry = e;
							moveOpen = true;
						},
						onCopy,
						onStar,
						onShare: (e: FileEntry) => {
							actionEntry = e;
							shareOpen = true;
						},
						onDelete
					}}
					<div
						data-folder-id={entry.kind === 'folder' ? entry.id : undefined}
						draggable="true"
						role="presentation"
						onkeydown={(e) => {
							if (e.key === ' ' && (e.target as HTMLElement).getAttribute('role') === 'option') {
								e.preventDefault();
								handleItemClick(entry);
							}
						}}
						onclick={() => handleItemClick(entry)}
						ondragstart={(e) => handleItemDragStart(entry, e)}
						ondragend={handleItemDragEnd}
						ondblclick={() => openEntry(entry)}
						ontouchstart={(e) => handleTouchStart(entry, e)}
						ontouchend={handleTouchEnd}
						ontouchmove={(e) => {
							if (isDraggingViaTouch) {
								e.preventDefault();
							} else if (longPressTimer) {
								clearTimeout(longPressTimer);
								longPressTimer = null;
							}
						}}
						ondragover={entry.kind === 'folder' ? (e) => handleFolderDragOver(entry, e) : undefined}
						ondragleave={entry.kind === 'folder'
							? (e) => handleFolderDragLeave(entry, e)
							: undefined}
						ondrop={entry.kind === 'folder' ? (e) => handleFolderDrop(entry, e) : undefined}
						class={`${entry.id === draggedEntryId ? 'cursor-grabbing opacity-50' : ''} ${
							entry.kind === 'folder' && dragOverFolderId === entry.id
								? 'rounded-xl border-primary bg-accent/40 ring-2 ring-primary/50 transition-all'
								: ''
						}`}
					>
						<FileExplorerContextMenu {entry} {variant} {...menuHandlers}>
							<FileGridCard
								kind={entry.kind}
								name={entry.name}
								subtitle={entry.kind === 'folder'
									? `${tx('Folder', 'Folder')} · ${formatModifiedAt(entry.modifiedAt)}`
									: `${formatSize(entry.sizeBytes)} · ${formatModifiedAt(entry.modifiedAt)}`}
								previewUrl={entry.kind !== 'folder' && entry.storageAvailable !== false
									? previewUrl(variant, entry.id)
									: undefined}
								selected={selectedIds.includes(entry.id)}
								disabled={entry.kind !== 'folder' && entry.storageAvailable === false}
								onOpen={() => openEntry(entry)}
							>
								{#snippet badges()}
									{#if entry.kind !== 'folder' && entry.storageAvailable === false}
										<OfflineFileNotice />
									{/if}
								{/snippet}
								{#snippet select()}
									<button
										type="button"
										disabled={entry.kind !== 'folder' && entry.storageAvailable === false}
										class={cn(
											'inline-flex size-7 items-center justify-center rounded-lg bg-surface/90 shadow-sm backdrop-blur-sm transition-opacity hover:bg-surface focus-visible:ring-1 focus-visible:ring-primary focus-visible:outline-none',
											selectedIds.includes(entry.id)
												? 'opacity-100'
												: 'opacity-0 group-hover:opacity-100 focus-within:opacity-100'
										)}
										title={selectedIds.includes(entry.id)
											? tx('Batal pilih', 'Deselect')
											: tx('Pilih', 'Select')}
										aria-label={tx(`Pilih ${entry.name}`, `Select ${entry.name}`)}
										onclick={(e) => {
											e.stopPropagation();
											toggleSelectEntry(entry.id);
										}}
									>
										<Checkbox
											checked={selectedIds.includes(entry.id)}
											disabled={entry.kind !== 'folder' && entry.storageAvailable === false}
											class="pointer-events-none"
											tabindex={-1}
										/>
									</button>
								{/snippet}
								{#snippet menu()}
									<FileExplorerActionsMenu {entry} {variant} {...menuHandlers} />
								{/snippet}
							</FileGridCard>
						</FileExplorerContextMenu>
					</div>
				{/each}
			</DriveFileGrid>
		{:else}
			<div
				class="overflow-hidden rounded-xl border border-subtle bg-surface"
				role="listbox"
				aria-label={tx('Daftar berkas', 'File list')}
			>
				{#each visible as entry (entry.id)}
					<div
						data-folder-id={entry.kind === 'folder' ? entry.id : undefined}
						draggable="true"
						role="presentation"
						onkeydown={(e) => {
							if (e.key === ' ' && (e.target as HTMLElement).getAttribute('role') === 'option') {
								e.preventDefault();
								handleItemClick(entry);
							}
						}}
						onclick={() => handleItemClick(entry)}
						ondragstart={(e) => handleItemDragStart(entry, e)}
						ondragend={handleItemDragEnd}
						ondblclick={() => openEntry(entry)}
						ontouchstart={(e) => handleTouchStart(entry, e)}
						ontouchend={handleTouchEnd}
						ontouchmove={(e) => {
							if (isDraggingViaTouch) {
								e.preventDefault();
							} else if (longPressTimer) {
								clearTimeout(longPressTimer);
								longPressTimer = null;
							}
						}}
						ondragover={entry.kind === 'folder' ? (e) => handleFolderDragOver(entry, e) : undefined}
						ondragleave={entry.kind === 'folder'
							? (e) => handleFolderDragLeave(entry, e)
							: undefined}
						ondrop={entry.kind === 'folder' ? (e) => handleFolderDrop(entry, e) : undefined}
						class={`${entry.id === draggedEntryId ? 'cursor-grabbing opacity-50' : ''} ${
							entry.kind === 'folder' && dragOverFolderId === entry.id
								? 'rounded-lg border-primary bg-accent/40 ring-2 ring-primary/50 transition-all'
								: ''
						}`}
					>
						<FileExplorerRow
							{entry}
							{variant}
							selected={selectedIds.includes(entry.id)}
							isDragOver={entry.kind === 'folder' && dragOverFolderId === entry.id}
							onOpen={openEntry}
							onSelect={() => {}}
							onToggleSelect={(e) => toggleSelectEntry(e.id)}
							{onDownload}
							onRename={(e) => {
								actionEntry = e;
								renameOpen = true;
							}}
							onMove={(e) => {
								actionEntry = e;
								moveOpen = true;
							}}
							{onCopy}
							{onStar}
							onShare={(e) => {
								actionEntry = e;
								shareOpen = true;
							}}
							{onDelete}
						/>
					</div>
				{/each}
			</div>
		{/if}
	</FileExplorerDropzone>
</div>

{#if isApiBacked && queueEntries.length > 0 && (!queueDismissed || queueHasActive)}
	<aside
		transition:fly={{ x: 350, duration: 250 }}
		class="fixed top-20 right-4 left-4 z-50 ml-auto w-auto max-w-sm rounded-xl border border-subtle bg-surface p-4 text-main shadow-none"
		aria-label={tx('Antrean unggahan', 'Upload queue')}
		data-testid="upload-queue"
		style={`touch-action: pan-y; ${
			isSwipingQueue
				? `transform: translateX(${queueSwipeOffset}px); opacity: ${Math.max(0.2, 1 - queueSwipeOffset / 200)}; transition: none;`
				: queueSwipeOffset > 0
					? 'transform: translateX(0px); opacity: 1; transition: transform 200ms ease-out, opacity 200ms ease-out;'
					: ''
		}`}
		onpointerdown={(event) => {
			queueStartX = event.clientX;
			queueSwipeOffset = 0;
			isSwipingQueue = true;
		}}
	>
		<div class="flex items-center justify-between gap-3">
			<button
				type="button"
				class="text-left text-sm font-semibold transition-opacity hover:opacity-80"
				aria-expanded={queueOpen}
				onclick={() => (queueOpen = !queueOpen)}
			>
				{queueHeading} · {queueCompletedCount}/{queueEntries.length}
			</button>
			<div class="flex shrink-0 items-center gap-1">
				<button
					type="button"
					class="inline-flex size-6 items-center justify-center rounded-md text-muted transition-colors hover:bg-muted/20 hover:text-main"
					aria-label={queueOpen
						? tx('Ciutkan antrean unggahan', 'Collapse upload queue')
						: tx('Buka antrean unggahan', 'Expand upload queue')}
					onclick={() => (queueOpen = !queueOpen)}
				>
					{#if queueOpen}
						<ChevronUp class="size-4" />
					{:else}
						<ChevronDown class="size-4" />
					{/if}
				</button>
				{#if queueCanDismiss}
					<button
						type="button"
						class="inline-flex size-6 items-center justify-center rounded-md text-muted transition-colors hover:bg-muted/20 hover:text-main"
						aria-label={tx('Tutup notifikasi unggahan', 'Dismiss upload notification')}
						data-testid="dismiss-upload-queue"
						onclick={dismissQueue}
					>
						<X class="size-4" />
					</button>
				{/if}
			</div>
		</div>
		{#if queueHasProgress}
			<div
				class="mt-2 h-1.5 overflow-hidden rounded-full bg-muted/30"
				role="progressbar"
				aria-label={tx('Progress unggahan', 'Upload progress')}
				aria-valuenow={uploadProgress}
				aria-valuemin="0"
				aria-valuemax="100"
			>
				<div class="h-full bg-primary transition-all" style={`width:${uploadProgress}%`}></div>
			</div>
		{/if}
		{#if queueOpen}
			<ul class="mt-3 max-h-72 space-y-3 overflow-y-auto text-xs">
				{#each queueEntries as item (item.id)}
					<li
						class="flex flex-col gap-1.5"
						data-testid={item.kind === 'upload' ? 'upload-queue-item' : 'download-queue-item'}
						data-state={item.state}
					>
						<div class="flex items-center justify-between gap-2">
							<span class="min-w-0 flex-1 truncate font-medium text-main" title={item.name}>
								{item.name}
							</span>
							<span class="shrink-0 text-muted">
								{queueStateLabel(
									item
								)}{#if !['completed', 'done', 'started', 'failed', 'expired'].includes(item.state)}
									· {item.progress}%{/if}
							</span>
						</div>
						{#if item.error}
							<span class="text-xs font-medium text-danger">{item.error}</span>
						{/if}
						{#if item.kind === 'upload'}
							<div class="flex flex-wrap justify-end gap-2">
								{#if transferCanPause(item)}
									<button
										type="button"
										class="text-primary hover:underline"
										aria-label={tx(`Jeda ${item.name}`, `Pause ${item.name}`)}
										data-testid={`pause-upload-${item.id}`}
										onclick={() => void pauseQueuedUpload(item.id)}>{tx('Jeda', 'Pause')}</button
									>
								{/if}
								{#if ['queued', 'paused', 'waiting-for-file', 'failed', 'expired'].includes(item.state) || (['uploading', 'retrying', 'verifying', 'committing'].includes(item.state) && !getInMemoryTransferFile(item.id))}
									<button
										type="button"
										class="text-primary hover:underline"
										aria-label={`${transferActionLabel(item)}: ${item.name}`}
										data-testid={`resume-upload-${item.id}`}
										onclick={() => void resumeQueuedUpload(item.id)}
										>{transferActionLabel(item)}</button
									>
								{/if}
								{#if item.state === 'failed' && transferHasSession(item)}
									<button
										type="button"
										class="text-primary hover:underline"
										aria-label={tx(
											`Mulai ulang dari nol: ${item.name}`,
											`Start over from scratch: ${item.name}`
										)}
										data-testid={`restart-upload-${item.id}`}
										onclick={() => void restartQueuedUpload(item.id)}
										>{tx('Mulai ulang dari nol', 'Start over from scratch')}</button
									>
								{/if}
								{#if transferCanCancel(item)}
									<button
										type="button"
										class="text-danger hover:underline"
										aria-label={tx(`Batalkan ${item.name}`, `Cancel ${item.name}`)}
										data-testid={`cancel-upload-${item.id}`}
										onclick={() => void cancelQueuedUpload(item.id)}
										>{tx('Batalkan', 'Cancel')}</button
									>
								{/if}
							</div>
						{/if}
					</li>
				{/each}
			</ul>
		{/if}
	</aside>
{/if}
<NewFolderDialog bind:open={newFolderOpen} onConfirm={onNewFolder} />
<RenameDialog bind:open={renameOpen} initialName={actionEntry?.name ?? ''} onConfirm={onRename} />
<MoveDialog
	bind:open={moveOpen}
	entry={moveMultipleEntries.length > 0 ? null : actionEntry}
	entries={moveMultipleEntries}
	folders={isApiBacked ? allFolders : tree}
	currentParentId={moveMultipleEntries[0]?.parentId ?? actionEntry?.parentId ?? null}
	onConfirm={onMove}
/>
<ConflictResolutionDialog
	bind:open={moveConflictOpen}
	title={tx('Nama Sudah Digunakan', 'Name Already in Use')}
	description={moveConflictEntry
		? tx(
				`“${moveConflictEntry.name}” sudah ada di folder tujuan. Ganti nama berkas atau batalkan pemindahan?`,
				`“${moveConflictEntry.name}” already exists in the destination folder. Rename the file or cancel the move?`
			)
		: ''}
	fileName={moveConflictEntry?.name ?? ''}
	canReplace={false}
	renameText={tx('Ganti nama', 'Rename')}
	cancelText={tx('Batal', 'Cancel')}
	onKeepBoth={() => {
		if (moveConflictEntry) {
			actionEntry = moveConflictEntry;
			renameOpen = true;
		}
	}}
	onCancel={() => {
		moveConflictEntry = null;
		moveConflictTargetFolder = null;
	}}
/>
{#if !isApiBacked}
	<ConflictResolutionDialog
		bind:open={conflictOpen}
		fileName={pendingUpload?.name ?? ''}
		onReplace={() => {
			if (!pendingUpload || !actionEntry) return;
			addFile(pendingUpload.name, pendingUpload.size, pendingUpload.kind, actionEntry.id);
			pendingUpload = null;
		}}
		onKeepBoth={() => {
			if (!pendingUpload) return;
			const ext = pendingUpload.name.includes('.')
				? pendingUpload.name.slice(pendingUpload.name.lastIndexOf('.'))
				: '';
			const base = pendingUpload.name.slice(0, pendingUpload.name.length - ext.length);
			let name = `${base} (1)${ext}`;
			let n = 2;
			while (findSiblingByName(name)) {
				name = `${base} (${n})${ext}`;
				n += 1;
			}
			addFile(name, pendingUpload.size, pendingUpload.kind);
			pendingUpload = null;
		}}
	/>
{/if}
<ShareSheet bind:open={shareOpen} entry={actionEntry} />
<FilePreviewDialog
	open={previewOpen}
	entry={previewEntry}
	source={variant}
	{onDownload}
	onClose={closePreview}
/>
<ConfirmDialog
	bind:open={deleteConfirmOpen}
	title={bulkDeleteEntries.length > 0
		? tx(
				`Pindahkan ${bulkDeleteEntries.length} item ke Trash?`,
				`Move ${bulkDeleteEntries.length} items to Trash?`
			)
		: deleteTargetEntry?.kind === 'folder'
			? tx('Pindahkan Folder ke Trash?', 'Move Folder to Trash?')
			: tx('Pindahkan Berkas ke Trash?', 'Move File to Trash?')}
	description={bulkDeleteEntries.length > 0
		? tx(
				`Pindahkan ${bulkDeleteEntries.length} item terpilih ke Trash?`,
				`Move ${bulkDeleteEntries.length} selected items to Trash?`
			)
		: deleteTargetEntry?.kind === 'folder'
			? tx(
					`Pindahkan “${deleteTargetEntry.name}” dan semua isinya ke Trash?`,
					`Move “${deleteTargetEntry.name}” and all its contents to Trash?`
				)
			: tx(
					`Pindahkan “${deleteTargetEntry?.name}” ke Trash?`,
					`Move “${deleteTargetEntry?.name}” to Trash?`
				)}
	confirmText={tx('Hapus ke Trash', 'Move to Trash')}
	variant="destructive"
	onConfirm={executeDelete}
/>
<ConfirmDialog
	bind:open={conflictConfirmOpen}
	title={conflictTitleText}
	description={conflictDescriptionText}
	confirmText={tx('Gunakan Nama Ini', 'Use This Name')}
	onConfirm={async () => {
		if (conflictAction) await conflictAction();
	}}
/>
