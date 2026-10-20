<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import type { FileEntry } from '$lib/types/drive.js';
	import { formatSize } from '$lib/mocks/file-tree.js';
	import { driveView } from '$lib/drive-view.svelte.js';
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
	import {
		Archive,
		Download,
		FolderPlus,
		Info,
		Loader2,
		Upload
	} from '@lucide/svelte';
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
				toast.error(e.message || 'Gagal memuat tautan berbagi');
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
				toast.success('Tautan berhasil dibuka');
				return true;
			}
			return false;
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Kata sandi tidak sesuai');
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
				toast.error(`Gagal mengunggah ${files[i].name}: ${err instanceof Error ? err.message : ''}`);
			}
		}
		uploading = false;
		if (successCount > 0) {
			toast.success(`${successCount} berkas berhasil diunggah`);
			void fetchShare(currentFolderId);
		}
		if (fileInput) fileInput.value = '';
	}

	async function handleCreateFolder() {
		if (!newFolderName.trim()) return;
		try {
			await createPublicFolder(data.token, newFolderName.trim(), currentFolderId);
			toast.success(`Folder “${newFolderName.trim()}” dibuat`);
			isCreateFolderOpen = false;
			newFolderName = '';
			void fetchShare(currentFolderId);
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Gagal membuat folder');
		}
	}

	async function handleRename() {
		if (!renameTarget || !renameNewName.trim()) return;
		try {
			await renamePublicItem(data.token, renameTarget.id, renameNewName.trim());
			toast.success('Nama berhasil diubah');
			isRenameOpen = false;
			renameTarget = null;
			renameNewName = '';
			void fetchShare(currentFolderId);
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Gagal mengubah nama');
		}
	}

	async function handleDelete() {
		if (!deleteTarget) return;
		try {
			await deletePublicItem(data.token, deleteTarget.id);
			toast.success(`“${deleteTarget.name}” berhasil dihapus`);
			isDeleteConfirmOpen = false;
			deleteTarget = null;
			void fetchShare(currentFolderId);
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Gagal menghapus');
		}
	}

	const isEditor = $derived(shareData?.permission === 'editor');
	const canEdit = $derived(isEditor && loggedIn);
	const needsPassword = $derived(
		!loading && shareData?.has_password && !shareData?.unlocked
	);
</script>

<svelte:head>
	<title>
		{rootEntry ? `${rootEntry.name} — Dibagikan` : 'Tautan Berbagi'} — CubyCloud
	</title>
	<meta name="robots" content="noindex" />
</svelte:head>

<ShareAppShell {loggedIn}>
	{#if loading}
		<div class="flex flex-1 items-center justify-center py-16 text-muted">
			<Loader2 class="size-6 animate-spin text-primary" />
			<span class="ms-2 text-sm font-medium">Memuat tautan...</span>
		</div>
	{:else if notFound}
		<ShareStatusView
			title="Tautan tidak ditemukan"
			description="Tautan ini tidak valid atau sudah dihapus oleh pemiliknya."
		/>
	{:else if expired}
		<ShareStatusView
			title="Tautan kedaluwarsa"
			description="Tautan berbagi ini sudah melewati tanggal batas waktu."
		/>
	{:else if needsPassword}
		<SharePasswordGate {onUnlock} />
	{:else if rootEntry}
		<div class="flex flex-col gap-6">
			<DrivePageHeader
				title={rootEntry.name}
				description={rootEntry.kind === 'folder'
					? 'Folder dibagikan secara publik melalui CubyCloud.'
					: 'Berkas dibagikan secara publik melalui CubyCloud.'}
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
				<div class="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-primary/20 bg-primary/5 p-4">
					<div class="flex items-center gap-3">
						<Info class="size-5 shrink-0 text-primary stroke-2" />
						<div>
							<p class="text-sm font-semibold text-main">Tautan berizin Ubah (Editor)</p>
							<p class="text-xs text-muted">
								Masuk ke akun CubyCloud Anda untuk mengunggah berkas, membuat folder, atau mengedit isi folder ini.
							</p>
						</div>
					</div>
					<Button href={loginHref} size="sm">Masuk untuk mengedit</Button>
				</div>
			{/if}

			{#if rootEntry.kind === 'folder'}
				<!-- Folder Toolbar & Breadcrumbs -->
				<div class="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-subtle bg-surface px-4 py-3">
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
							variant="outline"
							size="sm"
							class="gap-1.5"
							onclick={onDownloadZip}
						>
							<Archive class="size-4 stroke-2" />
							Unduh ZIP
						</Button>
					</div>
				</div>

				<ShareGuestExplorer
					items={folderItems}
					{onOpen}
					{canEdit}
					onDownload={onDownloadItem}
					onRename={(entry) => {
						renameTarget = entry;
						renameNewName = entry.name;
						isRenameOpen = true;
					}}
					onDelete={(entry) => {
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
								{formatSize(rootEntry.sizeBytes)} · Diperbarui {rootEntry.modifiedAt}
							</p>
						</div>
						<div class="flex items-center gap-3">
							<Button
								type="button"
								variant="secondary"
								onclick={() => rootEntry && syncPreview(rootEntry.id)}
							>
								Pratinjau
							</Button>
							<Button
								type="button"
								class="gap-2"
								onclick={() => rootEntry && onDownloadItem(rootEntry)}
							>
								<Download class="size-4 stroke-2" />
								Unduh berkas
							</Button>
						</div>
					</Card.Content>
				</Card.Root>
			{/if}
		</div>
	{:else}
		<ShareStatusView
			title="Tidak dapat memuat berkas"
			description="Terjadi kendala saat memuat berkas dari tautan ini."
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
	onDownload={(e) => onDownloadItem(e as FileEntry)}
	onClose={() => syncPreview(null)}
/>

<!-- Create Folder Dialog -->
<Dialog.Root bind:open={isCreateFolderOpen}>
	<Dialog.Content class="bg-surface text-main border-subtle">
		<Dialog.Header>
			<Dialog.Title>Folder Baru</Dialog.Title>
			<Dialog.Description>Buat folder baru di dalam tautan ini.</Dialog.Description>
		</Dialog.Header>
		<div class="grid gap-2 py-2">
			<Label for="public-new-folder-name">Nama Folder</Label>
			<Input
				id="public-new-folder-name"
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
			<Label for="public-rename-name">Nama Baru</Label>
			<Input
				id="public-rename-name"
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
