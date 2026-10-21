<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { onDestroy, onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import type { SharedFileItem } from '$lib/types/drive.js';
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
	const previewEntry = $derived(
		previewId ? (items.find((e) => e.id === previewId) ?? null) : null
	);
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
			toast.error(err instanceof Error ? err.message : 'Gagal memuat berkas dibagikan');
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
			toast.error(err instanceof Error ? err.message : 'Gagal memuat folder bersama');
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
				toast.error(`Gagal mengunggah ${files[i].name}: ${err instanceof Error ? err.message : ''}`);
			}
		}
		uploading = false;
		if (successCount > 0) {
			toast.success(`${successCount} berkas berhasil diunggah`);
			void loadFolder(activeFolderId);
		}
		if (fileInput) fileInput.value = '';
	}

	async function handleCreateFolder() {
		if (!newFolderName.trim() || !activeFolderId) return;
		try {
			await createSharedFolder(activeFolderId, newFolderName.trim());
			toast.success(`Folder “${newFolderName.trim()}” dibuat`);
			isCreateFolderOpen = false;
			newFolderName = '';
			void loadFolder(activeFolderId);
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Gagal membuat folder');
		}
	}

	async function handleRename() {
		if (!renameTarget || !renameNewName.trim()) return;
		try {
			await renameSharedItem(renameTarget.id, renameNewName.trim());
			toast.success('Nama berhasil diubah');
			isRenameOpen = false;
			renameTarget = null;
			renameNewName = '';
			if (activeFolderId) void loadFolder(activeFolderId);
			else void loadRoot();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Gagal mengubah nama');
		}
	}

	async function handleDelete() {
		if (!deleteTarget) return;
		try {
			await deleteSharedItem(deleteTarget.id);
			toast.success(`“${deleteTarget.name}” berhasil dihapus`);
			isDeleteConfirmOpen = false;
			deleteTarget = null;
			if (activeFolderId) void loadFolder(activeFolderId);
			else void loadRoot();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Gagal menghapus');
		}
	}

	const isInsideFolder = $derived(activeFolderId !== null);
	const canEditInFolder = $derived(isInsideFolder && folderPermission === 'editor');
</script>

<div class="flex flex-col gap-4">
	{#if isInsideFolder}
		<div class="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-subtle bg-surface px-4 py-3">
			<div class="flex items-center gap-2 overflow-x-auto text-sm">
				<button
					type="button"
					class="font-medium text-muted transition-colors hover:text-main"
					onclick={() => navigateBreadcrumb(null)}
				>
					Dibagikan pada saya
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
						Folder baru
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
						Unggah
					</Button>
				{/if}
				<Button
					type="button"
					variant="ghost"
					size="sm"
					class="gap-1.5 text-muted hover:text-main"
					onclick={() => activeFolderId && downloadSharedFolderZip(activeFolderId, currentFolderName)}
				>
					<Archive class="size-4 stroke-2" />
					Unduh ZIP
				</Button>
			</div>
		</div>
	{/if}

	{#if loading}
		<div class="flex items-center justify-center p-12 text-muted">
			<Loader2 class="mr-2 size-5 animate-spin text-primary" />
			<span class="text-sm font-medium">Memuat berkas bersama...</span>
		</div>
	{:else if items.length === 0}
		<div class="overflow-hidden rounded-xl border border-subtle bg-surface px-4 py-12 text-center">
			<p class="text-sm font-medium text-main">
				{isInsideFolder ? 'Folder ini kosong' : 'Belum ada berkas dibagikan'}
			</p>
			<p class="mt-1 text-xs text-muted">
				{isInsideFolder
					? 'Gunakan tombol Unggah atau Folder baru untuk menambahkan konten.'
					: 'Berkas atau folder yang dibagikan pengguna lain ke Anda akan muncul di sini.'}
			</p>
		</div>
	{:else if driveView.mode === 'grid'}
		<DriveFileGrid label="Berkas dibagikan">
			{#each items as entry (entry.id)}
				{@const isFolder = entry.kind === 'folder'}
				{@const canEditItem = isInsideFolder && folderPermission === 'editor'}
				<ContextMenu.Root>
					<ContextMenu.Trigger class="block w-full">
						<FileGridCard
							kind={entry.kind}
							name={entry.name}
							subtitle={isFolder
								? `Folder · Dari ${entry.ownerName}`
								: `${formatSize(entry.sizeBytes)} · Dari ${entry.ownerName}`}
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
												aria-label="Aksi berkas"
												onclick={(e: MouseEvent) => e.stopPropagation()}
											>
												<MoreVertical class="size-4 stroke-2" />
											</Button>
										{/snippet}
									</DropdownMenu.Trigger>
									<DropdownMenu.Content align="end" class="min-w-48 bg-surface border-subtle">
										{#if isFolder}
											<DropdownMenu.Item class="gap-2" onclick={() => onOpen(entry)}>
												<FolderOpen class="size-4 stroke-2" />
												Buka folder
											</DropdownMenu.Item>
										{:else}
											<DropdownMenu.Item class="gap-2" onclick={() => onOpen(entry)}>
												<Eye class="size-4 stroke-2" />
												Pratinjau
											</DropdownMenu.Item>
										{/if}
										<DropdownMenu.Item class="gap-2" onclick={() => onDownload(entry)}>
											<Download class="size-4 stroke-2" />
											Unduh
										</DropdownMenu.Item>
										<DropdownMenu.Item
											class="gap-2"
											onclick={() => {
												actionEntry = entry;
												detailsOpen = true;
											}}
										>
											<Info class="size-4 stroke-2" />
											Detail berbagi
										</DropdownMenu.Item>
										{#if canEditItem}
											<DropdownMenu.Separator />
											<DropdownMenu.Item class="gap-2" onclick={() => openRenameDialog(entry)}>
												<Pencil class="size-4 stroke-2" />
												Ubah nama
											</DropdownMenu.Item>
											<DropdownMenu.Item
												variant="destructive"
												class="gap-2"
												onclick={() => openDeleteDialog(entry)}
											>
												<Trash2 class="size-4 stroke-2" />
												Hapus
											</DropdownMenu.Item>
										{/if}
									</DropdownMenu.Content>
								</DropdownMenu.Root>
							{/snippet}
						</FileGridCard>
					</ContextMenu.Trigger>
					<ContextMenu.Content class="min-w-48 bg-surface border-subtle">
						{#if isFolder}
							<ContextMenu.Item class="gap-2" onclick={() => onOpen(entry)}>
								<FolderOpen class="size-4 stroke-2" />
								Buka folder
							</ContextMenu.Item>
						{:else}
							<ContextMenu.Item class="gap-2" onclick={() => onOpen(entry)}>
								<Eye class="size-4 stroke-2" />
								Pratinjau
							</ContextMenu.Item>
						{/if}
						<ContextMenu.Item class="gap-2" onclick={() => onDownload(entry)}>
							<Download class="size-4 stroke-2" />
							Unduh
						</ContextMenu.Item>
						<ContextMenu.Item
							class="gap-2"
							onclick={() => {
								actionEntry = entry;
								detailsOpen = true;
							}}
						>
							<Info class="size-4 stroke-2" />
							Detail berbagi
						</ContextMenu.Item>
						{#if canEditItem}
							<ContextMenu.Separator />
							<ContextMenu.Item class="gap-2" onclick={() => openRenameDialog(entry)}>
								<Pencil class="size-4 stroke-2" />
								Ubah nama
							</ContextMenu.Item>
							<ContextMenu.Item
								variant="destructive"
								class="gap-2"
								onclick={() => openDeleteDialog(entry)}
							>
								<Trash2 class="size-4 stroke-2" />
								Hapus
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
			aria-label="Berkas dibagikan"
		>
			{#each items as entry (entry.id)}
				<SharedFileRow
					{entry}
					selected={selectedId === entry.id}
					canEdit={canEditInFolder}
					onSelect={(e) => (selectedId = e.id)}
					onOpen={onOpen}
					onDownload={onDownload}
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
	onDownload={(e) => onDownload(e as SharedFileItem)}
	onClose={() => syncPreview(null)}
/>

<!-- Create Folder Dialog -->
<Dialog.Root bind:open={isCreateFolderOpen}>
	<Dialog.Content class="bg-surface text-main border-subtle">
		<Dialog.Header>
			<Dialog.Title>Folder Baru</Dialog.Title>
			<Dialog.Description>Buat folder baru di dalam folder bersama ini.</Dialog.Description>
		</Dialog.Header>
		<div class="grid gap-2 py-2">
			<Label for="shared-folder-name">Nama Folder</Label>
			<Input
				id="shared-folder-name"
				bind:value={newFolderName}
				placeholder="Folder baru"
				autocomplete="off"
				onkeydown={(e) => e.key === 'Enter' && handleCreateFolder()}
			/>
		</div>
		<Dialog.Footer>
			<Button variant="secondary" onclick={() => (isCreateFolderOpen = false)}>Batal</Button>
			<Button onclick={handleCreateFolder}>Buat</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

<!-- Rename Dialog -->
<Dialog.Root bind:open={isRenameOpen}>
	<Dialog.Content class="bg-surface text-main border-subtle">
		<Dialog.Header>
			<Dialog.Title>Ubah Nama</Dialog.Title>
			<Dialog.Description>Masukkan nama baru untuk berkas atau folder ini.</Dialog.Description>
		</Dialog.Header>
		<div class="grid gap-2 py-2">
			<Label for="shared-rename-name">Nama Baru</Label>
			<Input
				id="shared-rename-name"
				bind:value={renameNewName}
				autocomplete="off"
				onkeydown={(e) => e.key === 'Enter' && handleRename()}
			/>
		</div>
		<Dialog.Footer>
			<Button variant="secondary" onclick={() => (isRenameOpen = false)}>Batal</Button>
			<Button onclick={handleRename}>Simpan</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

<!-- Delete Dialog -->
<Dialog.Root bind:open={isDeleteConfirmOpen}>
	<Dialog.Content class="bg-surface text-main border-subtle">
		<Dialog.Header>
			<Dialog.Title>Hapus Berkas / Folder</Dialog.Title>
			<Dialog.Description>
				Apakah Anda yakin ingin menghapus “{deleteTarget?.name}”? Tindakan ini tidak dapat dibatalkan.
			</Dialog.Description>
		</Dialog.Header>
		<Dialog.Footer>
			<Button variant="secondary" onclick={() => (isDeleteConfirmOpen = false)}>Batal</Button>
			<Button variant="destructive" onclick={handleDelete}>Hapus</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
