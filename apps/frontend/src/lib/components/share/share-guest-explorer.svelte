<script lang="ts">
	import type { FileEntry } from '$lib/types/drive.js';
	import { formatSize } from '$lib/mocks/file-tree.js';
	import DriveFileGrid from '$lib/components/drive/drive-file-grid.svelte';
	import FileGridCard from '$lib/components/drive/file-grid-card.svelte';
	import DriveListRow from '$lib/components/drive/drive-list-row.svelte';
	import FileKindIcon from '$lib/components/drive/explorer/file-kind-icon.svelte';
	import { driveView } from '$lib/drive-view.svelte.js';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu/index.js';
	import * as ContextMenu from '$lib/components/ui/context-menu/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Download, Eye, FolderOpen, MoreVertical, Pencil, Trash2 } from '@lucide/svelte';
	import { locale } from '$lib/locale.svelte.js';

	let {
		items,
		onOpen,
		canEdit = false,
		onDownload,
		onRename,
		onDelete
	}: {
		items: FileEntry[];
		onOpen: (entry: FileEntry) => void;
		canEdit?: boolean;
		onDownload?: (entry: FileEntry) => void;
		onRename?: (entry: FileEntry) => void;
		onDelete?: (entry: FileEntry) => void;
	} = $props();

	let selectedId = $state<string | null>(null);
</script>

{#if items.length === 0}
	<div class="overflow-hidden rounded-xl border border-subtle bg-surface px-4 py-12 text-center">
		<p class="text-sm font-medium text-main">
			{locale.language === 'id' ? 'Folder kosong' : 'Empty folder'}
		</p>
		<p class="mt-1 text-xs text-muted">
			{locale.language === 'id'
				? 'Tidak ada berkas di dalam folder ini.'
				: 'There are no files in this folder.'}
		</p>
	</div>
{:else if driveView.mode === 'grid'}
	<DriveFileGrid label={locale.language === 'id' ? 'Berkas dibagikan' : 'Shared files'}>
		{#each items as entry (entry.id)}
			{@const isFolder = entry.kind === 'folder'}
			<ContextMenu.Root>
				<ContextMenu.Trigger class="block w-full">
					<FileGridCard
						kind={entry.kind}
						name={entry.name}
						subtitle={isFolder
							? `Folder · ${locale.formatDate(entry.modifiedAt)}`
							: `${formatSize(entry.sizeBytes)} · ${locale.formatDate(entry.modifiedAt)}`}
						selected={selectedId === entry.id}
						onOpen={() => onOpen(entry)}
						onSelect={() => (selectedId = entry.id)}
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
											aria-label={locale.language === 'id' ? 'Aksi berkas' : 'File actions'}
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
											{locale.language === 'id' ? 'Buka folder' : 'Open folder'}
										</DropdownMenu.Item>
									{:else}
										<DropdownMenu.Item class="gap-2" onclick={() => onOpen(entry)}>
											<Eye class="size-4 stroke-2" />
											{locale.language === 'id' ? 'Pratinjau' : 'Preview'}
										</DropdownMenu.Item>
									{/if}
									{#if onDownload}
										<DropdownMenu.Item class="gap-2" onclick={() => onDownload(entry)}>
											<Download class="size-4 stroke-2" />
											{locale.language === 'id' ? 'Unduh' : 'Download'}
										</DropdownMenu.Item>
									{/if}
									{#if canEdit}
										<DropdownMenu.Separator />
										{#if onRename}
											<DropdownMenu.Item class="gap-2" onclick={() => onRename(entry)}>
												<Pencil class="size-4 stroke-2" />
												{locale.language === 'id' ? 'Ubah nama' : 'Rename'}
											</DropdownMenu.Item>
										{/if}
										{#if onDelete}
											<DropdownMenu.Item
												variant="destructive"
												class="gap-2"
												onclick={() => onDelete(entry)}
											>
												<Trash2 class="size-4 stroke-2" />
												{locale.language === 'id' ? 'Hapus' : 'Delete'}
											</DropdownMenu.Item>
										{/if}
									{/if}
								</DropdownMenu.Content>
							</DropdownMenu.Root>
						{/snippet}
					</FileGridCard>
				</ContextMenu.Trigger>
				<ContextMenu.Content class="min-w-48 border-subtle bg-surface">
					{#if isFolder}
						<ContextMenu.Item class="gap-2" onclick={() => onOpen(entry)}>
							<FolderOpen class="size-4 stroke-2" />
							{locale.language === 'id' ? 'Buka folder' : 'Open folder'}
						</ContextMenu.Item>
					{:else}
						<ContextMenu.Item class="gap-2" onclick={() => onOpen(entry)}>
							<Eye class="size-4 stroke-2" />
							{locale.language === 'id' ? 'Pratinjau' : 'Preview'}
						</ContextMenu.Item>
					{/if}
					{#if onDownload}
						<ContextMenu.Item class="gap-2" onclick={() => onDownload(entry)}>
							<Download class="size-4 stroke-2" />
							{locale.language === 'id' ? 'Unduh' : 'Download'}
						</ContextMenu.Item>
					{/if}
					{#if canEdit}
						<ContextMenu.Separator />
						{#if onRename}
							<ContextMenu.Item class="gap-2" onclick={() => onRename(entry)}>
								<Pencil class="size-4 stroke-2" />
								{locale.language === 'id' ? 'Ubah nama' : 'Rename'}
							</ContextMenu.Item>
						{/if}
						{#if onDelete}
							<ContextMenu.Item variant="destructive" class="gap-2" onclick={() => onDelete(entry)}>
								<Trash2 class="size-4 stroke-2" />
								{locale.language === 'id' ? 'Hapus' : 'Delete'}
							</ContextMenu.Item>
						{/if}
					{/if}
				</ContextMenu.Content>
			</ContextMenu.Root>
		{/each}
	</DriveFileGrid>
{:else}
	<div
		class="overflow-hidden rounded-xl border border-subtle bg-surface"
		role="listbox"
		aria-label={locale.language === 'id' ? 'Berkas dibagikan' : 'Shared files'}
	>
		{#each items as entry (entry.id)}
			{@const isFolder = entry.kind === 'folder'}
			<ContextMenu.Root>
				<ContextMenu.Trigger class="block w-full">
					<DriveListRow
						selected={selectedId === entry.id}
						onOpen={() => onOpen(entry)}
						onSelect={() => (selectedId = entry.id)}
					>
						{#snippet leading()}
							<FileKindIcon kind={entry.kind} />
						{/snippet}
						{#snippet meta()}
							<p class="truncate text-sm font-medium text-main">{entry.name}</p>
							<p class="text-xs text-muted">
								{#if isFolder}
									{locale.language === 'id' ? 'Folder' : 'Folder'} · {locale.formatDate(
										entry.modifiedAt
									)}
								{:else}
									{formatSize(entry.sizeBytes)} · {locale.formatDate(entry.modifiedAt)}
								{/if}
							</p>
						{/snippet}
						{#snippet trailing()}
							<DropdownMenu.Root>
								<DropdownMenu.Trigger>
									{#snippet child({ props })}
										<Button
											{...props}
											type="button"
											variant="ghost"
											size="icon"
											class="bg-surface text-muted hover:bg-primary hover:text-white aria-expanded:bg-primary aria-expanded:text-white"
											aria-label={locale.language === 'id' ? 'Aksi berkas' : 'File actions'}
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
											{locale.language === 'id' ? 'Buka folder' : 'Open folder'}
										</DropdownMenu.Item>
									{:else}
										<DropdownMenu.Item class="gap-2" onclick={() => onOpen(entry)}>
											<Eye class="size-4 stroke-2" />
											{locale.language === 'id' ? 'Pratinjau' : 'Preview'}
										</DropdownMenu.Item>
									{/if}
									{#if onDownload}
										<DropdownMenu.Item class="gap-2" onclick={() => onDownload(entry)}>
											<Download class="size-4 stroke-2" />
											{locale.language === 'id' ? 'Unduh' : 'Download'}
										</DropdownMenu.Item>
									{/if}
									{#if canEdit}
										<DropdownMenu.Separator />
										{#if onRename}
											<DropdownMenu.Item class="gap-2" onclick={() => onRename(entry)}>
												<Pencil class="size-4 stroke-2" />
												{locale.language === 'id' ? 'Ubah nama' : 'Rename'}
											</DropdownMenu.Item>
										{/if}
										{#if onDelete}
											<DropdownMenu.Item
												variant="destructive"
												class="gap-2"
												onclick={() => onDelete(entry)}
											>
												<Trash2 class="size-4 stroke-2" />
												{locale.language === 'id' ? 'Hapus' : 'Delete'}
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
							{locale.language === 'id' ? 'Buka folder' : 'Open folder'}
						</ContextMenu.Item>
					{:else}
						<ContextMenu.Item class="gap-2" onclick={() => onOpen(entry)}>
							<Eye class="size-4 stroke-2" />
							{locale.language === 'id' ? 'Pratinjau' : 'Preview'}
						</ContextMenu.Item>
					{/if}
					{#if onDownload}
						<ContextMenu.Item class="gap-2" onclick={() => onDownload(entry)}>
							<Download class="size-4 stroke-2" />
							{locale.language === 'id' ? 'Unduh' : 'Download'}
						</ContextMenu.Item>
					{/if}
					{#if canEdit}
						<ContextMenu.Separator />
						{#if onRename}
							<ContextMenu.Item class="gap-2" onclick={() => onRename(entry)}>
								<Pencil class="size-4 stroke-2" />
								{locale.language === 'id' ? 'Ubah nama' : 'Rename'}
							</ContextMenu.Item>
						{/if}
						{#if onDelete}
							<ContextMenu.Item variant="destructive" class="gap-2" onclick={() => onDelete(entry)}>
								<Trash2 class="size-4 stroke-2" />
								{locale.language === 'id' ? 'Hapus' : 'Delete'}
							</ContextMenu.Item>
						{/if}
					{/if}
				</ContextMenu.Content>
			</ContextMenu.Root>
		{/each}
	</div>
{/if}
