<script lang="ts">
	import type { SharedFileItem } from '$lib/types/drive.js';
	import { formatSize } from '$lib/mocks/file-tree.js';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu/index.js';
	import * as ContextMenu from '$lib/components/ui/context-menu/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import {
		Download,
		Eye,
		Info,
		MoreVertical,
		Pencil,
		Share2,
		Trash2
	} from '@lucide/svelte';
	import DriveListRow from './drive-list-row.svelte';
	import PermissionBadge from './permission-badge.svelte';
	import FileKindIcon from './explorer/file-kind-icon.svelte';

	let {
		entry,
		selected = false,
		onSelect,
		onPreview,
		onDownload,
		onDetails,
		onShare,
		onEdit,
		onDelete
	}: {
		entry: SharedFileItem;
		selected?: boolean;
		onSelect: (entry: SharedFileItem) => void;
		onPreview: (entry: SharedFileItem) => void;
		onDownload: (entry: SharedFileItem) => void;
		onDetails: (entry: SharedFileItem) => void;
		onShare: (entry: SharedFileItem) => void;
		onEdit: (entry: SharedFileItem) => void;
		onDelete: (entry: SharedFileItem) => void;
	} = $props();

	const isEditor = $derived(entry.permission === 'editor');
</script>

<ContextMenu.Root>
	<ContextMenu.Trigger class="block w-full">
		<DriveListRow
			{selected}
			onSelect={() => onSelect(entry)}
			onOpen={() => onPreview(entry)}
		>
			{#snippet leading()}
				<FileKindIcon kind={entry.kind} />
			{/snippet}
			{#snippet meta()}
				<p class="truncate text-sm font-medium text-main">{entry.name}</p>
				<p class="text-xs text-muted">
					{formatSize(entry.sizeBytes)} · Dari {entry.ownerName} · {entry.modifiedAt}
				</p>
			{/snippet}
			{#snippet trailing()}
				<PermissionBadge permission={entry.permission} />
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
						<DropdownMenu.Item class="gap-2" onclick={() => onPreview(entry)}>
							<Eye class="size-4 stroke-2" />
							Pratinjau
						</DropdownMenu.Item>
						<DropdownMenu.Item class="gap-2" onclick={() => onDownload(entry)}>
							<Download class="size-4 stroke-2" />
							Unduh
						</DropdownMenu.Item>
						<DropdownMenu.Item class="gap-2" onclick={() => onDetails(entry)}>
							<Info class="size-4 stroke-2" />
							Detail berbagi
						</DropdownMenu.Item>
						{#if isEditor}
							<DropdownMenu.Separator />
							<DropdownMenu.Item class="gap-2" onclick={() => onShare(entry)}>
								<Share2 class="size-4 stroke-2" />
								Bagikan
							</DropdownMenu.Item>
							<DropdownMenu.Item class="gap-2" onclick={() => onEdit(entry)}>
								<Pencil class="size-4 stroke-2" />
								Edit
							</DropdownMenu.Item>
							<DropdownMenu.Item
								variant="destructive"
								class="gap-2"
								onclick={() => onDelete(entry)}
							>
								<Trash2 class="size-4 stroke-2" />
								Hapus
							</DropdownMenu.Item>
						{/if}
					</DropdownMenu.Content>
				</DropdownMenu.Root>
			{/snippet}
		</DriveListRow>
	</ContextMenu.Trigger>
	<ContextMenu.Content class="min-w-48">
		<ContextMenu.Item class="gap-2" onclick={() => onPreview(entry)}>
			<Eye class="size-4 stroke-2" />
			Pratinjau
		</ContextMenu.Item>
		<ContextMenu.Item class="gap-2" onclick={() => onDownload(entry)}>
			<Download class="size-4 stroke-2" />
			Unduh
		</ContextMenu.Item>
		<ContextMenu.Item class="gap-2" onclick={() => onDetails(entry)}>
			<Info class="size-4 stroke-2" />
			Detail berbagi
		</ContextMenu.Item>
		{#if isEditor}
			<ContextMenu.Separator />
			<ContextMenu.Item class="gap-2" onclick={() => onShare(entry)}>
				<Share2 class="size-4 stroke-2" />
				Bagikan
			</ContextMenu.Item>
			<ContextMenu.Item class="gap-2" onclick={() => onEdit(entry)}>
				<Pencil class="size-4 stroke-2" />
				Edit
			</ContextMenu.Item>
			<ContextMenu.Item variant="destructive" class="gap-2" onclick={() => onDelete(entry)}>
				<Trash2 class="size-4 stroke-2" />
				Hapus
			</ContextMenu.Item>
		{/if}
	</ContextMenu.Content>
</ContextMenu.Root>
