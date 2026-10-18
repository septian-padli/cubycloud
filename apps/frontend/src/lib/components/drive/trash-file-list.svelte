<script lang="ts">
	import { onMount, untrack } from 'svelte';
	import { toast } from 'svelte-sonner';
	import type { TrashFileItem } from '$lib/types/drive.js';
	import { formatSize } from '$lib/mocks/file-tree.js';
	import { driveView } from '$lib/drive-view.svelte.js';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu/index.js';
	import * as ContextMenu from '$lib/components/ui/context-menu/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Loader2, MoreVertical, RotateCcw, Trash2 } from '@lucide/svelte';
	import TrashFileRow from './trash-file-row.svelte';
	import MissingParentRestoreDialog from './missing-parent-restore-dialog.svelte';
	import PurgeTrashDialog from './purge-trash-dialog.svelte';
	import ConfirmDialog from '$lib/components/ui/confirm-dialog/confirm-dialog.svelte';
	import DriveFileGrid from './drive-file-grid.svelte';
	import FileGridCard from './file-grid-card.svelte';
	import DriveViewToggle from './drive-view-toggle.svelte';
	import { trashAPI } from '$lib/api/trash.js';

	let {
		initialItems
	}: {
		initialItems?: TrashFileItem[];
	} = $props();

	let items = $state<TrashFileItem[]>(
		untrack(() => (initialItems ? initialItems.map((e) => ({ ...e })) : []))
	);
	let loading = $state(untrack(() => !initialItems));
	let selectedId = $state<string | null>(null);
	let purgeOpen = $state(false);
	let missingParentOpen = $state(false);
	let singlePurgeOpen = $state(false);
	let actionEntry = $state<TrashFileItem | null>(null);
	let purgeOneTarget = $state<TrashFileItem | null>(null);
	let actionLoading = $state(false);

	async function loadTrash() {
		try {
			loading = true;
			items = await trashAPI.list();
		} catch (err: unknown) {
			const msg = err instanceof Error ? err.message : 'Gagal memuat Trash';
			toast.error(msg);
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		if (!initialItems) {
			loadTrash();
		}
	});

	function openPurge() {
		if (items.length === 0) {
			toast.info('Trash sudah kosong.');
			return;
		}
		purgeOpen = true;
	}

	function removeItem(id: string) {
		items = items.filter((e) => e.id !== id);
		if (selectedId === id) selectedId = null;
	}

	async function restoreEntry(entry: TrashFileItem, toRoot = false) {
		try {
			actionLoading = true;
			const res = await trashAPI.restore(entry.id, toRoot);
			removeItem(entry.id);
			if (res.restored_to_root) {
				toast.success(`“${entry.name}” dipulihkan ke Root My Storage`);
			} else {
				toast.success(`“${entry.name}” berhasil dipulihkan`);
			}
		} catch (err: unknown) {
			const msg = err instanceof Error ? err.message : 'Gagal memulihkan item';
			toast.error(msg);
		} finally {
			actionLoading = false;
		}
	}

	function onRestore(entry: TrashFileItem) {
		if (entry.originalParentMissing) {
			actionEntry = entry;
			missingParentOpen = true;
			return;
		}
		restoreEntry(entry, false);
	}

	function onPurgeOne(entry: TrashFileItem) {
		purgeOneTarget = entry;
		singlePurgeOpen = true;
	}

	async function confirmPurgeOne() {
		if (!purgeOneTarget) return;
		const target = purgeOneTarget;
		try {
			actionLoading = true;
			await trashAPI.purgeOne(target.id);
			removeItem(target.id);
			toast.success(`“${target.name}” dihapus permanen`);
		} catch (err: unknown) {
			const msg = err instanceof Error ? err.message : 'Gagal menghapus permanen';
			toast.error(msg);
		} finally {
			actionLoading = false;
			singlePurgeOpen = false;
			purgeOneTarget = null;
		}
	}

	async function onPurgeAll() {
		try {
			actionLoading = true;
			await trashAPI.purgeAll();
			items = [];
			selectedId = null;
			toast.success('Semua item di Trash berhasil dihapus permanen');
		} catch (err: unknown) {
			const msg = err instanceof Error ? err.message : 'Gagal mengosongkan Trash';
			toast.error(msg);
		} finally {
			actionLoading = false;
			purgeOpen = false;
		}
	}
</script>

<div class="flex flex-col gap-4">
	<div class="flex flex-wrap items-center justify-end gap-2">
		<Button
			type="button"
			variant="destructive"
			class="gap-1.5"
			data-testid="empty-trash-btn"
			disabled={items.length === 0 || loading || actionLoading}
			onclick={() => openPurge()}
		>
			<Trash2 class="size-4 stroke-2" />
			Kosongkan Sampah
		</Button>
		<DriveViewToggle />
	</div>

	{#if loading}
		<div class="flex items-center justify-center p-12 text-muted">
			<Loader2 class="mr-2 size-5 animate-spin" />
			<span class="text-sm">Memuat Trash...</span>
		</div>
	{:else if items.length === 0}
		<div class="overflow-hidden rounded-xl border border-subtle bg-surface px-4 py-12 text-center">
			<p class="text-sm font-medium text-main">Trash kosong</p>
			<p class="mt-1 text-xs text-muted">
				Berkas yang dihapus akan muncul di sini selama 30 hari.
			</p>
		</div>
	{:else if driveView.mode === 'grid'}
		<DriveFileGrid label="Trash">
			{#each items as entry (entry.id)}
				<ContextMenu.Root>
					<ContextMenu.Trigger class="block w-full">
						<FileGridCard
							kind={entry.kind}
							name={entry.name}
							subtitle={entry.kind === 'folder'
								? `Dihapus ${entry.deletedAt} · sisa ${entry.daysRemaining} hari`
								: `${formatSize(entry.sizeBytes)} · sisa ${entry.daysRemaining} hari`}
							selected={selectedId === entry.id}
							onSelect={() => (selectedId = entry.id)}
						>
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
												aria-label="Aksi trash"
												onclick={(e: MouseEvent) => e.stopPropagation()}
											>
												<MoreVertical class="size-4 stroke-2" />
											</Button>
										{/snippet}
									</DropdownMenu.Trigger>
									<DropdownMenu.Content align="end" class="min-w-48">
										<DropdownMenu.Item class="gap-2" onclick={() => onRestore(entry)}>
											<RotateCcw class="size-4 stroke-2" />
											Pulihkan
										</DropdownMenu.Item>
										<DropdownMenu.Separator />
										<DropdownMenu.Item
											variant="destructive"
											class="gap-2"
											onclick={() => onPurgeOne(entry)}
										>
											<Trash2 class="size-4 stroke-2" />
											Hapus permanen
										</DropdownMenu.Item>
									</DropdownMenu.Content>
								</DropdownMenu.Root>
							{/snippet}
						</FileGridCard>
					</ContextMenu.Trigger>
					<ContextMenu.Content class="min-w-48">
						<ContextMenu.Item class="gap-2" onclick={() => onRestore(entry)}>
							<RotateCcw class="size-4 stroke-2" />
							Pulihkan
						</ContextMenu.Item>
						<ContextMenu.Separator />
						<ContextMenu.Item
							variant="destructive"
							class="gap-2"
							onclick={() => onPurgeOne(entry)}
						>
							<Trash2 class="size-4 stroke-2" />
							Hapus permanen
						</ContextMenu.Item>
					</ContextMenu.Content>
				</ContextMenu.Root>
			{/each}
		</DriveFileGrid>
	{:else}
		<div
			class="overflow-hidden rounded-xl border border-subtle bg-surface"
			role="listbox"
			aria-label="Trash"
		>
			{#each items as entry (entry.id)}
				<TrashFileRow
					{entry}
					selected={selectedId === entry.id}
					onSelect={(e) => (selectedId = e.id)}
					onRestore={onRestore}
					onPurge={onPurgeOne}
				/>
			{/each}
		</div>
	{/if}
</div>

<MissingParentRestoreDialog
	bind:open={missingParentOpen}
	fileName={actionEntry?.name ?? ''}
	onConfirm={() => {
		if (actionEntry) restoreEntry(actionEntry, true);
	}}
/>
<PurgeTrashDialog
	bind:open={purgeOpen}
	itemCount={items.length}
	onConfirm={onPurgeAll}
/>
<ConfirmDialog
	bind:open={singlePurgeOpen}
	title="Hapus Permanen"
	description={`Hapus permanen “${purgeOneTarget?.name ?? ''}”? Tindakan ini tidak dapat dibatalkan.`}
	confirmText="Hapus permanen"
	cancelText="Batal"
	variant="destructive"
	loading={actionLoading}
	onConfirm={confirmPurgeOne}
/>
