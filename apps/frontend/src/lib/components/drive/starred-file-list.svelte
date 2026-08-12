<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { untrack } from 'svelte';
	import { toast } from 'svelte-sonner';
	import type { StarredFileItem } from '$lib/types/drive.js';
	import { formatSize } from '$lib/mocks/file-tree.js';
	import { driveView } from '$lib/drive-view.svelte.js';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu/index.js';
	import * as ContextMenu from '$lib/components/ui/context-menu/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Eye, MoreVertical, Share2, Star } from '@lucide/svelte';
	import StarredFileRow from './starred-file-row.svelte';
	import FilePreviewDialog from './explorer/file-preview-dialog.svelte';
	import ShareSheet from './explorer/share-sheet.svelte';
	import DriveFileGrid from './drive-file-grid.svelte';
	import FileGridCard from './file-grid-card.svelte';

	let {
		initialItems
	}: {
		initialItems: StarredFileItem[];
	} = $props();

	let items = $state<StarredFileItem[]>(untrack(() => initialItems.map((e) => ({ ...e }))));
	let selectedId = $state<string | null>(null);
	let shareOpen = $state(false);
	let actionEntry = $state<StarredFileItem | null>(null);

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

	function onPreview(entry: StarredFileItem) {
		if (entry.kind === 'folder') {
			toast.info(`Buka folder “${entry.name}” dari sumber asalnya.`);
			return;
		}
		syncPreview(entry.id);
	}

	function onUnstar(entry: StarredFileItem) {
		items = items.filter((e) => e.id !== entry.id);
		if (selectedId === entry.id) selectedId = null;
		if (previewId === entry.id) syncPreview(null);
		toast.success(`Bintang dihapus dari “${entry.name}”`);
	}
</script>

{#if items.length === 0}
	<div class="overflow-hidden rounded-xl border border-subtle bg-surface px-4 py-12 text-center">
		<p class="text-sm font-medium text-main">Belum ada berkas berbintang</p>
		<p class="mt-1 text-xs text-muted">
			Tandai berkas dengan bintang dari My Storage atau Organization.
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
						selected={selectedId === entry.id}
						onSelect={() => (selectedId = entry.id)}
						onOpen={() => onPreview(entry)}
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
											aria-label="Aksi berkas"
											onclick={(e: MouseEvent) => e.stopPropagation()}
										>
											<MoreVertical class="size-4 stroke-2" />
										</Button>
									{/snippet}
								</DropdownMenu.Trigger>
								<DropdownMenu.Content align="end" class="min-w-48">
									{#if entry.kind !== 'folder'}
										<DropdownMenu.Item class="gap-2" onclick={() => onPreview(entry)}>
											<Eye class="size-4 stroke-2" />
											Pratinjau
										</DropdownMenu.Item>
									{/if}
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
									<DropdownMenu.Item class="gap-2" onclick={() => onUnstar(entry)}>
										<Star class="size-4 stroke-2" />
										Hapus bintang
									</DropdownMenu.Item>
								</DropdownMenu.Content>
							</DropdownMenu.Root>
						{/snippet}
					</FileGridCard>
				</ContextMenu.Trigger>
				<ContextMenu.Content class="min-w-48">
					{#if entry.kind !== 'folder'}
						<ContextMenu.Item class="gap-2" onclick={() => onPreview(entry)}>
							<Eye class="size-4 stroke-2" />
							Pratinjau
						</ContextMenu.Item>
					{/if}
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
					<ContextMenu.Item class="gap-2" onclick={() => onUnstar(entry)}>
						<Star class="size-4 stroke-2" />
						Hapus bintang
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
			<StarredFileRow
				{entry}
				selected={selectedId === entry.id}
				onSelect={(e) => (selectedId = e.id)}
				onPreview={onPreview}
				onShare={(e) => {
					actionEntry = e;
					shareOpen = true;
				}}
				onUnstar={onUnstar}
			/>
		{/each}
	</div>
{/if}

<ShareSheet bind:open={shareOpen} entry={actionEntry} />
<FilePreviewDialog open={previewOpen} entry={previewEntry} onClose={() => syncPreview(null)} />
