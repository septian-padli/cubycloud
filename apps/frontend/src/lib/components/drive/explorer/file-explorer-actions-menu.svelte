<script lang="ts">
	import type { ExplorerVariant, FileEntry } from '$lib/types/drive.js';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import {
		Archive,
		Copy,
		Download,
		Eye,
		FolderInput,
		MoreVertical,
		Pencil,
		Share2,
		Star,
		Trash2
	} from '@lucide/svelte';
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
		onCompress
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
		onCompress?: (entry: FileEntry) => void;
	} = $props();
</script>

<DropdownMenu.Root>
	<DropdownMenu.Trigger>
		{#snippet child({ props })}
			<Button
				{...props}
				type="button"
				variant="ghost"
				size="icon"
				class="bg-surface text-muted hover:bg-primary hover:text-white aria-expanded:bg-primary aria-expanded:text-white"
				aria-label={locale.t('file.actions')}
				onclick={(e: MouseEvent) => e.stopPropagation()}
			>
				<MoreVertical class="size-4 stroke-2" />
			</Button>
		{/snippet}
	</DropdownMenu.Trigger>
	<DropdownMenu.Content align="end" class="min-w-48">
		{#if entry.kind !== 'folder'}
			<DropdownMenu.Item
				disabled={entry.storageAvailable === false}
				class="gap-2"
				onclick={() => onOpen(entry)}
			>
				<Eye class="size-4 stroke-2" />
				{locale.language === 'id' ? 'Pratinjau' : 'Preview'}
			</DropdownMenu.Item>
			<DropdownMenu.Item
				disabled={entry.storageAvailable === false}
				class="gap-2"
				onclick={() => onDownload(entry)}
			>
				<Download class="size-4 stroke-2" />
				{locale.t('action.download')}
			</DropdownMenu.Item>
		{:else}
			<DropdownMenu.Item class="gap-2" onclick={() => onOpen(entry)}>
				<Eye class="size-4 stroke-2" />
				{locale.t('action.open')}
			</DropdownMenu.Item>
			<DropdownMenu.Item class="gap-2" onclick={() => onDownload(entry)}>
				<Download class="size-4 stroke-2" />
				{locale.t('action.download')}
			</DropdownMenu.Item>
		{/if}
		{#if onCompress}
			<DropdownMenu.Item
				disabled={entry.kind !== 'folder' && entry.storageAvailable === false}
				class="gap-2"
				onclick={() => onCompress?.(entry)}
				data-testid="action-compress"
			>
				<Archive class="size-4 stroke-2" />
				{locale.t('action.compress')}
			</DropdownMenu.Item>
		{/if}
		<DropdownMenu.Item
			disabled={entry.kind !== 'folder' && entry.storageAvailable === false}
			class="gap-2"
			onclick={() => onRename(entry)}
		>
			<Pencil class="size-4 stroke-2" />
			{locale.t('action.rename')}
		</DropdownMenu.Item>
		<DropdownMenu.Item
			disabled={entry.kind !== 'folder' && entry.storageAvailable === false}
			class="gap-2"
			onclick={() => onMove(entry)}
		>
			<FolderInput class="size-4 stroke-2" />
			{locale.t('action.move')}
		</DropdownMenu.Item>
		<DropdownMenu.Item
			disabled={entry.kind !== 'folder' && entry.storageAvailable === false}
			class="gap-2"
			onclick={() => onCopy(entry)}
		>
			<Copy class="size-4 stroke-2" />
			{locale.t('action.copy')}
		</DropdownMenu.Item>
		<DropdownMenu.Item
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
		</DropdownMenu.Item>
		{#if allowShare}
			<DropdownMenu.Item
				disabled={entry.kind !== 'folder' && entry.storageAvailable === false}
				class="gap-2"
				onclick={() => onShare(entry)}
			>
				<Share2 class="size-4 stroke-2" />
				{locale.t('action.share')}
			</DropdownMenu.Item>
		{/if}
		<DropdownMenu.Separator />
		<DropdownMenu.Item
			disabled={entry.kind !== 'folder' && entry.storageAvailable === false}
			variant="destructive"
			class="gap-2"
			onclick={() => onDelete(entry)}
		>
			<Trash2 class="size-4 stroke-2" />
			{locale.language === 'id' ? 'Hapus ke Trash' : 'Move to Trash'}
		</DropdownMenu.Item>
	</DropdownMenu.Content>
</DropdownMenu.Root>
