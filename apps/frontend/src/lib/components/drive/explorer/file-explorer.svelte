<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { onDestroy, onMount, untrack } from 'svelte';
	import { fly } from 'svelte/transition';
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
	import DriveFileGrid from '$lib/components/drive/drive-file-grid.svelte';
	import FileGridCard from '$lib/components/drive/file-grid-card.svelte';
	import StorageNodeBadge from './storage-node-badge.svelte';
	import { driveView } from '$lib/drive-view.svelte.js';
	import { ChevronDown, ChevronUp, X } from '@lucide/svelte';

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
	const showNodeBadge = $derived(chrome.user.role === 'super_admin');

	let tree = $state<FileEntry[]>(untrack(() => cloneTree(initialTree)));
	let currentItems = $state<FileEntry[]>([]);
	let apiBreadcrumbs = $state<{ id: string; name: string }[]>([]);
	let allFolders = $state<FileEntry[]>([]);
	let uploadQueue = $state<{ name: string; size: number; progress: number; state: 'queued' | 'uploading' | 'done' | 'failed'; error?: string }[]>([]);
	let queueOpen = $state(false);
	let queueDismissed = $state(false);
	const uploadProgress = $derived.by(() => {
		let totalWeight = 0;
		let transferredWeight = 0;
		for (const item of uploadQueue) {
			const weight = Math.max(item.size, 1);
			totalWeight += weight;
			transferredWeight += weight * item.progress;
		}
		if (!totalWeight) return 0;
		return uploadQueue.every((item) => item.state === 'done')
			? 100
			: Math.min(Math.floor(transferredWeight / totalWeight), 99);
	});
	let queueStartX: number | null = null;
	let queueSwipeOffset = $state(0);
	let isSwipingQueue = $state(false);
	let storageLoading = $state(false);
	let category = $state<FileCategory>('all');
	let deleteConfirmOpen = $state(false);
	let deleteTargetEntry = $state<FileEntry | null>(null);
	let conflictConfirmOpen = $state(false);
	let conflictTitle = $state('Folder Sudah Ada');
	let conflictDescription = $state('');
	let conflictAction = $state<(() => Promise<void>) | null>(null);
	let selectedId = $state<string | null>(null);
	let dragActive = $state(false);
	let dragDepth = 0;

	let newFolderOpen = $state(false);
	let renameOpen = $state(false);
	let moveOpen = $state(false);
	let conflictOpen = $state(false);
	let shareOpen = $state(false);

	let actionEntry = $state<FileEntry | null>(null);
	let pendingUpload = $state<{ name: string; size: number; kind: FileKind } | null>(null);
	let fileInput: HTMLInputElement | undefined = $state();
	let folderInput: HTMLInputElement | undefined = $state();

	const folderId = $derived(page.url.searchParams.get('folder'));
	const previewId = $derived(page.url.searchParams.get('preview'));

	const breadcrumbs = $derived(
		variant === 'my-storage'
			? [{ id: null, label: rootLabel }, ...apiBreadcrumbs.map((item) => ({ id: item.id, label: item.name }))]
			: getBreadcrumbPath(tree, folderId, rootLabel)
	);
	const children = $derived(variant === 'my-storage' ? currentItems : getChildren(tree, folderId));
	const visible = $derived(category === 'all' ? children : children.filter((e) => e.kind === category));
	const previewEntry = $derived(
		previewId ? (variant === 'my-storage'
			? currentItems.find((entry) => entry.id === previewId) ?? null
			: findEntry(tree, previewId) ?? null) : null
	);
	const previewOpen = $derived(!!previewEntry);
	const moveFolders = $derived(
		variant === 'my-storage'
			? allFolders.filter((f) => f.id !== actionEntry?.id)
			: getFolderOptions(tree).filter((f) => f.id !== actionEntry?.id)
	);

	async function refreshStorage() {
		storageLoading = true;
		try {
			const [listing, folders] = await Promise.all([myStorageAPI.list(folderId), myStorageAPI.folders()]);
			currentItems = listing.items.map((item) => myStorageAPI.toEntry(item));
			apiBreadcrumbs = listing.breadcrumbs;
			allFolders = folders.items.map((item) => ({
				id: item.id, name: item.name, kind: 'folder' as const, sizeBytes: 0,
				parentId: item.parent_id, modifiedAt: ''
			}));
		} catch (error) {
			toast.error(error instanceof Error ? error.message : 'Gagal memuat My Storage');
		} finally {
			storageLoading = false;
		}
	}

	$effect(() => {
		if (variant === 'my-storage') void refreshStorage();
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
		selectedId = null;
		syncQuery({ folder: id, preview: null });
	}

	function openEntry(entry: FileEntry) {
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
				e.id === replaceId
					? { ...e, name, sizeBytes: size, kind, modifiedAt: 'Baru saja' }
					: e
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
		toast.success(`“${name}” diunggah (mock)`);
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

		if (variant !== 'my-storage') {
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

		if (queueDismissed && uploadQueue.every((item) => item.state === 'done' || item.state === 'failed')) {
			uploadQueue = [];
			queueOpen = false;
		}

		const folderPaths = new Map<string, string>();
		if (effectiveParentId) folderPaths.set('', effectiveParentId);

		async function ensureFolderPath(relPath: string): Promise<string | null> {
			const parts = relPath.replace(/\\/g, '/').split('/').filter(Boolean);
			let parent = effectiveParentId;
			let accumulated = '';
			for (const part of parts) {
				accumulated = accumulated ? `${accumulated}/${part}` : part;
				if (!folderPaths.has(accumulated)) {
					const folder = await myStorageAPI.findOrCreateFolder(part, parent, allFolders);
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
				} catch (err) {
					console.warn('Gagal membuat folder kosong:', emptyRelPath, err);
				}
			}
		}

		// If there were only empty folders without files:
		if (!list.length) {
			await refreshStorage();
			toast.success('Folder berhasil dibuat', {
				description: `${emptyFolders!.length} folder dibuat.`
			});
			return;
		}

		const start = uploadQueue.length;
		uploadQueue = [
			...uploadQueue,
			...list.map((file, index) => ({
				name: paths?.[index] ? paths[index].replace(/\\/g, '/') : file.name,
				size: file.size,
				progress: 0,
				state: 'queued' as const
			}))
		];
		queueDismissed = false;

		let quotaAborted = false;

		for (let index = 0; index < list.length; index += 1) {
			const file = list[index];
			const queueIndex = start + index;
			try {
				let parentId = effectiveParentId;
				if (paths?.[index]) {
					const normPath = paths[index].replace(/\\/g, '/');
					const parts = normPath.split('/').filter(Boolean);
					parts.pop();
					if (parts.length > 0) {
						parentId = await ensureFolderPath(parts.join('/'));
					}
				}
				uploadQueue[queueIndex] = { ...uploadQueue[queueIndex], state: 'uploading' };
				await myStorageAPI.upload(file, parentId, (progress) => {
					uploadQueue[queueIndex] = { ...uploadQueue[queueIndex], progress: Math.min(progress, 99) };
				});
				uploadQueue[queueIndex] = { ...uploadQueue[queueIndex], progress: 100, state: 'done' };
			} catch (error: unknown) {
				const err = error as { message?: string; status?: number; code?: string };
				const isQuota =
					err?.status === 413 ||
					err?.code === 'USER_QUOTA_EXCEEDED' ||
					err?.code === 'STORAGE_POOL_FULL' ||
					(err?.message && err.message.toLowerCase().includes('kuota'));

				uploadQueue[queueIndex] = {
					...uploadQueue[queueIndex],
					state: 'failed',
					error: isQuota ? 'Kuota penyimpanan tidak cukup' : (err?.message ?? 'Gagal')
				};

				if (isQuota) {
					quotaAborted = true;
					for (let rem = index + 1; rem < list.length; rem += 1) {
						const remQueueIndex = start + rem;
						uploadQueue[remQueueIndex] = {
							...uploadQueue[remQueueIndex],
							state: 'failed',
							error: 'Dibatalkan: kuota tidak cukup'
						};
					}
					break;
				}
			}
		}

		await refreshStorage();
		const failed = uploadQueue.slice(start).filter((item) => item.state === 'failed').length;
		if (quotaAborted) {
			toast.error('Unggahan dihentikan: Kuota penyimpanan penuh', {
				description: 'Sebagian berkas gagal diunggah karena batas kuota terlampaui.'
			});
		} else {
			toast[failed ? 'warning' : 'success'](
				failed ? `${failed} unggahan gagal` : 'Unggahan selesai',
				{
					description: failed ? 'Periksa daftar unggahan untuk detail.' : `${list.length} item berhasil diunggah.`
				}
			);
		}
	}

	function triggerUpload() {
		fileInput?.click();
	}

	function triggerFolderUpload() {
		folderInput?.click();
	}

	onMount(() => {
		chrome.registerUploadHandler(triggerUpload);

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
		draggedEntryId = null;
		if (longPressTimer) {
			clearTimeout(longPressTimer);
			longPressTimer = null;
		}
	});
	async function onNewFolder(name: string) {
		if (variant === 'my-storage') {
			try {
				await myStorageAPI.createFolder(name, folderId);
				await refreshStorage();
				toast.success(`Folder “${name}” dibuat`);
			} catch (error) {
				const conflict = error as Error & { status?: number; suggested_name?: string };
				if (conflict.status === 409 && conflict.suggested_name) {
					conflictTitle = 'Folder Sudah Ada';
					conflictDescription = `Folder “${name}” sudah ada. Gunakan nama yang disarankan “${conflict.suggested_name}”?`;
					conflictAction = async () => {
						try {
							await myStorageAPI.createFolder(conflict.suggested_name!, folderId);
							await refreshStorage();
							toast.success(`Folder “${conflict.suggested_name}” dibuat`);
						} catch (retryError) {
							toast.error(retryError instanceof Error ? retryError.message : 'Gagal membuat folder');
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
			toast.error(`Folder “${name}” sudah ada.`);
			return;
		}
		tree = [...tree, { id: nextId('folder'), name, kind: 'folder', sizeBytes: 0,
			modifiedAt: 'Baru saja', parentId: folderId, storageNode: 'ssd' }];
		toast.success(`Folder “${name}” dibuat`);
	}

	async function runMutation(update: { name?: string; parent_id?: string | null }) {
		if (!actionEntry) return;
		try {
			await myStorageAPI.mutate(actionEntry.id, update);
		} catch (error) {
			const conflict = error as Error & { status?: number; suggested_name?: string };
			if (conflict.status === 409 && conflict.suggested_name) {
				conflictTitle = 'Nama Sudah Digunakan';
				conflictDescription = `Nama sudah digunakan. Gunakan nama yang disarankan “${conflict.suggested_name}”?`;
				conflictAction = async () => {
					try {
						await myStorageAPI.mutate(actionEntry!.id, { ...update, name: conflict.suggested_name });
						await refreshStorage();
						toast.success(update.name !== undefined ? 'Nama diperbarui' : `“${actionEntry!.name}” dipindahkan`);
					} catch (retryError) {
						toast.error(retryError instanceof Error ? retryError.message : 'Gagal memperbarui item');
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
		toast.success(update.name !== undefined ? 'Nama diperbarui' : `“${actionEntry.name}” dipindahkan`);
	}

	function onRename(name: string) {
		if (variant === 'my-storage') { void runMutation({ name }); return; }
		if (!actionEntry) return;
		tree = tree.map((e) => e.id === actionEntry!.id ? { ...e, name, modifiedAt: 'Baru saja' } : e);
		toast.success('Nama diperbarui');
	}

	function onMove(targetFolderId: string | null) {
		if (!actionEntry) return;
		if (actionEntry.kind === 'folder' && targetFolderId === actionEntry.id) {
			toast.error('Tidak dapat memindahkan folder ke dirinya sendiri.');
			return;
		}
		if (variant === 'my-storage') { void runMutation({ parent_id: targetFolderId }); return; }
		tree = tree.map((e) => e.id === actionEntry!.id ? { ...e, parentId: targetFolderId, modifiedAt: 'Baru saja' } : e);
		toast.success(`“${actionEntry.name}” dipindahkan`);
	}


	async function onCopy(entry: FileEntry) {
		if (variant === 'my-storage') {
			try {
				await myStorageAPI.duplicate(entry.id);
				await refreshStorage();
				toast.success(`“${entry.name}” diduplikasi`);
			} catch (error) {
				toast.error(error instanceof Error ? error.message : 'Gagal menduplikasi');
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
		tree = [...tree, { ...entry, id: nextId('copy'), name, sizeBytes: 0, modifiedAt: 'Baru saja',
			parentId: folderId, starred: false }];
		toast.success(`Salinan zero-byte “${name}” dibuat`);
	}

	function onStar(entry: FileEntry) {
		if (variant === 'my-storage') {
			currentItems = currentItems.map((item) => item.id === entry.id ? { ...item, starred: !item.starred } : item);
		} else {
			tree = tree.map((item) => item.id === entry.id ? { ...item, starred: !item.starred } : item);
		}
		toast.success(entry.starred ? 'Bintang dihapus' : 'Ditandai berbintang');
	}

	function onDelete(entry: FileEntry) {
		deleteTargetEntry = entry;
		deleteConfirmOpen = true;
	}

	async function executeDelete() {
		if (!deleteTargetEntry) return;
		const entry = deleteTargetEntry;
		if (variant === 'my-storage') {
			try {
				await myStorageAPI.remove(entry.id);
				const deletedBreadcrumb = apiBreadcrumbs.findIndex((item) => item.id === entry.id);
				if (deletedBreadcrumb >= 0 && folderId && apiBreadcrumbs.slice(deletedBreadcrumb).some((item) => item.id === folderId)) {
					navigateFolder(deletedBreadcrumb > 0 ? apiBreadcrumbs[deletedBreadcrumb - 1].id : null);
				}
				await refreshStorage();
				toast.success(`“${entry.name}” dipindahkan ke Trash`);
			} catch (error) { toast.error(error instanceof Error ? error.message : 'Gagal menghapus item'); }
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
		if (selectedId && removeIds.has(selectedId)) selectedId = null;
		if (folderId && removeIds.has(folderId)) navigateFolder(entry.parentId);
		toast.success(`“${entry.name}” dipindahkan ke Trash (mock)`);
	}

	async function onDownload(entry: FileEntry) {
		if (variant === 'my-storage') {
			try { await myStorageAPI.download(entry.id, entry.kind === 'folder' ? `${entry.name}.zip` : entry.name); }
			catch (error) { toast.error(error instanceof Error ? error.message : 'Gagal mengunduh'); }
			return;
		}
		toast.info(`Mengunduh “${entry.name}” (mock)`, { description: formatSize(entry.sizeBytes) });
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
			setTimeout(() => { dragStartAllowed = true; }, 100);
		}
		lastClickTime = now;
	}

	function handleItemDragStart(entry: FileEntry, e: DragEvent) {
		if (!dragStartAllowed) {
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
				const targetFolder = findEntry(tree, targetFolderId || '');

				if (targetFolder && targetFolder.kind === 'folder') {
					const movedEntry = findEntry(tree, draggedEntryId);
					if (movedEntry && movedEntry.id !== targetFolder.id) {
						// Circular move check
						if (movedEntry.kind === 'folder' && isAncestor(tree, movedEntry.id, targetFolder.id)) {
							toast.error('Tidak bisa memindahkan folder ke dalam subfolder-nya sendiri');
						} else if (variant === 'my-storage') {
							try {
								await myStorageAPI.mutate(movedEntry.id, { parent_id: targetFolder.id });
								await refreshStorage();
								toast.success(`"${movedEntry.name}" dipindahkan ke "${targetFolder.name}"`);
							} catch {
								toast.error('Gagal memindahkan berkas');
							}
						} else {
							// Complete move
							tree = tree.map((ent) =>
								ent.id === draggedEntryId
									? { ...ent, parentId: targetFolder.id, modifiedAt: 'Baru saja' }
									: ent
							);
							toast.success(`"${movedEntry.name}" dipindahkan ke "${targetFolder.name}"`);
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
			const movedEntry = findEntry(tree, draggedEntryId);
			if (!movedEntry || movedEntry.id === targetFolder.id) {
				draggedEntryId = null;
				return;
			}

			// Prevent circular move: folder cannot be moved into its own descendant
			if (movedEntry.kind === 'folder' && isAncestor(tree, movedEntry.id, targetFolder.id)) {
				toast.error('Tidak bisa memindahkan folder ke dalam subfolder-nya sendiri');
				draggedEntryId = null;
				return;
			}

			if (variant === 'my-storage') {
				try {
					await myStorageAPI.mutate(movedEntry.id, { parent_id: targetFolder.id });
					await refreshStorage();
					toast.success(`"${movedEntry.name}" dipindahkan ke "${targetFolder.name}"`);
				} catch {
					toast.error('Gagal memindahkan berkas');
				}
			} else {
				tree = tree.map((ent) =>
					ent.id === draggedEntryId
						? { ...ent, parentId: targetFolder.id, modifiedAt: 'Baru saja' }
						: ent
				);
				toast.success(`"${movedEntry.name}" dipindahkan ke "${targetFolder.name}"`);
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
			if (queueSwipeOffset > 80) {
				queueDismissed = true;
				queueStartX = null;
				queueSwipeOffset = 0;
				isSwipingQueue = false;
			}
		}
	}}
	onpointerup={() => {
		if (queueStartX !== null && queueSwipeOffset > 60) {
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
	aria-label="Pilih berkas untuk diunggah"
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
	aria-label="Pilih folder untuk diunggah"
	data-testid="upload-folder-input"
	multiple
	webkitdirectory
	onchange={(e) => {
		const input = e.currentTarget as HTMLInputElement;
		if (input.files) {
			const files = Array.from(input.files);
			void handleUploadFiles(files, files.map((file) => (file as File & { webkitRelativePath?: string }).webkitRelativePath ?? file.name));
		}
		input.value = '';
	}}
/>

<div class="flex flex-col gap-4">
	<FileExplorerBreadcrumbs segments={breadcrumbs} onNavigate={navigateFolder} />
	{#if storageLoading}
		<p class="text-sm text-muted" role="status">Memuat My Storage…</p>
	{/if}
	<FileExplorerToolbar bind:category onNewFolder={() => (newFolderOpen = true)} onUpload={triggerUpload} onUploadFolder={triggerFolderUpload} showFolderUpload={variant === 'my-storage'} />

	<FileExplorerDropzone active={dragActive}>
		{#if visible.length === 0}
			<div
				class="overflow-hidden rounded-xl border border-subtle bg-surface px-4 py-12 text-center"
			>
				<p class="text-sm font-medium text-main">Folder kosong</p>
				<p class="mt-1 text-xs text-muted">Seret berkas ke sini atau gunakan tombol Unggah.</p>
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
						ondragleave={entry.kind === 'folder' ? (e) => handleFolderDragLeave(entry, e) : undefined}
						ondrop={entry.kind === 'folder' ? (e) => handleFolderDrop(entry, e) : undefined}
		class={`${entry.id === draggedEntryId ? 'opacity-50 cursor-grabbing' : ''} ${
			entry.kind === 'folder' && dragOverFolderId === entry.id
				? 'ring-2 ring-primary/50 rounded-xl'
				: ''
		}`}
					>
						<FileExplorerContextMenu {entry} {...menuHandlers}>
							<FileGridCard
								kind={entry.kind}
								name={entry.name}
								subtitle={entry.kind === 'folder'
									? `Folder · ${entry.modifiedAt}`
									: `${formatSize(entry.sizeBytes)} · ${entry.modifiedAt}`}
								selected={selectedId === entry.id}
								onSelect={() => (selectedId = entry.id)}
								onOpen={() => openEntry(entry)}
							>
								{#snippet menu()}
									<FileExplorerActionsMenu {entry} {...menuHandlers} />
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
				aria-label="Daftar berkas"
			>
				{#each visible as entry (entry.id)}
					<div
						data-folder-id={entry.kind === 'folder' ? entry.id : undefined}
						draggable="true"
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
						ondragleave={entry.kind === 'folder' ? (e) => handleFolderDragLeave(entry, e) : undefined}
						ondrop={entry.kind === 'folder' ? (e) => handleFolderDrop(entry, e) : undefined}
						class={`${entry.id === draggedEntryId ? 'opacity-50 cursor-grabbing' : ''} ${
							entry.kind === 'folder' && dragOverFolderId === entry.id
								? 'ring-2 ring-primary/50'
								: ''
						}`}
					>
						<FileExplorerRow
							{entry}
							selected={selectedId === entry.id}
							onOpen={openEntry}
							onSelect={(e) => (selectedId = e.id)}
							onDownload={onDownload}
							onRename={(e) => {
								actionEntry = e;
								renameOpen = true;
							}}
							onMove={(e) => {
								actionEntry = e;
								moveOpen = true;
							}}
							onCopy={onCopy}
							onStar={onStar}
							onShare={(e) => {
								actionEntry = e;
								shareOpen = true;
							}}
							onDelete={onDelete}
						/>
					</div>
				{/each}
			</div>
		{/if}
	</FileExplorerDropzone>
</div>

{#if variant === 'my-storage' && uploadQueue.length > 0 && !queueDismissed}
	<aside
		transition:fly={{ x: 350, duration: 250 }}
		class="fixed left-4 right-4 top-20 z-50 ml-auto w-auto max-w-sm rounded-xl border border-subtle bg-surface p-4 text-main shadow-none"
		aria-label="Antrean unggahan"
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
				class="text-left text-sm font-semibold hover:opacity-80 transition-opacity"
				aria-expanded={queueOpen}
				onclick={() => (queueOpen = !queueOpen)}
			>
				{uploadQueue.some((item) => item.state === 'uploading') ? 'Mengunggah' : 'Unggahan'} · {uploadQueue.filter((item) => item.state === 'done').length}/{uploadQueue.length}
			</button>
			<div class="flex items-center gap-1 shrink-0">
				<button
					type="button"
					class="inline-flex size-6 items-center justify-center rounded-md text-muted hover:bg-muted/20 hover:text-main transition-colors"
					aria-label={queueOpen ? 'Ciutkan antrean unggahan' : 'Buka antrean unggahan'}
					onclick={() => (queueOpen = !queueOpen)}
				>
					{#if queueOpen}
						<ChevronUp class="size-4" />
					{:else}
						<ChevronDown class="size-4" />
					{/if}
				</button>
				{#if uploadQueue.every((item) => item.state === 'done' || item.state === 'failed')}
					<button
						type="button"
						class="inline-flex size-6 items-center justify-center rounded-md text-muted hover:bg-muted/20 hover:text-main transition-colors"
						aria-label="Tutup notifikasi unggahan"
						data-testid="dismiss-upload-queue"
						onclick={() => (queueDismissed = true)}
					>
						<X class="size-4" />
					</button>
				{/if}
			</div>
		</div>
		<div class="mt-2 h-1.5 overflow-hidden rounded-full bg-muted/30" role="progressbar" aria-label="Progress unggahan" aria-valuenow={uploadProgress} aria-valuemin="0" aria-valuemax="100">
			<div class="h-full bg-primary transition-all" style={`width:${uploadProgress}%`}></div>
</div>
		{#if queueOpen}
			<ul class="mt-3 max-h-56 space-y-2 overflow-y-auto text-xs">
				{#each uploadQueue as item, index (index)}
					<li class="flex justify-between gap-2" data-testid="upload-queue-item">
						<span class="min-w-0 truncate" title={item.name}>{item.name}</span>
						<span class={item.state === 'failed' ? 'text-destructive' : 'text-muted'}>
							{item.state === 'failed' ? item.error : item.state === 'done' ? 'Selesai' : `${item.progress}%`}
						</span>
					</li>
				{/each}
			</ul>
		{/if}
	</aside>
{/if}
<NewFolderDialog bind:open={newFolderOpen} onConfirm={onNewFolder} />
<RenameDialog
	bind:open={renameOpen}
	initialName={actionEntry?.name ?? ''}
	onConfirm={onRename}
/>
<MoveDialog
	bind:open={moveOpen}
	folders={moveFolders}
	currentParentId={actionEntry?.parentId ?? null}
	onConfirm={onMove}
/>
{#if variant !== 'my-storage'}
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
	onDownload={onDownload}
	onClose={closePreview}
/>
<ConfirmDialog
	bind:open={deleteConfirmOpen}
	title={deleteTargetEntry?.kind === 'folder' ? 'Pindahkan Folder ke Trash?' : 'Pindahkan Berkas ke Trash?'}
	description={deleteTargetEntry?.kind === 'folder'
		? `Pindahkan “${deleteTargetEntry.name}” dan semua isinya ke Trash?`
		: `Pindahkan “${deleteTargetEntry?.name}” ke Trash?`}
	confirmText="Hapus ke Trash"
	variant="destructive"
	onConfirm={executeDelete}
/>
<ConfirmDialog
	bind:open={conflictConfirmOpen}
	title={conflictTitle}
	description={conflictDescription}
	confirmText="Gunakan Nama Ini"
	onConfirm={async () => {
		if (conflictAction) await conflictAction();
	}}
/>
