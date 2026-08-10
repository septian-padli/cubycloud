<script lang="ts">
	import type { QuickAccessItem } from '$lib/types/drive.js';
	import { driveView } from '$lib/drive-view.svelte.js';
	import QuickAccessCard from './quick-access-card.svelte';
	import FileKindIcon from './explorer/file-kind-icon.svelte';
	import DriveListRow from './drive-list-row.svelte';
	import DriveFileGrid from './drive-file-grid.svelte';

	let {
		items
	}: {
		items: QuickAccessItem[];
	} = $props();
</script>

<section class="space-y-3">
	<h2 class="text-sm font-semibold text-main">Akses Cepat</h2>
	{#if driveView.mode === 'list'}
		<div
			class="overflow-hidden rounded-xl border border-subtle bg-surface"
			role="listbox"
			aria-label="Akses cepat"
		>
			{#each items as item (item.id)}
				<DriveListRow>
					{#snippet leading()}
						<FileKindIcon kind={item.kind} />
					{/snippet}
					{#snippet meta()}
						<p class="truncate text-sm font-medium text-main">{item.name}</p>
						<p class="text-xs text-muted">{item.sizeLabel} · {item.openedAgo}</p>
					{/snippet}
				</DriveListRow>
			{/each}
		</div>
	{:else}
		<DriveFileGrid label="Akses cepat">
			{#each items as item (item.id)}
				<QuickAccessCard
					variant={item.kind}
					name={item.name}
					sizeLabel={item.sizeLabel}
					openedAgo={item.openedAgo}
				/>
			{/each}
		</DriveFileGrid>
	{/if}
</section>
