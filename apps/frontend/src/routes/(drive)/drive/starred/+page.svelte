<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { toast } from 'svelte-sonner';
	import DrivePageHeader from '$lib/components/drive/drive-page-header.svelte';
	import DriveViewToggle from '$lib/components/drive/drive-view-toggle.svelte';
	import DriveFileGrid from '$lib/components/drive/drive-file-grid.svelte';
	import FileGridCard from '$lib/components/drive/file-grid-card.svelte';
	import FileKindIcon from '$lib/components/drive/explorer/file-kind-icon.svelte';
	import { formatSize } from '$lib/mocks/file-tree.js';
	import { previewUrl } from '$lib/api/preview.js';
	import FilePreviewDialog from '$lib/components/drive/explorer/file-preview-dialog.svelte';
	import { driveView } from '$lib/drive-view.svelte.js';
	import { getStarredItems, toggleStar, myStorageAPI } from '$lib/api/my-storage.js';
	import type { FileEntry } from '$lib/types/drive.js';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu/index.js';
	import * as ContextMenu from '$lib/components/ui/context-menu/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { ConfirmDialog } from '$lib/components/ui/confirm-dialog/index.js';
	import { Download, Eye, FolderOpen, Loader2, MoreVertical, Star, Trash2 } from '@lucide/svelte';
	import { cn } from '$lib/utils.js';

	let items = $state<FileEntry[]>([]);
	let loading = $state(true);
	let selectedId = $state<string | null>(null);

	let deleteConfirmOpen = $state(false);
	let deleteTargetEntry = $state<FileEntry | null>(null);

	const previewId = $derived(page.url.searchParams.get('preview'));
	const previewEntry = $derived(
		previewId ? (items.find((e) => e.id === previewId) ?? null) : null
	);
	const previewOpen = $derived(!!previewEntry);

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

	async function loadStarred() {
		try {
			loading = true;
			const nodes = await getStarredItems();
			items = nodes.map((node) => myStorageAPI.toEntry(node));
		} catch (error) {
			toast.error(error instanceof Error ? error.message : 'Gagal memuat berkas berbintang');
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		void loadStarred();
	});

	function handleItemClick(entry: FileEntry) {
		selectedId = entry.id;
		if (entry.kind === 'folder') {
			void goto(`/drive/my-storage?folder=${encodeURIComponent(entry.id)}`);
		}
	}

	function handleItemDblClick(entry: FileEntry) {
		if (entry.kind === 'folder') {
			void goto(`/drive/my-storage?folder=${encodeURIComponent(entry.id)}`);
		} else {
			syncPreview(entry.id);
		}
	}

	async function onUnstar(entry: FileEntry) {
		// Remove immediately from starred list
		items = items.filter((e) => e.id !== entry.id);
		if (selectedId === entry.id) selectedId = null;
		if (previewId === entry.id) syncPreview(null);
		try {
			await toggleStar(entry.id);
			toast.success(`Bintang dihapus dari “${entry.name}”`);
		} catch (error) {
			toast.error(error instanceof Error ? error.message : 'Gagal menghapus bintang');
			void loadStarred();
		}
	}

	async function onDownload(entry: FileEntry) {
		try {
			await myStorageAPI.download(entry.id, entry.name);
		} catch (error) {
			toast.error(error instanceof Error ? error.message : 'Gagal mengunduh berkas');
		}
	}

	function onDelete(entry: FileEntry) {
		deleteTargetEntry = entry;
		deleteConfirmOpen = true;
	}

	async function executeDelete() {
		if (!deleteTargetEntry) return;
		const entry = deleteTargetEntry;
		try {
			await myStorageAPI.remove(entry.id);
			items = items.filter((e) => e.id !== entry.id);
			if (selectedId === entry.id) selectedId = null;
			if (previewId === entry.id) syncPreview(null);
			toast.success(`“${entry.name}” dipindahkan ke Trash`);
		} catch (error) {
			toast.error(error instanceof Error ? error.message : 'Gagal memindahkan ke Trash');
		}
	}
</script>

<svelte:head>
	<title>Berbintang — CubyCloud</title>
	<meta
		name="description"
		content="Akses cepat ke berkas dan folder yang Anda tandai dengan bintang."
	/>
</svelte:head>

<div class="flex flex-col gap-6">
	<DrivePageHeader
		title="Berbintang"
		description="Akses cepat ke berkas dan folder yang Anda tandai dengan bintang."
	>
		{#snippet actions()}
			<DriveViewToggle />
		{/snippet}
	</DrivePageHeader>

	{#if loading}
		<div
			class="flex items-center justify-center rounded-xl border border-subtle bg-surface px-4 py-16 text-muted"
		>
			<Loader2 class="size-6 animate-spin" />
			<span class="ms-2 text-sm font-medium">Memuat berkas berbintang...</span>
		</div>
	{:else if items.length === 0}
		<div
			class="overflow-hidden rounded-xl border border-subtle bg-surface px-4 py-12 text-center"
		>
			<p class="text-sm font-medium text-main">Belum ada berkas berbintang</p>
			<p class="mt-1 text-xs text-muted">
				Tandai berkas atau folder dengan bintang dari My Storage.
			</p>
		</div>
	{:else if driveView.mode === 'grid'}
		<DriveFileGrid label="Berkas berbintang">
			{#each items as entry (entry.id)}
				<ContextMenu.Root>
					<ContextMenu.Trigger class="block w-full">
						<FileGridCard
							kind={entry.kind}
							name={entry.name}
							subtitle={entry.kind === 'folder'
								? `Folder · ${entry.modifiedAt}`
								: `${formatSize(entry.sizeBytes)} · ${entry.modifiedAt}`}
							previewUrl={entry.kind !== 'folder' ? previewUrl('my-storage', entry.id) : undefined}
							selected={selectedId === entry.id}
							onSelect={() => handleItemClick(entry)}
							onOpen={() => handleItemDblClick(entry)}
						>
							{#snippet star()}
								<button
									type="button"
									class="inline-flex size-7 items-center justify-center rounded-lg bg-surface/80 p-1 backdrop-blur-sm transition-colors hover:bg-surface focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-primary"
									title="Hapus Bintang"
									aria-label="Hapus Bintang"
									onclick={(e) => {
										e.stopPropagation();
										onUnstar(entry);
									}}
								>
									<Star class="size-4 fill-amber-400 stroke-amber-500 text-amber-500 transition-colors" />
								</button>
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
									<DropdownMenu.Content align="end" class="min-w-48">
										{#if entry.kind === 'folder'}
											<DropdownMenu.Item
												class="gap-2"
												onclick={() => goto(`/drive/my-storage?folder=${encodeURIComponent(entry.id)}`)}
											>
												<FolderOpen class="size-4 stroke-2" />
												Buka Folder
											</DropdownMenu.Item>
										{:else}
											<DropdownMenu.Item class="gap-2" onclick={() => syncPreview(entry.id)}>
												<Eye class="size-4 stroke-2" />
												Lihat Pratinjau
											</DropdownMenu.Item>
											<DropdownMenu.Item class="gap-2" onclick={() => onDownload(entry)}>
												<Download class="size-4 stroke-2" />
												Unduh
											</DropdownMenu.Item>
										{/if}
										<DropdownMenu.Item class="gap-2" onclick={() => onUnstar(entry)}>
											<Star class="size-4 fill-amber-400 stroke-amber-500 text-amber-500" />
											Hapus Bintang
										</DropdownMenu.Item>
										<DropdownMenu.Separator />
										<DropdownMenu.Item
											variant="destructive"
											class="gap-2"
											onclick={() => onDelete(entry)}
										>
											<Trash2 class="size-4 stroke-2" />
											Hapus ke Trash
										</DropdownMenu.Item>
									</DropdownMenu.Content>
								</DropdownMenu.Root>
							{/snippet}
						</FileGridCard>
					</ContextMenu.Trigger>
					<ContextMenu.Content class="min-w-48">
						{#if entry.kind === 'folder'}
							<ContextMenu.Item
								class="gap-2"
								onclick={() => goto(`/drive/my-storage?folder=${encodeURIComponent(entry.id)}`)}
							>
								<FolderOpen class="size-4 stroke-2" />
								Buka Folder
							</ContextMenu.Item>
						{:else}
							<ContextMenu.Item class="gap-2" onclick={() => syncPreview(entry.id)}>
								<Eye class="size-4 stroke-2" />
								Lihat Pratinjau
							</ContextMenu.Item>
							<ContextMenu.Item class="gap-2" onclick={() => onDownload(entry)}>
								<Download class="size-4 stroke-2" />
								Unduh
							</ContextMenu.Item>
						{/if}
						<ContextMenu.Item class="gap-2" onclick={() => onUnstar(entry)}>
							<Star class="size-4 fill-amber-400 stroke-amber-500 text-amber-500" />
							Hapus Bintang
						</ContextMenu.Item>
						<ContextMenu.Separator />
						<ContextMenu.Item
							variant="destructive"
							class="gap-2"
							onclick={() => onDelete(entry)}
						>
							<Trash2 class="size-4 stroke-2" />
							Hapus ke Trash
						</ContextMenu.Item>
					</ContextMenu.Content>
				</ContextMenu.Root>
			{/each}
		</DriveFileGrid>
	{:else}
		<div
			class="overflow-hidden rounded-xl border border-subtle bg-surface"
			role="listbox"
			aria-label="Berkas berbintang"
		>
			{#each items as entry (entry.id)}
				<ContextMenu.Root>
					<ContextMenu.Trigger class="block w-full">
						<div
							role="option"
							tabindex="0"
							aria-selected={selectedId === entry.id}
							class={cn(
								'flex cursor-pointer items-center justify-between border-b border-subtle px-4 py-3 transition-colors hover:bg-black/5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/50 dark:hover:bg-white/5',
								selectedId === entry.id && 'bg-primary/5'
							)}
							onclick={() => handleItemClick(entry)}
							ondblclick={() => handleItemDblClick(entry)}
							onkeydown={(e) => {
								if (e.key === 'Enter') handleItemDblClick(entry);
							}}
						>
							<div class="flex min-w-0 items-center gap-3">
								<FileKindIcon kind={entry.kind} />
								<div class="min-w-0">
									<div class="flex items-center gap-1.5">
										<p class="truncate text-sm font-medium text-main">
											{entry.name}
										</p>
										<button
											type="button"
											class="inline-flex size-6 shrink-0 items-center justify-center rounded p-0.5 text-muted hover:text-amber-500 focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-primary"
											title="Hapus Bintang"
											aria-label="Hapus Bintang"
											onclick={(e) => {
												e.stopPropagation();
												onUnstar(entry);
											}}
										>
											<Star class="size-3.5 fill-amber-400 stroke-amber-500 text-amber-500 transition-colors" />
										</button>
									</div>
									<p class="text-xs text-muted">
										{#if entry.kind === 'folder'}
											Folder · Diubah {entry.modifiedAt}
										{:else}
											{formatSize(entry.sizeBytes)} · Diubah {entry.modifiedAt}
										{/if}
									</p>
								</div>
							</div>

							<div class="flex shrink-0 items-center gap-2">
								<DropdownMenu.Root>
									<DropdownMenu.Trigger>
										{#snippet child({ props })}
											<Button
												{...props}
												type="button"
												variant="ghost"
												size="icon"
												class="size-8 text-muted hover:bg-primary hover:text-white aria-expanded:bg-primary aria-expanded:text-white"
												aria-label="Aksi berkas"
												onclick={(e: MouseEvent) => e.stopPropagation()}
											>
												<MoreVertical class="size-4 stroke-2" />
											</Button>
										{/snippet}
									</DropdownMenu.Trigger>
									<DropdownMenu.Content align="end" class="min-w-48">
										{#if entry.kind === 'folder'}
											<DropdownMenu.Item
												class="gap-2"
												onclick={() => goto(`/drive/my-storage?folder=${encodeURIComponent(entry.id)}`)}
											>
												<FolderOpen class="size-4 stroke-2" />
												Buka Folder
											</DropdownMenu.Item>
										{:else}
											<DropdownMenu.Item class="gap-2" onclick={() => syncPreview(entry.id)}>
												<Eye class="size-4 stroke-2" />
												Lihat Pratinjau
											</DropdownMenu.Item>
											<DropdownMenu.Item class="gap-2" onclick={() => onDownload(entry)}>
												<Download class="size-4 stroke-2" />
												Unduh
											</DropdownMenu.Item>
										{/if}
										<DropdownMenu.Item class="gap-2" onclick={() => onUnstar(entry)}>
											<Star class="size-4 fill-amber-400 stroke-amber-500 text-amber-500" />
											Hapus Bintang
										</DropdownMenu.Item>
										<DropdownMenu.Separator />
										<DropdownMenu.Item
											variant="destructive"
											class="gap-2"
											onclick={() => onDelete(entry)}
										>
											<Trash2 class="size-4 stroke-2" />
											Hapus ke Trash
										</DropdownMenu.Item>
									</DropdownMenu.Content>
								</DropdownMenu.Root>
							</div>
						</div>
					</ContextMenu.Trigger>
					<ContextMenu.Content class="min-w-48">
						{#if entry.kind === 'folder'}
							<ContextMenu.Item
								class="gap-2"
								onclick={() => goto(`/drive/my-storage?folder=${encodeURIComponent(entry.id)}`)}
							>
								<FolderOpen class="size-4 stroke-2" />
								Buka Folder
							</ContextMenu.Item>
						{:else}
							<ContextMenu.Item class="gap-2" onclick={() => syncPreview(entry.id)}>
								<Eye class="size-4 stroke-2" />
								Lihat Pratinjau
							</ContextMenu.Item>
							<ContextMenu.Item class="gap-2" onclick={() => onDownload(entry)}>
								<Download class="size-4 stroke-2" />
								Unduh
							</ContextMenu.Item>
						{/if}
						<ContextMenu.Item class="gap-2" onclick={() => onUnstar(entry)}>
							<Star class="size-4 fill-amber-400 stroke-amber-500 text-amber-500" />
							Hapus Bintang
						</ContextMenu.Item>
						<ContextMenu.Separator />
						<ContextMenu.Item
							variant="destructive"
							class="gap-2"
							onclick={() => onDelete(entry)}
						>
							<Trash2 class="size-4 stroke-2" />
							Hapus ke Trash
						</ContextMenu.Item>
					</ContextMenu.Content>
				</ContextMenu.Root>
			{/each}
		</div>
	{/if}
</div>

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

<FilePreviewDialog
	open={previewOpen}
	entry={previewEntry}
	source="my-storage"
	onDownload={onDownload}
	onClose={() => syncPreview(null)}
/>
