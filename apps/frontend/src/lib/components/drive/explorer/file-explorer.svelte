<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { onDestroy, onMount, untrack } from 'svelte';
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
	import ShareSheet from './share-sheet.svelte';
	import FilePreviewDialog from './file-preview-dialog.svelte';
	import DriveFileGrid from '$lib/components/drive/drive-file-grid.svelte';
	import FileGridCard from '$lib/components/drive/file-grid-card.svelte';
	import StorageNodeBadge from './storage-node-badge.svelte';
	import { driveView } from '$lib/drive-view.svelte.js';

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
	let category = $state<FileCategory>('all');
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

	const folderId = $derived(page.url.searchParams.get('folder'));
	const previewId = $derived(page.url.searchParams.get('preview'));

	const breadcrumbs = $derived(getBreadcrumbPath(tree, folderId, rootLabel));
	const children = $derived(getChildren(tree, folderId));
	const visible = $derived(
		category === 'all' ? children : children.filter((e) => e.kind === category)
	);
	const previewEntry = $derived(previewId ? (findEntry(tree, previewId) ?? null) : null);
	const previewOpen = $derived(!!previewEntry);
	const moveFolders = $derived(
		getFolderOptions(tree).filter((f) => f.id !== actionEntry?.id)
	);

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

	function quotaRatio(): number | null {
		if (variant !== 'my-storage') return null;
		const { usedGb, limitGb } = chrome.quota;
		if (limitGb == null || limitGb <= 0) return null;
		return usedGb / limitGb;
	}

	function canUpload(): boolean {
		const ratio = quotaRatio();
		if (ratio == null) return true;
		if (ratio >= 1) {
			toast.error('Kuota My Storage penuh. Hubungi admin.', {
				description: 'USER_QUOTA_EXCEEDED'
			});
			return false;
		}
		if (ratio >= 0.8) {
			toast.warning('Kuota My Storage hampir penuh (≥ 80%).');
		}
		return true;
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

	function handleUploadFiles(files: FileList | File[]) {
		if (!canUpload()) return;
		const list = Array.from(files);
		if (list.length === 0) return;

		const file = list[0];
		const existing = findSiblingByName(file.name);
		if (existing) {
			pendingUpload = {
				name: file.name,
				size: file.size,
				kind: guessKind(file.name)
			};
			actionEntry = existing;
			conflictOpen = true;
			return;
		}
		addFile(file.name, file.size, guessKind(file.name));
	}

	function triggerUpload() {
		if (!canUpload()) return;
		fileInput?.click();
	}

	onMount(() => {
		chrome.registerUploadHandler(triggerUpload);
	});

	onDestroy(() => {
		chrome.registerUploadHandler(null);
		draggedEntryId = null;
		if (longPressTimer) {
			clearTimeout(longPressTimer);
			longPressTimer = null;
		}
	});

	function onNewFolder(name: string) {
		if (findSiblingByName(name)) {
			toast.error(`Folder “${name}” sudah ada.`);
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
		toast.success(`Folder “${name}” dibuat`);
	}

	function onRename(name: string) {
		if (!actionEntry) return;
		const clash = tree.find(
			(e) => e.parentId === actionEntry!.parentId && e.name === name && e.id !== actionEntry!.id
		);
		if (clash) {
			toast.error(`Nama “${name}” sudah dipakai.`);
			return;
		}
		tree = tree.map((e) =>
			e.id === actionEntry!.id ? { ...e, name, modifiedAt: 'Baru saja' } : e
		);
		toast.success('Nama diperbarui');
	}

	function onMove(targetFolderId: string | null) {
		if (!actionEntry) return;
		if (actionEntry.kind === 'folder' && targetFolderId === actionEntry.id) {
			toast.error('Tidak dapat memindahkan folder ke dirinya sendiri.');
			return;
		}
		tree = tree.map((e) =>
			e.id === actionEntry!.id
				? { ...e, parentId: targetFolderId, modifiedAt: 'Baru saja' }
				: e
		);
		toast.success(`“${actionEntry.name}” dipindahkan`);
	}

	function onCopy(entry: FileEntry) {
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
		toast.success(`Salinan zero-byte “${name}” dibuat`);
	}

	function onStar(entry: FileEntry) {
		tree = tree.map((e) => (e.id === entry.id ? { ...e, starred: !e.starred } : e));
		toast.success(entry.starred ? 'Bintang dihapus' : 'Ditandai berbintang');
	}

	function onDelete(entry: FileEntry) {
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

	function onDownload(entry: FileEntry) {
		toast.info(`Mengunduh “${entry.name}” (mock)`, {
			description: formatSize(entry.sizeBytes)
		});
	}

	function onDragEnter(e: DragEvent) {
		// Ignore internal item drag - check dataTransfer types
		// Internal drag has 'text/plain' but no 'Files' type
		const types = e.dataTransfer?.types || [];
		const isInternalDrag = types.includes('text/plain') && !types.includes('Files');
		if (isInternalDrag) return;
		
		e.preventDefault();
		dragDepth += 1;
		dragActive = true;
	}

	function onDragOver(e: DragEvent) {
		// Ignore internal item drag
		const types = e.dataTransfer?.types || [];
		const isInternalDrag = types.includes('text/plain') && !types.includes('Files');
		if (isInternalDrag) return;
		
		e.preventDefault();
		if (e.dataTransfer) e.dataTransfer.dropEffect = 'copy';
	}

	function onDragLeave(e: DragEvent) {
		// Ignore internal item drag
		const types = e.dataTransfer?.types || [];
		const isInternalDrag = types.includes('text/plain') && !types.includes('Files');
		if (isInternalDrag) return;
		
		e.preventDefault();
		dragDepth = Math.max(0, dragDepth - 1);
		if (dragDepth === 0) dragActive = false;
	}

	function onDrop(e: DragEvent) {
		// Ignore internal item drag
		const types = e.dataTransfer?.types || [];
		const isInternalDrag = types.includes('text/plain') && !types.includes('Files');
		if (isInternalDrag) return;
		
		e.preventDefault();
		dragDepth = 0;
		dragActive = false;
		if (e.dataTransfer?.files?.length) {
			handleUploadFiles(e.dataTransfer.files);
		}
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
			// ponytail: no visual feedback yet, add vibration/highlight when needed
		}, 200);
	}

	function handleTouchEnd(e: TouchEvent) {
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
		if (e.dataTransfer) e.dataTransfer.dropEffect = 'move';
		dragOverFolderId = entry.id;
	}
	function handleFolderDrop(targetFolder: FileEntry, e: DragEvent) {
		e.preventDefault();
		e.stopPropagation();
		if (!draggedEntryId) return;
		
		const movedEntry = findEntry(tree, draggedEntryId);
		if (!movedEntry) return;
		
		// Prevent self-drop
		if (movedEntry.id === targetFolder.id) return;
		
		
		// Prevent circular move: folder cannot be moved into its own descendant
		if (movedEntry.kind === 'folder' && isAncestor(tree, movedEntry.id, targetFolder.id)) {
			toast.error(`Tidak bisa memindahkan folder ke dalam subfolder-nya sendiri`);
			draggedEntryId = null;
			return;
		}
		
		// Update tree
		tree = tree.map((ent) =>
			ent.id === draggedEntryId
				? { ...ent, parentId: targetFolder.id, modifiedAt: 'Baru saja' }
				: ent
		);
		toast.success(`"${movedEntry.name}" dipindahkan ke "${targetFolder.name}"`);
		draggedEntryId = null;
	}
</script>

<input
	bind:this={fileInput}
	type="file"
	class="hidden"
	multiple
	onchange={(e) => {
		const files = (e.currentTarget as HTMLInputElement).files;
		if (files) handleUploadFiles(files);
		(e.currentTarget as HTMLInputElement).value = '';
	}}
/>

<div class="flex flex-col gap-4">
	<FileExplorerBreadcrumbs segments={breadcrumbs} onNavigate={navigateFolder} />
	<FileExplorerToolbar bind:category onNewFolder={() => (newFolderOpen = true)} onUpload={() => fileInput?.click()} />

	<FileExplorerDropzone
		active={dragActive}
		ondragenter={onDragEnter}
		ondragover={onDragOver}
		ondragleave={onDragLeave}
		ondrop={onDrop}
	>
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
<ShareSheet bind:open={shareOpen} entry={actionEntry} />
<FilePreviewDialog
	open={previewOpen}
	entry={previewEntry}
	onDownload={onDownload}
	onClose={closePreview}
/>
