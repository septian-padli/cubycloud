<script lang="ts">
	import { locale } from '$lib/locale.svelte.js';
	import type { CategoryUsage } from '$lib/types/drive.js';
	import { Progress } from '$lib/components/ui/progress/index.js';
	import * as Card from '$lib/components/ui/card/index.js';
	const tx = (id: string, en: string) => (locale.language === 'id' ? id : en);

	let {
		categories
	}: {
		categories: CategoryUsage[];
	} = $props();

	const total = $derived(categories.reduce((sum, c) => sum + c.usedGb, 0));
	function categoryLabel(cat: CategoryUsage): string {
		if (cat.id === 'image') return tx('Gambar', 'Images');
		if (cat.id === 'document') return tx('Dokumen', 'Documents');
		if (cat.id === 'archive') return tx('Arsip', 'Archives');
		if (cat.id === 'trash') return tx('Sampah', 'Trash');
		if (cat.id === 'other') return tx('Lainnya', 'Other');
		return cat.label;
	}
</script>

<section class="space-y-3">
	<h2 class="text-sm font-semibold text-main">
		{tx('Pemakaian per kategori', 'Usage by category')}
	</h2>
	<Card.Root class="rounded-xl border-subtle bg-surface shadow-none">
		<Card.Content class="space-y-4">
			{#each categories as cat (cat.id)}
				{@const ratio = total > 0 ? (cat.usedGb / total) * 100 : 0}
				{@const nearFull = ratio >= 80}
				{@const empty = cat.usedGb <= 0}
				<div class="space-y-1.5">
					<div class="flex items-center justify-between gap-2 text-sm">
						<span class="font-medium text-main">{categoryLabel(cat)}</span>
						<span class="text-muted">
							{#if empty}
								{tx('0 GB · belum digunakan', '0 GB · unused')}
							{:else}
								{cat.usedGb.toFixed(1).replace('.', locale.language === 'id' ? ',' : '.')} GB · {Math.round(ratio)}%
							{/if}
						</span>
					</div>
					<!-- shadcn Progress: hijau < 80%, terakota ≥ 80%; track muted = placeholder -->
					<Progress
						value={ratio}
						max={100}
						aria-label={tx(`Pemakaian ${categoryLabel(cat)}`, `${categoryLabel(cat)} usage`)}
						indicatorClass={nearFull ? 'bg-secondary-accent' : 'bg-primary'}
					/>
				</div>
			{/each}
			<p class="text-xs text-muted">
				{tx(
					`Total kategori (My Storage): ${total.toFixed(1).replace('.', ',')} GB. Berkas Organization tidak dihitung. Bar kosong menampilkan track placeholder; isi hijau saat aman, terakota jika ≥ 80%.`,
					`Category total (My Storage): ${total.toFixed(1)} GB. Organization files are excluded. Empty bars show a placeholder track; green indicates normal usage, terracotta indicates ≥ 80%.`
				)}
			</p>
		</Card.Content>
	</Card.Root>
</section>
