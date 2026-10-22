<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import type { FileEntry } from '$lib/types/drive.js';
	import { formatSize } from '$lib/mocks/file-tree.js';
	import { driveView } from '$lib/drive-view.svelte.js';
	import { locale } from '$lib/locale.svelte.js';
	import { authAPI } from '$lib/api/auth.js';
	import {
		getPublicShare,
		unlockPublicShare,
		downloadPublicFile,
		downloadPublicZip,
		uploadPublicFile,
		createPublicFolder,
		renamePublicItem,
		deletePublicItem,
		detectFileKind,
		type PublicShareResponse
	} from '$lib/api/share.js';
	import ShareAppShell from '$lib/components/share/share-app-shell.svelte';
	import ShareStatusView from '$lib/components/share/share-status-view.svelte';
	import SharePasswordGate from '$lib/components/share/share-password-gate.svelte';
	import ShareGuestExplorer from '$lib/components/share/share-guest-explorer.svelte';
	import DrivePageHeader from '$lib/components/drive/drive-page-header.svelte';
	import DriveViewToggle from '$lib/components/drive/drive-view-toggle.svelte';
	import FilePreviewDialog from '$lib/components/drive/explorer/file-preview-dialog.svelte';
	import FileKindIcon from '$lib/components/drive/explorer/file-kind-icon.svelte';
	import * as Card from '$lib/components/ui/card/index.js';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { Archive, Download, FolderPlus, Info, Loader2, Upload } from '@lucide/svelte';
	import PermissionBadge from '$lib/components/drive/permission-badge.svelte';
	import type { PageData } from './$types.js';

	let { data }: { data: PageData } = $props();

	let loading = $state(true);
	let notFound = $state(false);
	let expired = $state(false);
	let loggedIn = $state(false);
	let shareData = $state<PublicShareResponse | null>(null);

	let currentFolderId = $state<string | null>(null);
	let folderStack = $state<{ id: string; name: string }[]>([]);
	let rootEntry = $state<FileEntry | null>(null);
	let folderItems = $state<FileEntry[]>([]);

	let uploading = $state(false);
	let fileInput = $state<HTMLInputElement | null>(null);
	let isCreateFolderOpen = $state(false);
	let newFolderName = $state('');
	let isRenameOpen = $state(false);
	let renameTarget = $state<FileEntry | null>(null);
	let renameNewName = $state('');
	let isDeleteConfirmOpen = $state(false);
	let deleteTarget = $state<FileEntry | null>(null);

	const previewId = $derived(page.url.searchParams.get('preview'));
	const previewEntry = $derived.by(() => {
		if (!previewId) return null;
		if (rootEntry && rootEntry.id === previewId) return rootEntry;
		return folderItems.find((e) => e.id === previewId) ?? null;
	});
	const previewOpen = $derived(!!previewEntry);

	const loginHref = $derived(`/login?redirect=/s/${data.token}`);

	async function checkAuth() {
		try {
			const res = await authAPI.getMe();
			loggedIn = !!res.data && !res.error;
		} catch {
			loggedIn = false;
		}
	}

	async function fetchShare(folderId?: string | null) {
		loading = true;
		try {
			const res = await getPublicShare(data.token, folderId);
			shareData = res;
			if (res.unlocked && res.item) {
				rootEntry = {
					id: res.item.id,
					name: res.item.name,
					kind: detectFileKind(res.item.name, res.item.kind),
					sizeBytes: res.item.size_bytes,
					modifiedAt: res.item.modified_at,
					parentId: res.item.parent_id ?? null
				};
				folderItems = (res.children ?? []).map((ch) => ({
					id: ch.id,
					name: ch.name,
					kind: detectFileKind(ch.name, ch.kind),
					sizeBytes: ch.size_bytes,
					modifiedAt: ch.modified_at,
					parentId: ch.parent_id ?? null
				}));
			}
		} catch (err: unknown) {
			const e = err as { status?: number; message?: string };
			if (e.status === 404) {
				notFound = true;
			} else if (e.status === 410) {
				expired = true;
			} else {
				toast.error(
					e.message ||
						(locale.language === 'id'
							? 'Gagal memuat tautan berbagi'
							: 'Failed to load shared link')
				);
			}
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		driveView.init();
		void checkAuth();
		void fetchShare(null);
	});

	async function onUnlock(pwd: string): Promise<boolean> {
		try {
			const res = await unlockPublicShare(data.token, pwd);
			if (res.success) {
				await fetchShare(currentFolderId);
				toast.success(
					locale.language === 'id' ? 'Tautan berhasil dibuka' : 'Link unlocked successfully'
				);
				return true;
			}
			return false;
		} catch (err) {
			toast.error(
				err instanceof Error
					? err.message
					: locale.language === 'id'
						? 'Kata sandi tidak sesuai'
						: 'Incorrect password'
			);
			return false;
		}
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

	function onOpen(entry: FileEntry) {
		if (entry.kind === 'folder') {
			currentFolderId = entry.id;
			folderStack = [...folderStack, { id: entry.id, name: entry.name }];
			void fetchShare(entry.id);
			return;
		}
		syncPreview(entry.id);
	}

	function navigateBreadcrumb(index: number) {
		if (index === -1) {
			currentFolderId = null;
			folderStack = [];
			void fetchShare(null);
		} else {
			const target = folderStack[index];
			folderStack = folderStack.slice(0, index + 1);
			currentFolderId = target.id;
			void fetchShare(target.id);
		}
	}

	function onDownloadItem(entry: FileEntry) {
		if (entry.kind === 'folder') {
			downloadPublicZip(data.token, entry.id, entry.name);
		} else {
			downloadPublicFile(data.token, entry.id === rootEntry?.id ? null : entry.id, entry.name);
		}
	}

	function onDownloadZip() {
		downloadPublicZip(data.token, currentFolderId, rootEntry?.name || 'archive');
	}

	async function handleUpload(e: Event) {
		const files = (e.target as HTMLInputElement).files;
		if (!files || files.length === 0) return;
		uploading = true;
		let successCount = 0;
		for (let i = 0; i < files.length; i++) {
			try {
				await uploadPublicFile(data.token, files[i], currentFolderId);
				successCount++;
			} catch (err) {
				toast.error(
					`${locale.language === 'id' ? 'Gagal mengunggah' : 'Failed to upload'} ${files[i].name}: ${err instanceof Error ? err.message : ''}`
				);
			}
		}
		uploading = false;
		if (successCount > 0) {
			toast.success(
				`${successCount} ${locale.language === 'id' ? 'berkas berhasil diunggah' : 'file(s) uploaded successfully'}`
			);
			void fetchShare(currentFolderId);
		}
		if (fileInput) fileInput.value = '';
	}

	async function handleCreateFolder() {
		if (!newFolderName.trim()) return;
		try {
			await createPublicFolder(data.token, newFolderName.trim(), currentFolderId);
			toast.success(
				locale.language === 'id'
					? `Folder “${newFolderName.trim()}” dibuat`
					: `Folder “${newFolderName.trim()}” created`
			);
			isCreateFolderOpen = false;
			newFolderName = '';
			void fetchShare(currentFolderId);
		} catch (err) {
			toast.error(
				err instanceof Error
					? err.message
					: locale.language === 'id'
						? 'Gagal membuat folder'
						: 'Failed to create folder'
			);
		}
	}

	async function handleRename() {
		if (!renameTarget || !renameNewName.trim()) return;
		try {
			await renamePublicItem(data.token, renameTarget.id, renameNewName.trim());
			toast.success(
				locale.language === 'id' ? 'Nama berhasil diubah' : 'Name updated successfully'
			);
			isRenameOpen = false;
			renameTarget = null;
			renameNewName = '';
			void fetchShare(currentFolderId);
		} catch (err) {
			toast.error(
				err instanceof Error
					? err.message
					: locale.language === 'id'
						? 'Gagal mengubah nama'
						: 'Failed to rename'
			);
		}
	}

	async function handleDelete() {
		if (!deleteTarget) return;
		try {
			await deletePublicItem(data.token, deleteTarget.id);
			toast.success(
				locale.language === 'id'
					? `“${deleteTarget.name}” berhasil dihapus`
					: `“${deleteTarget.name}” deleted successfully`
			);
			isDeleteConfirmOpen = false;
			deleteTarget = null;
			void fetchShare(currentFolderId);
		} catch (err) {
			toast.error(
				err instanceof Error
					? err.message
					: locale.language === 'id'
						? 'Gagal menghapus'
						: 'Failed to delete'
			);
		}
	}

	const isEditor = $derived(shareData?.permission === 'editor');
	const canEdit = $derived(isEditor && loggedIn);
	const needsPassword = $derived(!loading && shareData?.has_password && !shareData?.unlocked);
</script>

<svelte:head>
	<title>
		{rootEntry
			? `${rootEntry.name} — ${locale.language === 'id' ? 'Dibagikan' : 'Shared'}`
			: locale.language === 'id'
				? 'Tautan Berbagi'
				: 'Shared Link'} — CubyCloud
	</title>
	<meta name="robots" content="noindex" />
</svelte:head>

<ShareAppShell {loggedIn}>
	{#if loading}
		<div class="flex flex-1 items-center justify-center py-16 text-muted">
			<Loader2 class="size-6 animate-spin text-primary" />
			<span class="ms-2 text-sm font-medium"
				>{locale.language === 'id' ? 'Memuat tautan...' : 'Loading link...'}</span
			>
		</div>
	{:else if notFound}
		<ShareStatusView
			title={locale.language === 'id' ? 'Tautan tidak ditemukan' : 'Link not found'}
			description={locale.language === 'id'
				? 'Tautan ini tidak valid atau sudah dihapus oleh pemiliknya.'
				: 'This link is invalid or has been deleted by its owner.'}
		/>
	{:else if expired}
		<ShareStatusView
			title={locale.language === 'id' ? 'Tautan kedaluwarsa' : 'Link expired'}
			description={locale.language === 'id'
				? 'Tautan berbagi ini sudah melewati tanggal batas waktu.'
				: 'This shared link has passed its expiration date.'}
		/>
	{:else if needsPassword}
		<SharePasswordGate {onUnlock} />
	{:else if rootEntry}
		<div class="flex flex-col gap-6">
			<DrivePageHeader
				title={rootEntry.name}
				description={rootEntry.kind === 'folder'
					? locale.language === 'id'
						? 'Folder dibagikan secara publik melalui CubyCloud.'
						: 'This folder is publicly shared through CubyCloud.'
					: locale.language === 'id'
						? 'Berkas dibagikan secara publik melalui CubyCloud.'
						: 'This file is publicly shared through CubyCloud.'}
			>
				{#snippet actions()}
					<div class="flex items-center gap-2">
						{#if shareData?.permission}
							<PermissionBadge permission={shareData.permission} />
						{/if}
						<DriveViewToggle />
					</div>
				{/snippet}
			</DrivePageHeader>

			{#if isEditor && !loggedIn}
				<div
					class="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-primary/20 bg-primary/5 p-4"
				>
					<div class="flex items-center gap-3">
						<Info class="size-5 shrink-0 stroke-2 text-primary" />
						<div>
							<p class="text-sm font-semibold text-main">
								{locale.language === 'id'
									? 'Tautan berizin Ubah (Editor)'
									: 'Link with edit access (Editor)'}
							</p>
							<p class="text-xs text-muted">
								{locale.language === 'id'
									? 'Masuk ke akun CubyCloud Anda untuk mengunggah berkas, membuat folder, atau mengedit isi folder ini.'
									: 'Sign in to your CubyCloud account to upload files, create folders, or edit this folder.'}
							</p>
						</div>
					</div>
					<Button href={loginHref} size="sm">
						{locale.language === 'id' ? 'Masuk untuk mengedit' : 'Sign in to edit'}
					</Button>
				</div>
			{/if}

			{#if rootEntry.kind === 'folder'}
				<!-- Folder Toolbar & Breadcrumbs -->
				<div
					class="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-subtle bg-surface px-4 py-3"
				>
					<div class="flex items-center gap-2 overflow-x-auto text-sm">
						<button
							type="button"
							class="font-medium text-muted transition-colors hover:text-main"
							onclick={() => navigateBreadcrumb(-1)}
						>
							{rootEntry.name}
						</button>
						{#each folderStack as folder, i (folder.id)}
							<span class="text-muted">/</span>
							{#if i === folderStack.length - 1}
								<span class="font-semibold text-main">{folder.name}</span>
							{:else}
								<button
									type="button"
									class="font-medium text-muted transition-colors hover:text-main"
									onclick={() => navigateBreadcrumb(i)}
								>
									{folder.name}
								</button>
							{/if}
						{/each}
					</div>

					<div class="flex items-center gap-2">
						{#if canEdit}
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
								{locale.language === 'id' ? 'Folder baru' : 'New folder'}
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
								{locale.language === 'id' ? 'Unggah' : 'Upload'}
							</Button>
						{/if}
						<Button
							type="button"
							variant="outline"
							size="sm"
							class="gap-1.5"
							onclick={onDownloadZip}
						>
							<Archive class="size-4 stroke-2" />
							{locale.language === 'id' ? 'Unduh ZIP' : 'Download ZIP'}
						</Button>
					</div>
				</div>

				<ShareGuestExplorer
					items={folderItems}
					{onOpen}
					{canEdit}
					onDownload={onDownloadItem}
					onRename={(entry: FileEntry) => {
						renameTarget = entry;
						renameNewName = entry.name;
						isRenameOpen = true;
					}}
					onDelete={(entry: FileEntry) => {
						deleteTarget = entry;
						isDeleteConfirmOpen = true;
					}}
				/>
			{:else}
				<!-- Single File Card -->
				<Card.Root class="rounded-xl border-subtle bg-surface shadow-none">
					<Card.Content class="flex flex-col items-center gap-4 py-12 text-center">
						<div class="flex size-16 items-center justify-center rounded-2xl bg-app">
							<FileKindIcon kind={rootEntry.kind} class="size-8" />
						</div>
						<div>
							<p class="text-lg font-semibold text-main">{rootEntry.name}</p>
							<p class="mt-1 text-sm text-muted">
								{formatSize(rootEntry.sizeBytes)} · {locale.language === 'id'
									? 'Diperbarui'
									: 'Updated'}
								{locale.formatDate(rootEntry.modifiedAt)}
							</p>
						</div>
						<div class="flex items-center gap-3">
							<Button
								type="button"
								variant="secondary"
								onclick={() => rootEntry && syncPreview(rootEntry.id)}
							>
								{locale.language === 'id' ? 'Pratinjau' : 'Preview'}
							</Button>
							<Button
								type="button"
								class="gap-2"
								onclick={() => rootEntry && onDownloadItem(rootEntry)}
							>
								<Download class="size-4 stroke-2" />
								{locale.language === 'id' ? 'Unduh berkas' : 'Download file'}
							</Button>
						</div>
					</Card.Content>
				</Card.Root>
			{/if}
		</div>
	{:else}
		<ShareStatusView
			title={locale.language === 'id' ? 'Tidak dapat memuat berkas' : 'Unable to load file'}
			description={locale.language === 'id'
				? 'Terjadi kendala saat memuat berkas dari tautan ini.'
				: 'There was a problem loading the file from this link.'}
		/>
	{/if}
</ShareAppShell>

<!-- Preview Dialog -->
<FilePreviewDialog
	open={previewOpen}
	entry={previewEntry}
	source="public"
	token={data.token}
	rootFile={previewEntry?.id === rootEntry?.id}
	onDownload={(e: FileEntry) => onDownloadItem(e)}
	onClose={() => syncPreview(null)}
/>

<!-- Create Folder Dialog -->
<Dialog.Root bind:open={isCreateFolderOpen}>
	<Dialog.Content class="border-subtle bg-surface text-main">
		<Dialog.Header>
			<Dialog.Title>{locale.language === 'id' ? 'Folder Baru' : 'New Folder'}</Dialog.Title>
			<Dialog.Description>
				{locale.language === 'id'
					? 'Buat folder baru di dalam tautan ini.'
					: 'Create a new folder in this link.'}
			</Dialog.Description>
		</Dialog.Header>
		<div class="grid gap-2 py-2">
			<Label for="public-new-folder-name"
				>{locale.language === 'id' ? 'Nama Folder' : 'Folder name'}</Label
			>
			<Input
				id="public-new-folder-name"
				bind:value={newFolderName}
				placeholder={locale.language === 'id' ? 'Folder baru' : 'New folder'}
				autocomplete="off"
				onkeydown={(e) => e.key === 'Enter' && handleCreateFolder()}
			/>
		</div>
		<Dialog.Footer>
			<Button variant="secondary" onclick={() => (isCreateFolderOpen = false)}>
				{locale.language === 'id' ? 'Batal' : 'Cancel'}
			</Button>
			<Button onclick={handleCreateFolder}>{locale.language === 'id' ? 'Buat' : 'Create'}</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

<!-- Rename Dialog -->
<Dialog.Root bind:open={isRenameOpen}>
	<Dialog.Content class="border-subtle bg-surface text-main">
		<Dialog.Header>
			<Dialog.Title>{locale.language === 'id' ? 'Ubah Nama' : 'Rename'}</Dialog.Title>
			<Dialog.Description>
				{locale.language === 'id'
					? 'Masukkan nama baru untuk berkas atau folder ini.'
					: 'Enter a new name for this file or folder.'}
			</Dialog.Description>
		</Dialog.Header>
		<div class="grid gap-2 py-2">
			<Label for="public-rename-name">{locale.language === 'id' ? 'Nama Baru' : 'New name'}</Label>
			<Input
				id="public-rename-name"
				bind:value={renameNewName}
				autocomplete="off"
				onkeydown={(e) => e.key === 'Enter' && handleRename()}
			/>
		</div>
		<Dialog.Footer>
			<Button variant="secondary" onclick={() => (isRenameOpen = false)}>
				{locale.language === 'id' ? 'Batal' : 'Cancel'}
			</Button>
			<Button onclick={handleRename}>{locale.language === 'id' ? 'Simpan' : 'Save'}</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

<!-- Delete Dialog -->
<Dialog.Root bind:open={isDeleteConfirmOpen}>
	<Dialog.Content class="border-subtle bg-surface text-main">
		<Dialog.Header>
			<Dialog.Title
				>{locale.language === 'id' ? 'Hapus Berkas / Folder' : 'Delete file / folder'}</Dialog.Title
			>
			<Dialog.Description>
				{locale.language === 'id'
					? `Apakah Anda yakin ingin menghapus “${deleteTarget?.name}”? Tindakan ini tidak dapat dibatalkan.`
					: `Are you sure you want to delete “${deleteTarget?.name}”? This action cannot be undone.`}
			</Dialog.Description>
		</Dialog.Header>
		<Dialog.Footer>
			<Button variant="secondary" onclick={() => (isDeleteConfirmOpen = false)}>
				{locale.language === 'id' ? 'Batal' : 'Cancel'}
			</Button>
			<Button variant="destructive" onclick={handleDelete}>
				{locale.language === 'id' ? 'Hapus' : 'Delete'}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
