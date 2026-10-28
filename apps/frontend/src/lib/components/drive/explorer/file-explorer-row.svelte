<script lang="ts">
	import type { ExplorerVariant, FileEntry } from '$lib/types/drive.js';
	import { formatSize } from '$lib/mocks/file-tree.js';
	import FileKindIcon from './file-kind-icon.svelte';
	import FileExplorerActionsMenu from './file-explorer-actions-menu.svelte';
	import FileExplorerContextMenu from './file-explorer-context-menu.svelte';
	import { Checkbox } from '$lib/components/ui/checkbox/index.js';
	import { cn } from '$lib/utils.js';
	import OfflineFileNotice from './offline-file-notice.svelte';
	import { locale } from '$lib/locale.svelte.js';

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
		onDelete,
		onCompress
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
		onCompress?: (entry: FileEntry) => void;
	} = $props();
	const modifiedAt = $derived(
		entry.modifiedAt === 'Baru saja'
			? locale.language === 'id'
				? 'Baru saja'
				: 'Just now'
			: entry.modifiedAt
	);
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
	{onCompress}
>
	<div
		role="option"
		tabindex="0"
		aria-selected={selected}
		aria-disabled={entry.kind !== 'folder' && entry.storageAvailable === false}
		class={cn(
			'group/row flex cursor-pointer items-center justify-between border-b border-subtle px-4 py-3 transition-colors hover:bg-black/5 focus-visible:ring-2 focus-visible:ring-primary/50 focus-visible:outline-none dark:hover:bg-white/5',
			entry.kind !== 'folder' && entry.storageAvailable === false && 'opacity-60',
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
				tabindex={entry.kind !== 'folder' && entry.storageAvailable === false ? -1 : 0}
				aria-label={`${locale.t('file.select')} ${entry.name}`}
				aria-disabled={entry.kind !== 'folder' && entry.storageAvailable === false}
				class="relative flex size-6 shrink-0 cursor-pointer items-center justify-center"
				onclick={(e) => {
					e.stopPropagation();
					if (entry.kind !== 'folder' && entry.storageAvailable === false) return;
					onToggleSelect?.(entry);
				}}
				onkeydown={(e) => {
					if (e.key === ' ' || e.key === 'Enter') {
						e.stopPropagation();
						if (entry.kind !== 'folder' && entry.storageAvailable === false) return;
						onToggleSelect?.(entry);
					}
				}}
			>
				<div
					class={cn(
						'flex items-center justify-center transition-opacity',
						selected ? 'hidden' : 'group-hover/row:hidden'
					)}
				>
					<FileKindIcon kind={entry.kind} />
				</div>
				<div
					class={cn(
						'items-center justify-center',
						selected ? 'flex' : 'hidden group-hover/row:flex'
					)}
				>
					<Checkbox
						checked={selected}
						disabled={entry.kind !== 'folder' && entry.storageAvailable === false}
						aria-label={`${locale.t('file.select')} ${entry.name}`}
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
						{locale.t('file.folder')} · {locale.t('file.changed')} {modifiedAt}
					{:else}
						{formatSize(entry.sizeBytes)} · {locale.t('file.changed')} {modifiedAt}
					{/if}
				</p>
			</div>
		</div>
		<div class="flex shrink-0 items-center gap-2">
			{#if entry.kind !== 'folder' && entry.storageAvailable === false}
				<OfflineFileNotice />
			{/if}
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
				{onCompress}
			/>
		</div>
	</div>
</FileExplorerContextMenu>
