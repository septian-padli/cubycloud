<script lang="ts">
	import { locale } from '$lib/locale.svelte.js';
	import type { SharedFileItem } from '$lib/types/drive.js';
	import { formatSize } from '$lib/mocks/file-tree.js';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu/index.js';
	import * as ContextMenu from '$lib/components/ui/context-menu/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Download, Eye, FolderOpen, Info, MoreVertical, Pencil, Trash2 } from '@lucide/svelte';
	import DriveListRow from './drive-list-row.svelte';
	import PermissionBadge from './permission-badge.svelte';
	import FileKindIcon from './explorer/file-kind-icon.svelte';
	const tx = (id: string, en: string) => (locale.language === 'id' ? id : en);

	let {
		entry,
		selected = false,
		canEdit = false,
		onSelect,
		onOpen,
		onDownload,
		onDetails,
		onRename,
		onDelete
	}: {
		entry: SharedFileItem;
		selected?: boolean;
		canEdit?: boolean;
		onSelect: (entry: SharedFileItem) => void;
		onOpen: (entry: SharedFileItem) => void;
		onDownload: (entry: SharedFileItem) => void;
		onDetails: (entry: SharedFileItem) => void;
		onRename?: (entry: SharedFileItem) => void;
		onDelete?: (entry: SharedFileItem) => void;
	} = $props();

	const isFolder = $derived(entry.kind === 'folder');
</script>

<ContextMenu.Root>
	<ContextMenu.Trigger class="block w-full">
		<DriveListRow {selected} onSelect={() => onSelect(entry)} onOpen={() => onOpen(entry)}>
			{#snippet leading()}
				<FileKindIcon kind={entry.kind} />
			{/snippet}
			{#snippet meta()}
				<p class="truncate text-sm font-medium text-main">{entry.name}</p>
				<p class="text-xs text-muted">
					{#if isFolder}
						Folder · {tx('Dari', 'From')} {entry.ownerName} · {locale.formatDate(entry.modifiedAt)}
					{:else}
						{formatSize(entry.sizeBytes)} · {tx('Dari', 'From')}
						{entry.ownerName} · {locale.formatDate(entry.modifiedAt)}
					{/if}
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
								aria-label={tx('Aksi berkas', 'File actions')}
								onclick={(e: MouseEvent) => e.stopPropagation()}
							>
								<MoreVertical class="size-4 stroke-2" />
							</Button>
						{/snippet}
					</DropdownMenu.Trigger>
					<DropdownMenu.Content align="end" class="min-w-48 border-subtle bg-surface">
						{#if isFolder}
							<DropdownMenu.Item class="gap-2" onclick={() => onOpen(entry)}>
								<FolderOpen class="size-4 stroke-2" />
								{tx('Buka folder', 'Open folder')}
							</DropdownMenu.Item>
						{:else}
							<DropdownMenu.Item class="gap-2" onclick={() => onOpen(entry)}>
								<Eye class="size-4 stroke-2" />
								{tx('Pratinjau', 'Preview')}
							</DropdownMenu.Item>
						{/if}
						<DropdownMenu.Item class="gap-2" onclick={() => onDownload(entry)}>
							<Download class="size-4 stroke-2" />
							{tx('Unduh', 'Download')}
						</DropdownMenu.Item>
						<DropdownMenu.Item class="gap-2" onclick={() => onDetails(entry)}>
							<Info class="size-4 stroke-2" />
							{tx('Detail berbagi', 'Sharing details')}
						</DropdownMenu.Item>
						{#if canEdit}
							<DropdownMenu.Separator />
							{#if onRename}
								<DropdownMenu.Item class="gap-2" onclick={() => onRename(entry)}>
									<Pencil class="size-4 stroke-2" />
									{tx('Ubah nama', 'Rename')}
								</DropdownMenu.Item>
							{/if}
							{#if onDelete}
								<DropdownMenu.Item
									variant="destructive"
									class="gap-2"
									onclick={() => onDelete(entry)}
								>
									<Trash2 class="size-4 stroke-2" />
									{tx('Hapus', 'Delete')}
								</DropdownMenu.Item>
							{/if}
						{/if}
					</DropdownMenu.Content>
				</DropdownMenu.Root>
			{/snippet}
		</DriveListRow>
	</ContextMenu.Trigger>
	<ContextMenu.Content class="min-w-48 border-subtle bg-surface">
		{#if isFolder}
			<ContextMenu.Item class="gap-2" onclick={() => onOpen(entry)}>
				<FolderOpen class="size-4 stroke-2" />
				{tx('Buka folder', 'Open folder')}
			</ContextMenu.Item>
		{:else}
			<ContextMenu.Item class="gap-2" onclick={() => onOpen(entry)}>
				<Eye class="size-4 stroke-2" />
				{tx('Pratinjau', 'Preview')}
			</ContextMenu.Item>
		{/if}
		<ContextMenu.Item class="gap-2" onclick={() => onDownload(entry)}>
			<Download class="size-4 stroke-2" />
			{tx('Unduh', 'Download')}
		</ContextMenu.Item>
		<ContextMenu.Item class="gap-2" onclick={() => onDetails(entry)}>
			<Info class="size-4 stroke-2" />
			{tx('Detail berbagi', 'Sharing details')}
		</ContextMenu.Item>
		{#if canEdit}
			<ContextMenu.Separator />
			{#if onRename}
				<ContextMenu.Item class="gap-2" onclick={() => onRename(entry)}>
					<Pencil class="size-4 stroke-2" />
					{tx('Ubah nama', 'Rename')}
				</ContextMenu.Item>
			{/if}
			{#if onDelete}
				<ContextMenu.Item variant="destructive" class="gap-2" onclick={() => onDelete(entry)}>
					<Trash2 class="size-4 stroke-2" />
					{tx('Hapus', 'Delete')}
				</ContextMenu.Item>
			{/if}
		{/if}
	</ContextMenu.Content>
</ContextMenu.Root>
