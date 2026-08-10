<script lang="ts">
	import type { StarredFileItem } from '$lib/types/drive.js';
	import { formatSize } from '$lib/mocks/file-tree.js';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu/index.js';
	import * as ContextMenu from '$lib/components/ui/context-menu/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Eye, MoreVertical, Share2, Star } from '@lucide/svelte';
	import DriveListRow from './drive-list-row.svelte';
	import SourceBadge from './source-badge.svelte';
	import FileKindIcon from './explorer/file-kind-icon.svelte';

	let {
		entry,
		selected = false,
		onSelect,
		onPreview,
		onShare,
		onUnstar
	}: {
		entry: StarredFileItem;
		selected?: boolean;
		onSelect: (entry: StarredFileItem) => void;
		onPreview: (entry: StarredFileItem) => void;
		onShare: (entry: StarredFileItem) => void;
		onUnstar: (entry: StarredFileItem) => void;
	} = $props();
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
				<p class="truncate text-sm font-medium text-main">
					{entry.name}
					<Star
						class="ms-1 inline size-3.5 fill-secondary stroke-2 text-secondary"
						aria-label="Berbintang"
					/>
				</p>
				<p class="text-xs text-muted">
					{#if entry.kind === 'folder'}
						Folder · {entry.modifiedAt}
					{:else}
						{formatSize(entry.sizeBytes)} · {entry.modifiedAt}
					{/if}
				</p>
			{/snippet}
			{#snippet trailing()}
				<SourceBadge source={entry.source} label={entry.sourceLabel} />
				<DropdownMenu.Root>
					<DropdownMenu.Trigger>
						{#snippet child({ props })}
							<Button
								{...props}
								type="button"
								variant="ghost"
								size="icon-sm"
								class="bg-surface text-muted hover:bg-primary hover:text-white aria-expanded:bg-primary aria-expanded:text-white"
								aria-label="Aksi berkas"
								onclick={(e: MouseEvent) => e.stopPropagation()}
							>
								<MoreVertical class="size-4 stroke-2" />
							</Button>
						{/snippet}
					</DropdownMenu.Trigger>
					<DropdownMenu.Content align="end" class="min-w-48">
						{#if entry.kind !== 'folder'}
							<DropdownMenu.Item class="gap-2" onclick={() => onPreview(entry)}>
								<Eye class="size-4 stroke-2" />
								Pratinjau
							</DropdownMenu.Item>
							<DropdownMenu.Item class="gap-2" onclick={() => onShare(entry)}>
								<Share2 class="size-4 stroke-2" />
								Bagikan
							</DropdownMenu.Item>
						{/if}
						<DropdownMenu.Item class="gap-2" onclick={() => onUnstar(entry)}>
							<Star class="size-4 stroke-2" />
							Hapus bintang
						</DropdownMenu.Item>
					</DropdownMenu.Content>
				</DropdownMenu.Root>
			{/snippet}
		</DriveListRow>
	</ContextMenu.Trigger>
	<ContextMenu.Content class="min-w-48">
		{#if entry.kind !== 'folder'}
			<ContextMenu.Item class="gap-2" onclick={() => onPreview(entry)}>
				<Eye class="size-4 stroke-2" />
				Pratinjau
			</ContextMenu.Item>
			<ContextMenu.Item class="gap-2" onclick={() => onShare(entry)}>
				<Share2 class="size-4 stroke-2" />
				Bagikan
			</ContextMenu.Item>
		{/if}
		<ContextMenu.Item class="gap-2" onclick={() => onUnstar(entry)}>
			<Star class="size-4 stroke-2" />
			Hapus bintang
		</ContextMenu.Item>
	</ContextMenu.Content>
</ContextMenu.Root>
