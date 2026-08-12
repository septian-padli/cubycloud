<script lang="ts">
	import { untrack } from 'svelte';
	import { toast } from 'svelte-sonner';
	import type { TrashFileItem } from '$lib/types/drive.js';
	import { formatSize } from '$lib/mocks/file-tree.js';
	import { driveView } from '$lib/drive-view.svelte.js';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu/index.js';
	import * as ContextMenu from '$lib/components/ui/context-menu/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { MoreVertical, RotateCcw, Trash2 } from '@lucide/svelte';
	import TrashFileRow from './trash-file-row.svelte';
	import MissingParentRestoreDialog from './missing-parent-restore-dialog.svelte';
	import PurgeTrashDialog from './purge-trash-dialog.svelte';
	import DriveFileGrid from './drive-file-grid.svelte';
	import FileGridCard from './file-grid-card.svelte';
	import DriveViewToggle from './drive-view-toggle.svelte';

	let {
		initialItems
	}: {
		initialItems: TrashFileItem[];
	} = $props();

	let items = $state<TrashFileItem[]>(untrack(() => initialItems.map((e) => ({ ...e }))));
	let selectedId = $state<string | null>(null);
	let purgeOpen = $state(false);
	let missingParentOpen = $state(false);
	let actionEntry = $state<TrashFileItem | null>(null);

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

	function restoreEntry(entry: TrashFileItem, toRoot = false) {
		removeItem(entry.id);
		toast.success(
			toRoot ? `“${entry.name}” dipulihkan ke Root` : `“${entry.name}” dipulihkan (mock)`
		);
	}

	function onRestore(entry: TrashFileItem) {
		if (entry.originalParentMissing) {
			actionEntry = entry;
			missingParentOpen = true;
			return;
		}
		restoreEntry(entry);
	}

	function onPurgeOne(entry: TrashFileItem) {
		removeItem(entry.id);
		toast.success(`“${entry.name}” dihapus permanen`);
	}

	function onPurgeAll() {
		const n = items.length;
		items = [];
		selectedId = null;
		toast.success(`${n} item dihapus permanen dari Trash`);
	}
</script>

<div class="flex flex-col gap-4">
	<div class="flex flex-wrap items-center justify-end gap-2">
		<Button
			type="button"
			variant="destructive"
			class="gap-1.5"
			disabled={items.length === 0}
			onclick={() => openPurge()}
		>
			<Trash2 class="size-4 stroke-2" />
			Kosongkan Sampah
		</Button>
		<DriveViewToggle />
	</div>

	{#if items.length === 0}
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
<PurgeTrashDialog bind:open={purgeOpen} itemCount={items.length} onConfirm={onPurgeAll} />
