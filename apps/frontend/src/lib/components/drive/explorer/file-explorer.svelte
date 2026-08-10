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
	import ShareLinkSheet from './share-link-sheet.svelte';
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
		e.preventDefault();
		dragDepth += 1;
		dragActive = true;
	}

	function onDragOver(e: DragEvent) {
		e.preventDefault();
		if (e.dataTransfer) e.dataTransfer.dropEffect = 'copy';
	}

	function onDragLeave(e: DragEvent) {
		e.preventDefault();
		dragDepth = Math.max(0, dragDepth - 1);
		if (dragDepth === 0) dragActive = false;
	}

	function onDrop(e: DragEvent) {
		e.preventDefault();
		dragDepth = 0;
		dragActive = false;
		if (e.dataTransfer?.files?.length) {
			handleUploadFiles(e.dataTransfer.files);
		}
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
	<FileExplorerToolbar bind:category onNewFolder={() => (newFolderOpen = true)} />

	<FileExplorerDropzone
		active={dragActive}
		ondragenter={onDragEnter}
		ondragover={onDragOver}
		ondragleave={onDragLeave}
		ondrop={onDrop}
	>
		{#if visible.length === 0}
			<div class="overflow-hidden rounded-xl border border-subtle bg-surface px-4 py-12 text-center">
				<p class="text-sm font-medium text-main">Folder kosong</p>
				<p class="mt-1 text-xs text-muted">Seret berkas ke sini atau gunakan tombol Unggah.</p>
			</div>
		{:else if driveView.mode === 'grid'}
			<DriveFileGrid>
				{#each visible as entry (entry.id)}
					{@const menuHandlers = {
						onOpen: openEntry,
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
				{/each}
			</DriveFileGrid>
		{:else}
			<div
				class="overflow-hidden rounded-xl border border-subtle bg-surface"
				role="listbox"
				aria-label="Daftar berkas"
			>
				{#each visible as entry (entry.id)}
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
<ShareLinkSheet bind:open={shareOpen} entry={actionEntry} />
<FilePreviewDialog
	open={previewOpen}
	entry={previewEntry}
	onDownload={onDownload}
	onClose={closePreview}
/>
