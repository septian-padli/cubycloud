<script lang="ts">
	import type { FileEntry } from '$lib/types/drive.js';
	import { formatSize } from '$lib/mocks/file-tree.js';
	import DriveFileGrid from '$lib/components/drive/drive-file-grid.svelte';
	import FileGridCard from '$lib/components/drive/file-grid-card.svelte';
	import DriveListRow from '$lib/components/drive/drive-list-row.svelte';
	import FileKindIcon from '$lib/components/drive/explorer/file-kind-icon.svelte';
	import { driveView } from '$lib/drive-view.svelte.js';

	let {
		items,
		onOpen
	}: {
		items: FileEntry[];
		onOpen: (entry: FileEntry) => void;
	} = $props();
</script>

{#if items.length === 0}
	<div class="overflow-hidden rounded-xl border border-subtle bg-surface px-4 py-12 text-center">
		<p class="text-sm font-medium text-main">Folder kosong</p>
		<p class="mt-1 text-xs text-muted">Tidak ada berkas di folder ini.</p>
	</div>
{:else if driveView.mode === 'grid'}
	<DriveFileGrid label="Berkas dibagikan">
		{#each items as entry (entry.id)}
			<FileGridCard
				kind={entry.kind}
				name={entry.name}
				subtitle={entry.kind === 'folder'
					? `Folder · ${entry.modifiedAt}`
					: `${formatSize(entry.sizeBytes)} · ${entry.modifiedAt}`}
				onOpen={() => onOpen(entry)}
				onSelect={() => onOpen(entry)}
			/>
		{/each}
	</DriveFileGrid>
{:else}
	<div
		class="overflow-hidden rounded-xl border border-subtle bg-surface"
		role="listbox"
		aria-label="Berkas dibagikan"
	>
		{#each items as entry (entry.id)}
			<DriveListRow onOpen={() => onOpen(entry)} onSelect={() => onOpen(entry)}>
				{#snippet leading()}
					<FileKindIcon kind={entry.kind} />
				{/snippet}
				{#snippet meta()}
					<p class="truncate text-sm font-medium text-main">{entry.name}</p>
					<p class="text-xs text-muted">
						{#if entry.kind === 'folder'}
							Folder · {entry.modifiedAt}
						{:else}
							{formatSize(entry.sizeBytes)} · {entry.modifiedAt}
						{/if}
					</p>
				{/snippet}
			</DriveListRow>
		{/each}
	</div>
{/if}
