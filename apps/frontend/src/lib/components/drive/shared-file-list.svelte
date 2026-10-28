<script lang="ts">
	import { locale } from '$lib/locale.svelte.js';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { onDestroy, onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import type { FileEntry, SharedFileItem } from '$lib/types/drive.js';
	import { formatSize } from '$lib/mocks/file-tree.js';
	import { driveView } from '$lib/drive-view.svelte.js';
	import { getDriveChrome } from '$lib/drive-context.svelte.js';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu/index.js';
	import * as ContextMenu from '$lib/components/ui/context-menu/index.js';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import {
		Archive,
		Download,
		Eye,
		FolderOpen,
		FolderPlus,
		Info,
		Loader2,
		MoreVertical,
		Pencil,
		Trash2,
		Upload
	} from '@lucide/svelte';
	import SharedFileRow from './shared-file-row.svelte';
	import ShareDetailsDialog from './share-details-dialog.svelte';
	import FilePreviewDialog from './explorer/file-preview-dialog.svelte';
	import { previewUrl } from '$lib/api/preview.js';
	import DriveFileGrid from './drive-file-grid.svelte';
	import FileGridCard from './file-grid-card.svelte';
	import PermissionBadge from './permission-badge.svelte';
	import {
		getSharedWithMe,
		getSharedFolder,
		uploadSharedFile,
		createSharedFolder,
		renameSharedItem,
		deleteSharedItem,
		downloadSharedItem,
		downloadSharedFolderZip,
		type SharePermission,
		type SharedItem,
		detectFileKind
	} from '$lib/api/share.js';
	const tx = (id: string, en: string) => (locale.language === 'id' ? id : en);

	let {
		initialItems
	}: {
		initialItems?: SharedFileItem[];
	} = $props();

	const chrome = getDriveChrome();

	let items = $state<SharedFileItem[]>([]);
	let loading = $state(true);
	let activeFolderId = $state<string | null>(null);
	let breadcrumbs = $state<{ id: string; name: string }[]>([]);
	let folderPermission = $state<SharePermission>('viewer');
	let currentFolderName = $state('');

	let selectedId = $state<string | null>(null);
	let detailsOpen = $state(false);
	let actionEntry = $state<SharedFileItem | null>(null);

	let isCreateFolderOpen = $state(false);
	let newFolderName = $state('');
	let isRenameOpen = $state(false);
	let renameTarget = $state<SharedFileItem | null>(null);
	let renameNewName = $state('');
	let isDeleteConfirmOpen = $state(false);
	let deleteTarget = $state<SharedFileItem | null>(null);
	let uploading = $state(false);
	let fileInput = $state<HTMLInputElement | null>(null);

	const previewId = $derived(page.url.searchParams.get('preview'));
	const previewEntry = $derived(previewId ? (items.find((e) => e.id === previewId) ?? null) : null);
	const previewOpen = $derived(!!previewEntry);

	function toSharedFileItem(it: SharedItem): SharedFileItem {
		return {
			id: it.id,
			name: it.name,
			kind: detectFileKind(it.name, it.kind),
			sizeBytes: it.size_bytes,
			modifiedAt: it.modified_at,
			parentId: it.parent_id ?? null,
			ownerName: it.owner_name,
			permission: it.permission
		};
	}

	async function loadRoot() {
		chrome.setSharedUploadContext(null, null);
		loading = true;
		try {
			const res = await getSharedWithMe();
			items = (res.items ?? []).map(toSharedFileItem);
			breadcrumbs = [];
			folderPermission = 'viewer';
			currentFolderName = '';
		} catch (err) {
			toast.error(
				err instanceof Error
					? err.message
					: tx('Gagal memuat berkas dibagikan', 'Failed to load shared files')
			);
		} finally {
			loading = false;
		}
	}

	async function loadFolder(folderId: string) {
		loading = true;
		chrome.setSharedUploadContext(folderId, null);
		try {
			const res = await getSharedFolder(folderId);
			items = (res.items ?? []).map(toSharedFileItem);
			breadcrumbs = res.breadcrumbs ?? [];
			folderPermission = res.permission;
			currentFolderName = breadcrumbs[breadcrumbs.length - 1]?.name || 'Folder';
			chrome.setSharedUploadContext(folderId, res.permission);
		} catch (err) {
			toast.error(
				err instanceof Error
					? err.message
					: tx('Gagal memuat folder bersama', 'Failed to load shared folder')
			);
			activeFolderId = null;
			await loadRoot();
			chrome.setSharedUploadContext(null, null);
		} finally {
			loading = false;
		}
	}

	onDestroy(() => {
		chrome.setSharedUploadContext(null, null);
		chrome.registerStorageChangeHandler?.(null);
	});

	onMount(() => {
		chrome.registerStorageChangeHandler?.(() => {
			if (activeFolderId) void loadFolder(activeFolderId);
		});
		const folderId = page.url.searchParams.get('folder');
		if (folderId) {
			activeFolderId = folderId;
			void loadFolder(folderId);
		} else if (initialItems && initialItems.length > 0) {
			items = initialItems;
			loading = false;
			chrome.setSharedUploadContext(null, null);
		} else {
			void loadRoot();
		}
	});

	function syncFolder(id: string | null) {
		const params = new URLSearchParams(page.url.searchParams);
		if (id) params.set('folder', id);
		else params.delete('folder');
		params.delete('preview');
		const qs = params.toString();
		void goto(`${page.url.pathname}${qs ? `?${qs}` : ''}`, {
			replaceState: true,
			keepFocus: true,
			noScroll: true
		});
	}

	function syncPreview(id: string | null) {
		const params = new URLSearchParams(page.url.searchParams);
		if (id) params.set('preview', id);
		else params.delete('preview');
		const qs = params.toString();
		void goto(`${page.url.pathname}${qs ? `?${qs}` : ''}`, {
			replaceState: true,
			keepFocus: true,
			noScroll: true
		});
	}

	function onOpen(entry: SharedFileItem) {
		if (entry.kind === 'folder') {
			activeFolderId = entry.id;
			syncFolder(entry.id);
			void loadFolder(entry.id);
			return;
		}
		syncPreview(entry.id);
	}

	function navigateBreadcrumb(id: string | null) {
		if (id === null) {
			activeFolderId = null;
			syncFolder(null);
			void loadRoot();
		} else {
			activeFolderId = id;
			syncFolder(id);
			void loadFolder(id);
		}
	}

	function onDownload(entry: SharedFileItem) {
		if (entry.kind === 'folder') {
			downloadSharedFolderZip(entry.id, entry.name);
		} else {
			downloadSharedItem(entry.id, entry.name);
		}
	}

	function openRenameDialog(entry: SharedFileItem) {
		renameTarget = entry;
		renameNewName = entry.name;
		isRenameOpen = true;
	}

	function openDeleteDialog(entry: SharedFileItem) {
		deleteTarget = entry;
		isDeleteConfirmOpen = true;
	}

	async function handleUpload(e: Event) {
		const files = (e.target as HTMLInputElement).files;
		if (!files || files.length === 0 || !activeFolderId) return;
		uploading = true;
		let successCount = 0;
		for (let i = 0; i < files.length; i++) {
			try {
				await uploadSharedFile(activeFolderId, files[i]);
				successCount++;
			} catch (err) {
				toast.error(
					tx(
						`Gagal mengunggah ${files[i].name}: ${err instanceof Error ? err.message : ''}`,
						`Failed to upload ${files[i].name}: ${err instanceof Error ? err.message : ''}`
					)
				);
			}
		}
		uploading = false;
		if (successCount > 0) {
			toast.success(
				tx(
					`${successCount} berkas berhasil diunggah`,
					`${successCount} files uploaded successfully`
				)
			);
			void loadFolder(activeFolderId);
		}
		if (fileInput) fileInput.value = '';
	}

	async function handleCreateFolder() {
		if (!newFolderName.trim() || !activeFolderId) return;
		try {
			await createSharedFolder(activeFolderId, newFolderName.trim());
			toast.success(
				tx(`Folder “${newFolderName.trim()}” dibuat`, `Folder “${newFolderName.trim()}” created`)
			);
			isCreateFolderOpen = false;
			newFolderName = '';
			void loadFolder(activeFolderId);
		} catch (err) {
			toast.error(
				err instanceof Error ? err.message : tx('Gagal membuat folder', 'Failed to create folder')
			);
		}
	}

	async function handleRename() {
		if (!renameTarget || !renameNewName.trim()) return;
		try {
			await renameSharedItem(renameTarget.id, renameNewName.trim());
			toast.success(tx('Nama berhasil diubah', 'Name updated'));
			isRenameOpen = false;
			renameTarget = null;
			renameNewName = '';
			if (activeFolderId) void loadFolder(activeFolderId);
			else void loadRoot();
		} catch (err) {
			toast.error(
				err instanceof Error ? err.message : tx('Gagal mengubah nama', 'Failed to rename')
			);
		}
	}

	async function handleDelete() {
		if (!deleteTarget) return;
		try {
			await deleteSharedItem(deleteTarget.id);
			toast.success(
				tx(`“${deleteTarget.name}” berhasil dihapus`, `“${deleteTarget.name}” deleted successfully`)
			);
			isDeleteConfirmOpen = false;
			deleteTarget = null;
			if (activeFolderId) void loadFolder(activeFolderId);
			else void loadRoot();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : tx('Gagal menghapus', 'Failed to delete'));
		}
	}

	const isInsideFolder = $derived(activeFolderId !== null);
	const canEditInFolder = $derived(isInsideFolder && folderPermission === 'editor');
</script>

<div class="flex flex-col gap-4">
	{#if isInsideFolder}
		<div
			class="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-subtle bg-surface px-4 py-3"
		>
			<div class="flex items-center gap-2 overflow-x-auto text-sm">
				<button
					type="button"
					class="font-medium text-muted transition-colors hover:text-main"
					onclick={() => navigateBreadcrumb(null)}
				>
					{tx('Dibagikan pada saya', 'Shared with me')}
				</button>
				{#each breadcrumbs as crumb, i (crumb.id)}
					<span class="text-muted">/</span>
					{#if i === breadcrumbs.length - 1}
						<span class="font-semibold text-main">{crumb.name}</span>
					{:else}
						<button
							type="button"
							class="font-medium text-muted transition-colors hover:text-main"
							onclick={() => navigateBreadcrumb(crumb.id)}
						>
							{crumb.name}
						</button>
					{/if}
				{/each}
			</div>

			<div class="flex items-center gap-2">
				<PermissionBadge permission={folderPermission} />
				{#if canEditInFolder}
					<input
						type="file"
						multiple
						bind:this={fileInput}
						onchange={handleUpload}
						class="hidden"
					/>
					<Button
						type="button"
						variant="secondary"
						size="sm"
						class="gap-1.5"
						onclick={() => (isCreateFolderOpen = true)}
					>
						<FolderPlus class="size-4 stroke-2" />
						{tx('Folder baru', 'New folder')}
					</Button>
					<Button
						type="button"
						size="sm"
						class="gap-1.5"
						disabled={uploading}
						onclick={() => fileInput?.click()}
					>
						{#if uploading}
							<Loader2 class="size-4 animate-spin stroke-2" />
						{:else}
							<Upload class="size-4 stroke-2" />
						{/if}
						{tx('Unggah', 'Upload')}
					</Button>
				{/if}
				<Button
					type="button"
					variant="ghost"
					size="sm"
					class="gap-1.5 text-muted hover:text-main"
					onclick={() =>
						activeFolderId && downloadSharedFolderZip(activeFolderId, currentFolderName)}
				>
					<Archive class="size-4 stroke-2" />
					{tx('Unduh ZIP', 'Download ZIP')}
				</Button>
			</div>
		</div>
	{/if}

	{#if loading}
		<div class="flex items-center justify-center p-12 text-muted">
			<Loader2 class="mr-2 size-5 animate-spin text-primary" />
			<span class="text-sm font-medium"
				>{tx('Memuat berkas bersama...', 'Loading shared files...')}</span
			>
		</div>
	{:else if items.length === 0}
		<div class="overflow-hidden rounded-xl border border-subtle bg-surface px-4 py-12 text-center">
			<p class="text-sm font-medium text-main">
				{isInsideFolder
					? tx('Folder ini kosong', 'This folder is empty')
					: tx('Belum ada berkas dibagikan', 'No shared files yet')}
			</p>
			<p class="mt-1 text-xs text-muted">
				{isInsideFolder
					? tx(
							'Gunakan tombol Unggah atau Folder baru untuk menambahkan konten.',
							'Use Upload or New folder to add content.'
						)
					: tx(
							'Berkas atau folder yang dibagikan pengguna lain ke Anda akan muncul di sini.',
							'Files and folders shared with you by other users will appear here.'
						)}
			</p>
		</div>
	{:else if driveView.mode === 'grid'}
		<DriveFileGrid label={tx('Berkas dibagikan', 'Shared files')}>
			{#each items as entry (entry.id)}
				{@const isFolder = entry.kind === 'folder'}
				{@const canEditItem = isInsideFolder && folderPermission === 'editor'}
				<ContextMenu.Root>
					<ContextMenu.Trigger class="block w-full">
						<FileGridCard
							kind={entry.kind}
							name={entry.name}
							subtitle={isFolder
								? tx(`Folder · Dari ${entry.ownerName}`, `Folder · From ${entry.ownerName}`)
								: tx(
										`${formatSize(entry.sizeBytes)} · Dari ${entry.ownerName}`,
										`${formatSize(entry.sizeBytes)} · From ${entry.ownerName}`
									)}
							previewUrl={!isFolder ? previewUrl('shared', entry.id) : undefined}
							selected={selectedId === entry.id}
							onSelect={() => (selectedId = entry.id)}
							onOpen={() => onOpen(entry)}
						>
							{#snippet badges()}
								<PermissionBadge permission={entry.permission} />
							{/snippet}
							{#snippet menu()}
								<DropdownMenu.Root>
									<DropdownMenu.Trigger>
										{#snippet child({ props })}
											<Button
												{...props}
												type="button"
												variant="ghost"
												size="icon"
												class="bg-surface text-muted hover:bg-primary hover:text-white aria-expanded:bg-primary aria-expanded:text-white"
												aria-label={tx('Aksi berkas', 'File actions')}
												onclick={(e: MouseEvent) => e.stopPropagation()}
											>
												<MoreVertical class="size-4 stroke-2" />
											</Button>
										{/snippet}
									</DropdownMenu.Trigger>
									<DropdownMenu.Content align="end" class="min-w-48 border-subtle bg-surface">
										{#if isFolder}
											<DropdownMenu.Item class="gap-2" onclick={() => onOpen(entry)}>
												<FolderOpen class="size-4 stroke-2" />
												{tx('Buka folder', 'Open folder')}
											</DropdownMenu.Item>
										{:else}
											<DropdownMenu.Item class="gap-2" onclick={() => onOpen(entry)}>
												<Eye class="size-4 stroke-2" />
												{tx('Pratinjau', 'Preview')}
											</DropdownMenu.Item>
										{/if}
										<DropdownMenu.Item class="gap-2" onclick={() => onDownload(entry)}>
											<Download class="size-4 stroke-2" />
											{tx('Unduh', 'Download')}
										</DropdownMenu.Item>
										<DropdownMenu.Item
											class="gap-2"
											onclick={() => {
												actionEntry = entry;
												detailsOpen = true;
											}}
										>
											<Info class="size-4 stroke-2" />
											{tx('Detail berbagi', 'Sharing details')}
										</DropdownMenu.Item>
										{#if canEditItem}
											<DropdownMenu.Separator />
											<DropdownMenu.Item class="gap-2" onclick={() => openRenameDialog(entry)}>
												<Pencil class="size-4 stroke-2" />
												{tx('Ubah nama', 'Rename')}
											</DropdownMenu.Item>
											<DropdownMenu.Item
												variant="destructive"
												class="gap-2"
												onclick={() => openDeleteDialog(entry)}
											>
												<Trash2 class="size-4 stroke-2" />
												{tx('Hapus', 'Delete')}
											</DropdownMenu.Item>
										{/if}
									</DropdownMenu.Content>
								</DropdownMenu.Root>
							{/snippet}
						</FileGridCard>
					</ContextMenu.Trigger>
					<ContextMenu.Content class="min-w-48 border-subtle bg-surface">
						{#if isFolder}
							<ContextMenu.Item class="gap-2" onclick={() => onOpen(entry)}>
								<FolderOpen class="size-4 stroke-2" />
								{tx('Buka folder', 'Open folder')}
							</ContextMenu.Item>
						{:else}
							<ContextMenu.Item class="gap-2" onclick={() => onOpen(entry)}>
								<Eye class="size-4 stroke-2" />
								{tx('Pratinjau', 'Preview')}
							</ContextMenu.Item>
						{/if}
						<ContextMenu.Item class="gap-2" onclick={() => onDownload(entry)}>
							<Download class="size-4 stroke-2" />
							{tx('Unduh', 'Download')}
						</ContextMenu.Item>
						<ContextMenu.Item
							class="gap-2"
							onclick={() => {
								actionEntry = entry;
								detailsOpen = true;
							}}
						>
							<Info class="size-4 stroke-2" />
							{tx('Detail berbagi', 'Sharing details')}
						</ContextMenu.Item>
						{#if canEditItem}
							<ContextMenu.Separator />
							<ContextMenu.Item class="gap-2" onclick={() => openRenameDialog(entry)}>
								<Pencil class="size-4 stroke-2" />
								{tx('Ubah nama', 'Rename')}
							</ContextMenu.Item>
							<ContextMenu.Item
								variant="destructive"
								class="gap-2"
								onclick={() => openDeleteDialog(entry)}
							>
								<Trash2 class="size-4 stroke-2" />
								{tx('Hapus', 'Delete')}
							</ContextMenu.Item>
						{/if}
					</ContextMenu.Content>
				</ContextMenu.Root>
			{/each}
		</DriveFileGrid>
	{:else}
		<div
			class="overflow-hidden rounded-xl border border-subtle bg-surface"
			role="listbox"
			aria-label={tx('Berkas dibagikan', 'Shared files')}
		>
			{#each items as entry (entry.id)}
				<SharedFileRow
					{entry}
					selected={selectedId === entry.id}
					canEdit={canEditInFolder}
					onSelect={(e) => (selectedId = e.id)}
					{onOpen}
					{onDownload}
					onDetails={(e) => {
						actionEntry = e;
						detailsOpen = true;
					}}
					onRename={canEditInFolder ? openRenameDialog : undefined}
					onDelete={canEditInFolder ? openDeleteDialog : undefined}
				/>
			{/each}
		</div>
	{/if}
</div>

<!-- Details Dialog -->
<ShareDetailsDialog bind:open={detailsOpen} entry={actionEntry} />

<!-- Preview Dialog -->
<FilePreviewDialog
	open={previewOpen}
	entry={previewEntry}
	source="shared"
	onDownload={(e: FileEntry) => onDownload(e as SharedFileItem)}
	onClose={() => syncPreview(null)}
/>

<!-- Create Folder Dialog -->
<Dialog.Root bind:open={isCreateFolderOpen}>
	<Dialog.Content class="border-subtle bg-surface text-main">
		<Dialog.Header>
			<Dialog.Title>{tx('Folder Baru', 'New folder')}</Dialog.Title>
			<Dialog.Description
				>{tx(
					'Buat folder baru di dalam folder bersama ini.',
					'Create a new folder in this shared folder.'
				)}</Dialog.Description
			>
		</Dialog.Header>
		<div class="grid gap-2 py-2">
			<Label for="shared-folder-name">{tx('Nama Folder', 'Folder name')}</Label>
			<Input
				id="shared-folder-name"
				bind:value={newFolderName}
				placeholder={tx('Folder baru', 'New folder')}
				autocomplete="off"
				onkeydown={(e) => e.key === 'Enter' && handleCreateFolder()}
			/>
		</div>
		<Dialog.Footer>
			<Button variant="secondary" onclick={() => (isCreateFolderOpen = false)}
				>{tx('Batal', 'Cancel')}</Button
			>
			<Button onclick={handleCreateFolder}>{tx('Buat', 'Create')}</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

<!-- Rename Dialog -->
<Dialog.Root bind:open={isRenameOpen}>
	<Dialog.Content class="border-subtle bg-surface text-main">
		<Dialog.Header>
			<Dialog.Title>{tx('Ubah Nama', 'Rename')}</Dialog.Title>
			<Dialog.Description
				>{tx(
					'Masukkan nama baru untuk berkas atau folder ini.',
					'Enter a new name for this file or folder.'
				)}</Dialog.Description
			>
		</Dialog.Header>
		<div class="grid gap-2 py-2">
			<Label for="shared-rename-name">{tx('Nama Baru', 'New name')}</Label>
			<Input
				id="shared-rename-name"
				bind:value={renameNewName}
				autocomplete="off"
				onkeydown={(e) => e.key === 'Enter' && handleRename()}
			/>
		</div>
		<Dialog.Footer>
			<Button variant="secondary" onclick={() => (isRenameOpen = false)}
				>{tx('Batal', 'Cancel')}</Button
			>
			<Button onclick={handleRename}>{tx('Simpan', 'Save')}</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

<!-- Delete Dialog -->
<Dialog.Root bind:open={isDeleteConfirmOpen}>
	<Dialog.Content class="border-subtle bg-surface text-main">
		<Dialog.Header>
			<Dialog.Title>{tx('Hapus Berkas / Folder', 'Delete file / folder')}</Dialog.Title>
			<Dialog.Description>
				{tx(
					`Apakah Anda yakin ingin menghapus “${deleteTarget?.name}”? Tindakan ini tidak dapat dibatalkan.`,
					`Are you sure you want to delete “${deleteTarget?.name}”? This action cannot be undone.`
				)}
			</Dialog.Description>
		</Dialog.Header>
		<Dialog.Footer>
			<Button variant="secondary" onclick={() => (isDeleteConfirmOpen = false)}
				>{tx('Batal', 'Cancel')}</Button
			>
			<Button variant="destructive" onclick={handleDelete}>{tx('Hapus', 'Delete')}</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
