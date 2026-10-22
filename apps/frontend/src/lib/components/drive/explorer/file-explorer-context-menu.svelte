<script lang="ts">
	import type { ExplorerVariant, FileEntry } from '$lib/types/drive.js';
	import type { Snippet } from 'svelte';
	import * as ContextMenu from '$lib/components/ui/context-menu/index.js';
	import { Copy, Download, Eye, FolderInput, Pencil, Share2, Star, Trash2 } from '@lucide/svelte';
	import { cn } from '$lib/utils.js';
	import { locale } from '$lib/locale.svelte.js';

	let {
		entry,
		variant,
		allowShare = variant !== 'organization',
		onOpen,
		onDownload,
		onRename,
		onMove,
		onCopy,
		onStar,
		onShare,
		onDelete,
		children
	}: {
		entry: FileEntry;
		variant?: ExplorerVariant;
		allowShare?: boolean;
		onOpen: (entry: FileEntry) => void;
		onDownload: (entry: FileEntry) => void;
		onRename: (entry: FileEntry) => void;
		onMove: (entry: FileEntry) => void;
		onCopy: (entry: FileEntry) => void;
		onStar: (entry: FileEntry) => void;
		onShare: (entry: FileEntry) => void;
		onDelete: (entry: FileEntry) => void;
		children: Snippet;
	} = $props();
</script>

<ContextMenu.Root>
	<ContextMenu.Trigger class="block w-full">
		{@render children()}
	</ContextMenu.Trigger>
	<ContextMenu.Content class="min-w-48">
		{#if entry.kind !== 'folder'}
			<ContextMenu.Item
				disabled={entry.storageAvailable === false}
				class="gap-2"
				onclick={() => onOpen(entry)}
			>
				<Eye class="size-4 stroke-2" />
				{locale.language === 'id' ? 'Pratinjau' : 'Preview'}
			</ContextMenu.Item>
			<ContextMenu.Item
				disabled={entry.storageAvailable === false}
				class="gap-2"
				onclick={() => onDownload(entry)}
			>
				<Download class="size-4 stroke-2" />
				{locale.t('action.download')}
			</ContextMenu.Item>
		{:else}
			<ContextMenu.Item class="gap-2" onclick={() => onOpen(entry)}>
				<Eye class="size-4 stroke-2" />
				{locale.t('action.open')}
			</ContextMenu.Item>
			<ContextMenu.Item class="gap-2" onclick={() => onDownload(entry)}>
				<Download class="size-4 stroke-2" />
				{locale.t('action.download')}
			</ContextMenu.Item>
		{/if}
		<ContextMenu.Item
			disabled={entry.kind !== 'folder' && entry.storageAvailable === false}
			class="gap-2"
			onclick={() => onRename(entry)}
		>
			<Pencil class="size-4 stroke-2" />
			{locale.t('action.rename')}
		</ContextMenu.Item>
		<ContextMenu.Item
			disabled={entry.kind !== 'folder' && entry.storageAvailable === false}
			class="gap-2"
			onclick={() => onMove(entry)}
		>
			<FolderInput class="size-4 stroke-2" />
			{locale.t('action.move')}
		</ContextMenu.Item>
		<ContextMenu.Item
			disabled={entry.kind !== 'folder' && entry.storageAvailable === false}
			class="gap-2"
			onclick={() => onCopy(entry)}
		>
			<Copy class="size-4 stroke-2" />
			{locale.t('action.copy')}
		</ContextMenu.Item>
		<ContextMenu.Item
			disabled={entry.kind !== 'folder' && entry.storageAvailable === false}
			class="gap-2"
			onclick={() => onStar(entry)}
		>
			<Star
				class={cn(
					'size-4 stroke-2',
					entry.starred ? 'fill-amber-400 stroke-amber-500 text-amber-500' : ''
				)}
			/>
			{entry.starred
				? locale.language === 'id'
					? 'Hapus Bintang'
					: 'Unstar'
				: locale.t('action.star')}
		</ContextMenu.Item>
		{#if allowShare}
			<ContextMenu.Item
				disabled={entry.kind !== 'folder' && entry.storageAvailable === false}
				class="gap-2"
				onclick={() => onShare(entry)}
			>
				<Share2 class="size-4 stroke-2" />
				{locale.t('action.share')}
			</ContextMenu.Item>
		{/if}
		<ContextMenu.Separator />
		<ContextMenu.Item
			disabled={entry.kind !== 'folder' && entry.storageAvailable === false}
			variant="destructive"
			class="gap-2"
			onclick={() => onDelete(entry)}
		>
			<Trash2 class="size-4 stroke-2" />
			{locale.language === 'id' ? 'Hapus ke Trash' : 'Move to Trash'}
		</ContextMenu.Item>
	</ContextMenu.Content>
</ContextMenu.Root>
