<script lang="ts">
	import type { CategoryUsage } from '$lib/types/drive.js';
	import { Progress } from '$lib/components/ui/progress/index.js';
	import * as Card from '$lib/components/ui/card/index.js';

	let {
		categories
	}: {
		categories: CategoryUsage[];
	} = $props();

	const total = $derived(categories.reduce((sum, c) => sum + c.usedGb, 0));
</script>

<section class="space-y-3">
	<h2 class="text-sm font-semibold text-main">Pemakaian per kategori</h2>
	<Card.Root class="rounded-xl border-subtle bg-surface shadow-none">
		<Card.Content class="space-y-4">
			{#each categories as cat (cat.id)}
				{@const ratio = total > 0 ? (cat.usedGb / total) * 100 : 0}
				{@const nearFull = ratio >= 80}
				{@const empty = cat.usedGb <= 0}
				<div class="space-y-1.5">
					<div class="flex items-center justify-between gap-2 text-sm">
						<span class="font-medium text-main">{cat.label}</span>
						<span class="text-muted">
							{#if empty}
								0 GB · placeholder
							{:else}
								{cat.usedGb.toFixed(1)} GB · {Math.round(ratio)}%
							{/if}
						</span>
					</div>
					<!-- shadcn Progress: hijau < 80%, terakota ≥ 80%; track muted = placeholder -->
					<Progress
						value={ratio}
						max={100}
						aria-label="Pemakaian {cat.label}"
						indicatorClass={nearFull ? 'bg-secondary-accent' : 'bg-primary'}
					/>
				</div>
			{/each}
			<p class="text-xs text-muted">
				Total kategori (My Storage): {total.toFixed(1)} GB. Berkas Organization tidak dihitung.
				Bar kosong menampilkan track placeholder; isi hijau saat aman, terakota jika ≥ 80%.
			</p>
		</Card.Content>
	</Card.Root>
</section>
