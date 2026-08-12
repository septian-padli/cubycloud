<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { untrack } from 'svelte';
	import { toast } from 'svelte-sonner';
	import type { SharedFileItem } from '$lib/types/drive.js';
	import { formatSize } from '$lib/mocks/file-tree.js';
	import { driveView } from '$lib/drive-view.svelte.js';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu/index.js';
	import * as ContextMenu from '$lib/components/ui/context-menu/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Download, Eye, Info, MoreVertical, Pencil, Share2, Trash2 } from '@lucide/svelte';
	import SharedFileRow from './shared-file-row.svelte';
	import ShareDetailsDialog from './share-details-dialog.svelte';
	import ShareSheet from './explorer/share-sheet.svelte';
	import FilePreviewDialog from './explorer/file-preview-dialog.svelte';
	import DriveFileGrid from './drive-file-grid.svelte';
	import FileGridCard from './file-grid-card.svelte';
	import PermissionBadge from './permission-badge.svelte';

	let {
		initialItems
	}: {
		initialItems: SharedFileItem[];
	} = $props();

	let items = $state<SharedFileItem[]>(untrack(() => initialItems.map((e) => ({ ...e }))));
	let selectedId = $state<string | null>(null);
	let detailsOpen = $state(false);
	let shareOpen = $state(false);
	let actionEntry = $state<SharedFileItem | null>(null);

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

	function onDownload(entry: SharedFileItem) {
		toast.info(`Mengunduh “${entry.name}” (mock)`, {
			description: formatSize(entry.sizeBytes)
		});
	}

	function onEdit(entry: SharedFileItem) {
		if (entry.permission !== 'editor') {
			toast.error('Anda hanya memiliki izin Viewer.');
			return;
		}
		toast.success(`Mode edit “${entry.name}” (mock)`);
	}

	function onDelete(entry: SharedFileItem) {
		if (entry.permission !== 'editor') {
			toast.error('Anda hanya memiliki izin Viewer.');
			return;
		}
		items = items.filter((e) => e.id !== entry.id);
		if (selectedId === entry.id) selectedId = null;
		if (previewId === entry.id) syncPreview(null);
		toast.success(`“${entry.name}” dihapus (mock)`);
	}
</script>

{#if items.length === 0}
	<div class="overflow-hidden rounded-xl border border-subtle bg-surface px-4 py-12 text-center">
		<p class="text-sm font-medium text-main">Belum ada berkas dibagikan</p>
		<p class="mt-1 text-xs text-muted">Berkas yang dibagikan ke Anda akan muncul di sini.</p>
	</div>
{:else if driveView.mode === 'grid'}
	<DriveFileGrid label="Berkas dibagikan">
		{#each items as entry (entry.id)}
			{@const isEditor = entry.permission === 'editor'}
			<ContextMenu.Root>
				<ContextMenu.Trigger class="block w-full">
					<FileGridCard
						kind={entry.kind}
						name={entry.name}
						subtitle="{formatSize(entry.sizeBytes)} · Dari {entry.ownerName}"
						selected={selectedId === entry.id}
						onSelect={() => (selectedId = entry.id)}
						onOpen={() => syncPreview(entry.id)}
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
								<DropdownMenu.Content align="end" class="min-w-48">
									<DropdownMenu.Item class="gap-2" onclick={() => syncPreview(entry.id)}>
										<Eye class="size-4 stroke-2" />
										Pratinjau
									</DropdownMenu.Item>
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
									{#if isEditor}
										<DropdownMenu.Separator />
										<DropdownMenu.Item
											class="gap-2"
											onclick={() => {
												actionEntry = entry;
												shareOpen = true;
											}}
										>
											<Share2 class="size-4 stroke-2" />
											Bagikan
										</DropdownMenu.Item>
										<DropdownMenu.Item class="gap-2" onclick={() => onEdit(entry)}>
											<Pencil class="size-4 stroke-2" />
											Edit
										</DropdownMenu.Item>
										<DropdownMenu.Item
											variant="destructive"
											class="gap-2"
											onclick={() => onDelete(entry)}
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
				<ContextMenu.Content class="min-w-48">
					<ContextMenu.Item class="gap-2" onclick={() => syncPreview(entry.id)}>
						<Eye class="size-4 stroke-2" />
						Pratinjau
					</ContextMenu.Item>
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
					{#if isEditor}
						<ContextMenu.Separator />
						<ContextMenu.Item
							class="gap-2"
							onclick={() => {
								actionEntry = entry;
								shareOpen = true;
							}}
						>
							<Share2 class="size-4 stroke-2" />
							Bagikan
						</ContextMenu.Item>
						<ContextMenu.Item class="gap-2" onclick={() => onEdit(entry)}>
							<Pencil class="size-4 stroke-2" />
							Edit
						</ContextMenu.Item>
						<ContextMenu.Item
							variant="destructive"
							class="gap-2"
							onclick={() => onDelete(entry)}
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
				onSelect={(e) => (selectedId = e.id)}
				onPreview={(e) => syncPreview(e.id)}
				onDownload={onDownload}
				onDetails={(e) => {
					actionEntry = e;
					detailsOpen = true;
				}}
				onShare={(e) => {
					actionEntry = e;
					shareOpen = true;
				}}
				onEdit={onEdit}
				onDelete={onDelete}
			/>
		{/each}
	</div>
{/if}

<ShareDetailsDialog bind:open={detailsOpen} entry={actionEntry} />
<ShareSheet bind:open={shareOpen} entry={actionEntry} />
<FilePreviewDialog
	open={previewOpen}
	entry={previewEntry}
	onDownload={(e) => onDownload(e as SharedFileItem)}
	onClose={() => syncPreview(null)}
/>
