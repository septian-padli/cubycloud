<script lang="ts">
	import type { FileEntry } from '$lib/types/drive.js';
	import { formatSize } from '$lib/mocks/file-tree.js';
	import { getDriveChrome } from '$lib/drive-context.svelte.js';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import {
		Copy,
		Download,
		FolderInput,
		MoreVertical,
		Pencil,
		Share2,
		Star,
		Trash2
	} from '@lucide/svelte';
	import FileKindIcon from './file-kind-icon.svelte';
	import StorageNodeBadge from './storage-node-badge.svelte';
	import { cn } from '$lib/utils.js';

	let {
		entry,
		selected = false,
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

<div
	role="listitem"
	tabindex="0"
	aria-selected={selected}
	class={cn(
		'flex cursor-pointer items-center justify-between border-b border-subtle px-4 py-3 transition-colors hover:bg-black/5 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/50 dark:hover:bg-white/5',
		selected && 'bg-primary/5'
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
			<p class="truncate text-sm font-medium text-main">
				{entry.name}
				{#if entry.starred}
					<Star class="ms-1 inline size-3.5 fill-secondary stroke-2 text-secondary" aria-label="Berbintang" />
				{/if}
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
		{#if showNodeBadge && entry.storageNode}
			<StorageNodeBadge node={entry.storageNode} />
		{/if}

		<DropdownMenu.Root>
			<DropdownMenu.Trigger>
				{#snippet child({ props })}
					<Button
						{...props}
						type="button"
						variant="ghost"
						size="icon-sm"
						class="text-muted hover:text-main"
						aria-label="Aksi berkas"
						onclick={(e: MouseEvent) => e.stopPropagation()}
					>
						<MoreVertical class="size-4 stroke-2" />
					</Button>
				{/snippet}
			</DropdownMenu.Trigger>
			<DropdownMenu.Content align="end" class="min-w-48">
				{#if entry.kind !== 'folder'}
					<DropdownMenu.Item
						class="gap-2"
						onclick={() => onDownload(entry)}
					>
						<Download class="size-4 stroke-2" />
						Unduh
					</DropdownMenu.Item>
				{/if}
				<DropdownMenu.Item class="gap-2" onclick={() => onRename(entry)}>
					<Pencil class="size-4 stroke-2" />
					Ganti nama
				</DropdownMenu.Item>
				<DropdownMenu.Item class="gap-2" onclick={() => onMove(entry)}>
					<FolderInput class="size-4 stroke-2" />
					Pindahkan
				</DropdownMenu.Item>
				{#if entry.kind !== 'folder'}
					<DropdownMenu.Item class="gap-2" onclick={() => onCopy(entry)}>
						<Copy class="size-4 stroke-2" />
						Salin zero-byte
					</DropdownMenu.Item>
				{/if}
				<DropdownMenu.Item class="gap-2" onclick={() => onStar(entry)}>
					<Star class="size-4 stroke-2" />
					{entry.starred ? 'Hapus bintang' : 'Bintang'}
				</DropdownMenu.Item>
				{#if entry.kind !== 'folder'}
					<DropdownMenu.Item class="gap-2" onclick={() => onShare(entry)}>
						<Share2 class="size-4 stroke-2" />
						Bagikan
					</DropdownMenu.Item>
				{/if}
				<DropdownMenu.Separator />
				<DropdownMenu.Item
					variant="destructive"
					class="gap-2"
					onclick={() => onDelete(entry)}
				>
					<Trash2 class="size-4 stroke-2" />
					Hapus ke Trash
				</DropdownMenu.Item>
			</DropdownMenu.Content>
		</DropdownMenu.Root>
	</div>
</div>
