<script lang="ts">
	import { CheckCircle2, FolderArchive, HardDrive } from '@lucide/svelte';
	import type { AdminStorageStats } from '$lib/types/admin.js';
	import SegmentedStorageBar, {
		type StorageCategoryItem
	} from '$lib/components/drive/segmented-storage-bar.svelte';
	import { formatSize } from '$lib/mocks/file-tree.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import { cn } from '$lib/utils.js';

	let {
		stats,
		loading = false,
		class: className
	}: {
		stats: AdminStorageStats | null;
		loading?: boolean;
		class?: string;
	} = $props();

	const totalCap = $derived(stats?.total_capacity_bytes ?? 0);
	const totalUsed = $derived(stats?.total_used_bytes ?? 0);
	const freeBytes = $derived(stats?.free_bytes ?? 0);
	const nodesCount = $derived(stats?.nodes_count ?? 0);

	const usedPercent = $derived(
		totalCap > 0 ? Math.min(100, Math.max(0, (totalUsed / totalCap) * 100)) : 0
	);
	const usedLabel = $derived(formatSize(totalUsed));
	const capLabel = $derived(formatSize(totalCap));
	const freeLabel = $derived(formatSize(freeBytes));

	const categories = $derived<StorageCategoryItem[]>(
		(stats?.categories ?? []).map((c) => ({
			id: c.id,
			label: c.label,
			usedGb: c.used_gb,
			description: `${c.label}: ${c.used_gb < 0.1 && c.used_gb > 0 ? '< 0,1' : c.used_gb.toFixed(2)} GB`
		}))
	);
</script>

<section class={cn('space-y-3', className)}>
	<h2 class="text-sm font-semibold text-main">Detail kuota & pemakaian sistem</h2>
	<Card.Root class="rounded-xl border-subtle bg-surface shadow-none">
		<Card.Content class="space-y-6 p-5">
			{#if loading && !stats}
				<div class="space-y-4 animate-pulse">
					<div class="h-8 w-48 rounded bg-muted/20"></div>
					<div class="h-3.5 w-full rounded-full bg-muted/20"></div>
					<div class="grid grid-cols-1 gap-3 pt-2 sm:grid-cols-3">
						<div class="h-20 rounded-lg bg-muted/10"></div>
						<div class="h-20 rounded-lg bg-muted/10"></div>
						<div class="h-20 rounded-lg bg-muted/10"></div>
					</div>
				</div>
			{:else if stats}
				<div class="space-y-6">
					<!-- Segmented Storage Bar dengan kategori, popover, dan legend -->
					<SegmentedStorageBar
						{categories}
						usedGb={stats.total_used_gb}
						limitGb={stats.total_capacity_gb}
						showSummary={true}
						showLegend={true}
					/>

					<!-- Stat cards -->
					<div class="grid grid-cols-1 gap-3 pt-1 sm:grid-cols-3">
						<div class="flex items-center gap-3 rounded-lg border border-subtle bg-app/50 p-3.5">
							<div
								class="flex size-10 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary"
							>
								<HardDrive class="size-5" />
							</div>
							<div class="min-w-0">
								<p class="text-xs font-medium text-muted">Total Kapasitas Node</p>
								<p class="truncate text-base font-semibold text-main">{capLabel}</p>
								<p class="text-[11px] text-muted">{nodesCount} node terdaftar</p>
							</div>
						</div>

						<div class="flex items-center gap-3 rounded-lg border border-subtle bg-app/50 p-3.5">
							<div
								class="flex size-10 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary"
							>
								<FolderArchive class="size-5" />
							</div>
							<div class="min-w-0">
								<p class="text-xs font-medium text-muted">Total Pemakaian Berkas</p>
								<p class="truncate text-base font-semibold text-main">{usedLabel}</p>
								<p class="text-[11px] text-muted">{usedPercent.toFixed(1)}% dari kapasitas</p>
							</div>
						</div>

						<div class="flex items-center gap-3 rounded-lg border border-subtle bg-app/50 p-3.5">
							<div
								class="flex size-10 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary"
							>
								<CheckCircle2 class="size-5" />
							</div>
							<div class="min-w-0">
								<p class="text-xs font-medium text-muted">Sisa Kapasitas Bebas</p>
								<p class="truncate text-base font-semibold text-main">{freeLabel}</p>
								<p class="text-[11px] text-muted">{(100 - usedPercent).toFixed(1)}% tersedia</p>
							</div>
						</div>
					</div>
				</div>

				<div class="border-t border-subtle pt-3">
					<p class="text-xs text-muted">
						Kapasitas dihitung dari akumulasi seluruh storage node yang terdaftar. Pemakaian
						mencakup seluruh berkas pengguna.
					</p>
				</div>
			{:else}
				<p class="text-sm text-muted">Data statistik penyimpanan belum tersedia.</p>
			{/if}
		</Card.Content>
	</Card.Root>
</section>
