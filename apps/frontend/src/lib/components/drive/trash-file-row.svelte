<script lang="ts">
	import { locale } from '$lib/locale.svelte.js';
	import type { TrashFileItem } from '$lib/types/drive.js';
	import { formatSize } from '$lib/mocks/file-tree.js';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu/index.js';
	import * as ContextMenu from '$lib/components/ui/context-menu/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { MoreVertical, RotateCcw, Trash2 } from '@lucide/svelte';
	import DriveListRow from './drive-list-row.svelte';
	import FileKindIcon from './explorer/file-kind-icon.svelte';
	const tx = (id: string, en: string) => (locale.language === 'id' ? id : en);

	let {
		entry,
		selected = false,
		onSelect,
		onRestore,
		onPurge
	}: {
		entry: TrashFileItem;
		selected?: boolean;
		onSelect: (entry: TrashFileItem) => void;
		onRestore: (entry: TrashFileItem) => void;
		onPurge: (entry: TrashFileItem) => void;
	} = $props();
</script>

<ContextMenu.Root>
	<ContextMenu.Trigger class="block w-full">
		<DriveListRow {selected} onSelect={() => onSelect(entry)}>
			{#snippet leading()}
				<FileKindIcon kind={entry.kind} />
			{/snippet}
			{#snippet meta()}
				<p class="truncate text-sm font-medium text-main">{entry.name}</p>
				<p class="text-xs text-muted">
					{#if entry.kind === 'folder'}
						Folder · {tx('Dihapus', 'Deleted')}
						{locale.formatDate(entry.deletedAt)} · {tx('sisa', 'remaining')}
						{entry.daysRemaining}
						{tx('hari', 'days')}
					{:else}
						{formatSize(entry.sizeBytes)} · {tx('Dihapus', 'Deleted')}
						{locale.formatDate(entry.deletedAt)} · {tx('sisa', 'remaining')}
						{entry.daysRemaining}
						{tx('hari', 'days')}
					{/if}
				</p>
			{/snippet}
			{#snippet trailing()}
				{#if entry.daysRemaining <= 7}
					<span class="rounded-full bg-danger/15 px-2.5 py-1 text-xs font-medium text-danger">
						{entry.daysRemaining}h
					</span>
				{/if}
				<DropdownMenu.Root>
					<DropdownMenu.Trigger>
						{#snippet child({ props })}
							<Button
								{...props}
								type="button"
								variant="ghost"
								size="icon"
								class="bg-surface text-muted hover:bg-primary hover:text-white aria-expanded:bg-primary aria-expanded:text-white"
								aria-label={tx('Aksi trash', 'Trash actions')}
								onclick={(e: MouseEvent) => e.stopPropagation()}
							>
								<MoreVertical class="size-4 stroke-2" />
							</Button>
						{/snippet}
					</DropdownMenu.Trigger>
					<DropdownMenu.Content align="end" class="min-w-48">
						<DropdownMenu.Item class="gap-2" onclick={() => onRestore(entry)}>
							<RotateCcw class="size-4 stroke-2" />
							{tx('Pulihkan', 'Restore')}
						</DropdownMenu.Item>
						<DropdownMenu.Separator />
						<DropdownMenu.Item variant="destructive" class="gap-2" onclick={() => onPurge(entry)}>
							<Trash2 class="size-4 stroke-2" />
							{tx('Hapus permanen', 'Delete permanently')}
						</DropdownMenu.Item>
					</DropdownMenu.Content>
				</DropdownMenu.Root>
			{/snippet}
		</DriveListRow>
	</ContextMenu.Trigger>
	<ContextMenu.Content class="min-w-48">
		<ContextMenu.Item class="gap-2" onclick={() => onRestore(entry)}>
			<RotateCcw class="size-4 stroke-2" />
			{tx('Pulihkan', 'Restore')}
		</ContextMenu.Item>
		<ContextMenu.Separator />
		<ContextMenu.Item variant="destructive" class="gap-2" onclick={() => onPurge(entry)}>
			<Trash2 class="size-4 stroke-2" />
			{tx('Hapus permanen', 'Delete permanently')}
		</ContextMenu.Item>
	</ContextMenu.Content>
</ContextMenu.Root>
