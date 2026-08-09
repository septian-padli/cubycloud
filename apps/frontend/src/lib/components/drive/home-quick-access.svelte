<script lang="ts">
	import type { QuickAccessItem } from '$lib/types/drive.js';
	import QuickAccessCard from './quick-access-card.svelte';

	let {
		items
	}: {
		items: QuickAccessItem[];
	} = $props();
</script>

<section class="space-y-3">
	<h2 class="text-sm font-semibold text-main">Akses Cepat</h2>
	<div class="grid grid-cols-2 gap-3 md:grid-cols-3 lg:grid-cols-4">
		{#each items as item (item.id)}
			<QuickAccessCard
				variant={item.kind}
				name={item.name}
				sizeLabel={item.sizeLabel}
				openedAgo={item.openedAgo}
			/>
		{/each}
	</div>
</section>
