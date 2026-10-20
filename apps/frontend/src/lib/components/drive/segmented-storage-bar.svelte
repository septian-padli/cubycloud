<script lang="ts">
	import * as Popover from '$lib/components/ui/popover/index.js';
	import { cn } from '$lib/utils.js';

	export interface StorageCategoryItem {
		id: string;
		label: string;
		usedGb: number;
		description?: string;
		color?: string;
	}

	let {
		categories,
		usedGb: rawUsedGb,
		limitGb = null,
		showSummary = true,
		showLegend = true,
		class: className
	}: {
		categories: StorageCategoryItem[];
		usedGb?: number;
		limitGb?: number | null;
		showSummary?: boolean;
		showLegend?: boolean;
		class?: string;
	} = $props();

	let openId = $state<string | null>(null);
	let closeTimer: ReturnType<typeof setTimeout> | null = null;

	const totalCategoryUsed = $derived(categories.reduce((acc, c) => acc + c.usedGb, 0));
	const effectiveUsedGb = $derived(rawUsedGb ?? totalCategoryUsed);
	const unlimited = $derived(limitGb == null || limitGb <= 0);

	const DEFAULT_COLORS: Record<string, string> = {
		video: 'bg-primary',
		image: 'bg-secondary',
		document: 'bg-secondary-accent',
		archive: 'bg-primary-accent',
		other: 'bg-muted-foreground',
		trash: 'bg-danger'
	};
	const PALETTE_FALLBACKS = [
		'bg-primary',
		'bg-secondary',
		'bg-secondary-accent',
		'bg-primary-accent',
		'bg-danger',
		'bg-muted-foreground'
	];

	function getCategoryColor(cat: StorageCategoryItem, index: number): string {
		if (cat.color) return cat.color;
		return DEFAULT_COLORS[cat.id] ?? PALETTE_FALLBACKS[index % PALETTE_FALLBACKS.length];
	}

	function formatStorage(valGb: number): string {
		if (valGb <= 0) return '0 GB';
		if (valGb >= 1024) {
			const tb = valGb / 1024;
			const formatted = Number.isInteger(tb) ? String(tb) : tb.toFixed(2).replace('.', ',');
			return `${formatted} TB`;
		}
		if (valGb < 0.1) {
			const mb = Math.round(valGb * 1024);
			return `${mb} MB`;
		}
		const formatted = Number.isInteger(valGb)
			? String(valGb)
			: valGb.toFixed(valGb < 10 ? 1 : 2).replace('.', ',');
		return `${formatted} GB`;
	}

	const usedLabel = $derived(formatStorage(effectiveUsedGb));
	const limitLabel = $derived(limitGb != null ? formatStorage(limitGb) : '');

	const baseLimit = $derived(
		unlimited ? (effectiveUsedGb > 0 ? effectiveUsedGb : 1) : (limitGb ?? 1)
	);

	const totalFilledPct = $derived(
		unlimited
			? effectiveUsedGb > 0
				? 100
				: 0
			: Math.min(100, Math.max(0, baseLimit > 0 ? (effectiveUsedGb / baseLimit) * 100 : 0))
	);
	const visibleCategories = $derived.by(() => {
		const scaleFactor =
			rawUsedGb != null && totalCategoryUsed > 0 ? effectiveUsedGb / totalCategoryUsed : 1;

		return categories
			.map((cat, index) => {
				const displayUsedGb = cat.usedGb * scaleFactor;
				const rawQuotaPct = baseLimit > 0 ? (displayUsedGb / baseLimit) * 100 : 0;
				const quotaPctLabel =
					rawQuotaPct < 0.1 && rawQuotaPct > 0
						? '< 0,1'
						: (Math.round(rawQuotaPct * 10) / 10).toString().replace('.', ',');
				return {
					cat,
					index,
					displayUsedGb,
					quotaPctLabel
				};
			})
			.filter((item) => item.cat.usedGb > 0);
	});

	function openCategory(id: string) {
		if (closeTimer) {
			clearTimeout(closeTimer);
			closeTimer = null;
		}
		openId = id;
	}

	function closeCategory() {
		openId = null;
	}

	function scheduleClose(id: string) {
		if (closeTimer) clearTimeout(closeTimer);
		closeTimer = setTimeout(() => {
			if (openId === id) openId = null;
		}, 150);
	}

	function toggleCategory(id: string) {
		if (closeTimer) {
			clearTimeout(closeTimer);
			closeTimer = null;
		}
		openId = id;
	}
</script>

<div class={cn('space-y-4', className)}>
	{#if showSummary}
		<div class="flex flex-wrap items-baseline gap-2">
			<span class="text-3xl font-semibold tracking-tight text-main">
				{usedLabel}
			</span>
			{#if unlimited}
				<span class="text-sm font-normal text-muted sm:text-base"> telah digunakan </span>
			{:else}
				<span class="text-sm font-normal text-muted sm:text-base">
					dari {limitLabel} telah digunakan
				</span>
			{/if}
		</div>
	{/if}

	<!-- Segmented Bar Track: spans 100% of container, cleanly rounded and clipped -->
	<div
		class="relative flex h-3.5 w-full items-center overflow-hidden rounded-full bg-black/10 dark:bg-white/10"
		role="img"
		aria-label="Pemakaian kuota penyimpanan My Storage"
	>
		{#if totalFilledPct > 0}
			<div
				class="flex h-full items-center overflow-hidden rounded-full"
				style="width: {totalFilledPct}%; max-width: 100%;"
			>
				{#each visibleCategories as { cat, index, displayUsedGb, quotaPctLabel } (cat.id)}
					<Popover.Root
						open={openId === cat.id}
						onOpenChange={(next) => {
							if (next) openCategory(cat.id);
							else if (openId === cat.id) closeCategory();
						}}
					>
						<Popover.Trigger>
							{#snippet child({ props })}
								<button
									{...props}
									type="button"
									class={cn(
										'h-full transition-opacity duration-150 hover:opacity-90 focus-visible:ring-2 focus-visible:ring-primary focus-visible:outline-none',
										getCategoryColor(cat, index),
										openId === cat.id && 'z-10 brightness-110'
									)}
									style="flex: 0 0 {(cat.usedGb / totalCategoryUsed) * 100}%;"
									aria-label="{cat.label}: {formatStorage(displayUsedGb)}"
									onmouseenter={() => openCategory(cat.id)}
									onmouseleave={() => scheduleClose(cat.id)}
									onclick={() => toggleCategory(cat.id)}
								></button>
							{/snippet}
						</Popover.Trigger>
						<Popover.Content
							side="top"
							sideOffset={10}
							align="center"
							class="z-50 w-60 space-y-2 rounded-xl border border-subtle bg-surface p-3.5 text-sm shadow-md"
							onmouseenter={() => openCategory(cat.id)}
							onmouseleave={() => scheduleClose(cat.id)}
						>
							<!-- Title -->
							<div class="flex items-center gap-2">
								<span
									class={cn('size-2.5 shrink-0 rounded-full', getCategoryColor(cat, index))}
									aria-hidden="true"
								></span>
								<span class="font-semibold text-main">{cat.label}</span>
							</div>

							<!-- Size terpakai (besar & align center) -->
							<div class="py-1 text-center">
								<span class="text-3xl font-bold tracking-tight text-main">
									{formatStorage(displayUsedGb)}
								</span>
							</div>

							<!-- Proporsi pemakaian -->
							<div
								class="flex items-center justify-between border-t border-subtle pt-2 text-xs text-muted"
							>
								<span>{unlimited ? 'Proporsi pemakaian' : 'Proporsi kuota'}</span>
								<span class="font-semibold text-main">
									{quotaPctLabel}%
								</span>
							</div>
						</Popover.Content>
					</Popover.Root>
				{/each}
			</div>
		{/if}
	</div>

	{#if showLegend}
		<!-- Legend Row -->
		<div class="flex flex-wrap items-center gap-x-6 gap-y-2.5 pt-1">
			{#each categories.filter((c) => c.usedGb > 0 || c.id !== 'other') as cat, index (cat.id)}
				<button
					type="button"
					class={cn(
						'flex items-center gap-2 rounded text-xs text-main transition-colors hover:text-primary focus-visible:ring-1 focus-visible:ring-primary focus-visible:outline-none sm:text-sm',
						openId === cat.id && 'font-medium text-primary'
					)}
					onmouseenter={() => openCategory(cat.id)}
					onmouseleave={() => scheduleClose(cat.id)}
					onclick={() => toggleCategory(cat.id)}
				>
					<span
						class={cn('size-2.5 shrink-0 rounded-full', getCategoryColor(cat, index))}
						aria-hidden="true"
					></span>
					<span>{cat.label}</span>
				</button>
			{/each}
		</div>
	{/if}
</div>
