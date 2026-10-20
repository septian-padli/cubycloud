<script lang="ts">
	import type { ExplorerVariant, FileEntry } from '$lib/types/drive.js';
	import { formatSize } from '$lib/mocks/file-tree.js';
	import { getDriveChrome } from '$lib/drive-context.svelte.js';
	import FileKindIcon from './file-kind-icon.svelte';
	import FileExplorerActionsMenu from './file-explorer-actions-menu.svelte';
	import FileExplorerContextMenu from './file-explorer-context-menu.svelte';
	import { Checkbox } from '$lib/components/ui/checkbox/index.js';
	import { cn } from '$lib/utils.js';

	let {
		entry,
		variant,
		selected = false,
		isDragOver = false,
		onOpen,
		onSelect,
		onToggleSelect,
		onDownload,
		onRename,
		onMove,
		onCopy,
		onStar,
		onShare,
		onDelete
	}: {
		entry: FileEntry;
		variant?: ExplorerVariant;
		selected?: boolean;
		isDragOver?: boolean;
		onOpen: (entry: FileEntry) => void;
		onSelect: (entry: FileEntry) => void;
		onToggleSelect?: (entry: FileEntry) => void;
		onDownload: (entry: FileEntry) => void;
		onRename: (entry: FileEntry) => void;
		onMove: (entry: FileEntry) => void;
		onCopy: (entry: FileEntry) => void;
		onStar: (entry: FileEntry) => void;
		onShare: (entry: FileEntry) => void;
		onDelete: (entry: FileEntry) => void;
	} = $props();

	const { user } = getDriveChrome();
</script>

<FileExplorerContextMenu
	{entry}
	{variant}
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
			'group/row flex cursor-pointer items-center justify-between border-b border-subtle px-4 py-3 transition-colors hover:bg-black/5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/50 dark:hover:bg-white/5',
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
			<div
				role="button"
				tabindex="0"
				aria-label={`Pilih ${entry.name}`}
				class="relative flex size-6 shrink-0 items-center justify-center cursor-pointer"
				onclick={(e) => {
					e.stopPropagation();
					onToggleSelect?.(entry);
				}}
				onkeydown={(e) => {
					if (e.key === ' ' || e.key === 'Enter') {
						e.stopPropagation();
						onToggleSelect?.(entry);
					}
				}}
			>
				<div class={cn('flex items-center justify-center transition-opacity', selected ? 'hidden' : 'group-hover/row:hidden')}>
					<FileKindIcon kind={entry.kind} />
				</div>
				<div class={cn('items-center justify-center', selected ? 'flex' : 'hidden group-hover/row:flex')}>
					<Checkbox
						checked={selected}
						aria-label={`Pilih ${entry.name}`}
						class="pointer-events-none"
						tabindex={-1}
					/>
				</div>
			</div>
			<div class="min-w-0">
				<p class="truncate text-sm font-medium text-main">
					{entry.name}
				</p>
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
			<FileExplorerActionsMenu
				{entry}
				{variant}
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
