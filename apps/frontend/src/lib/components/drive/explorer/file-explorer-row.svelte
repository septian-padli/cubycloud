<script lang="ts">
	import type { FileEntry } from '$lib/types/drive.js';
	import { formatSize } from '$lib/mocks/file-tree.js';
	import { getDriveChrome } from '$lib/drive-context.svelte.js';
	import { Star } from '@lucide/svelte';
	import FileKindIcon from './file-kind-icon.svelte';
	import StorageNodeBadge from './storage-node-badge.svelte';
	import FileExplorerActionsMenu from './file-explorer-actions-menu.svelte';
	import FileExplorerContextMenu from './file-explorer-context-menu.svelte';
	import { cn } from '$lib/utils.js';

	let {
		entry,
		selected = false,
		isDragOver = false,
		onOpen,
		onSelect,
		onDownload,
		onRename,
		onMove,
		onCopy,
		onStar,
		onShare,
		onDelete
	}: {
		entry: FileEntry;
		selected?: boolean;
		isDragOver?: boolean;
		onOpen: (entry: FileEntry) => void;
		onSelect: (entry: FileEntry) => void;
		onDownload: (entry: FileEntry) => void;
		onRename: (entry: FileEntry) => void;
		onMove: (entry: FileEntry) => void;
		onCopy: (entry: FileEntry) => void;
		onStar: (entry: FileEntry) => void;
		onShare: (entry: FileEntry) => void;
		onDelete: (entry: FileEntry) => void;
	} = $props();

	const { user } = getDriveChrome();
	const showNodeBadge = $derived(user.role === 'super_admin' && !!entry.storageNode);
</script>

<FileExplorerContextMenu
	{entry}
	{onOpen}
	{onDownload}
	{onRename}
	{onMove}
	{onCopy}
	{onStar}
	{onShare}
	{onDelete}
>
	<div
		role="option"
		tabindex="0"
		aria-selected={selected}
		class={cn(
			'flex cursor-pointer items-center justify-between border-b border-subtle px-4 py-3 transition-colors hover:bg-black/5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/50 dark:hover:bg-white/5',
			selected && 'bg-primary/5',
			isDragOver && 'border-primary bg-accent/40 ring-2 ring-primary/50'
		)}
		onclick={() => onSelect(entry)}
		ondblclick={() => onOpen(entry)}
		onkeydown={(e) => {
			if (e.key === 'Enter') onOpen(entry);
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
						title={entry.starred ? 'Hapus Bintang' : 'Bintang'}
						aria-label={entry.starred ? 'Hapus Bintang' : 'Bintang'}
						onclick={(e) => {
							e.stopPropagation();
							onStar(entry);
						}}
					>
						<Star
							class={cn(
								'size-3.5 transition-colors',
								entry.starred
									? 'fill-amber-400 stroke-amber-500 text-amber-500'
									: 'stroke-1 text-muted-foreground/40 hover:text-amber-500'
							)}
						/>
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
			{#if showNodeBadge && entry.storageNode}
				<StorageNodeBadge node={entry.storageNode} />
			{/if}
			<FileExplorerActionsMenu
				{entry}
				{onOpen}
				{onDownload}
				{onRename}
				{onMove}
				{onCopy}
				{onStar}
				{onShare}
				{onDelete}
			/>
		</div>
	</div>
</FileExplorerContextMenu>
