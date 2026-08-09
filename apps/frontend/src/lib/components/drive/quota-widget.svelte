<script lang="ts">
	import { Progress } from '$lib/components/ui/progress/index.js';
	import { cn } from '$lib/utils.js';

	let {
		usedGb,
		limitGb = null,
		class: className
	}: {
		usedGb: number;
		limitGb?: number | null;
		class?: string;
	} = $props();

	// ponytail: SITEMAP — Super Admin always Unlimited (pass limitGb=null when auth is wired).
	// UI mock keeps limitGb set so the limited progress bar can be reviewed.
	const unlimited = $derived(limitGb == null);
	const ratio = $derived(
		!unlimited && limitGb != null && limitGb > 0 ? usedGb / limitGb : 0
	);
	const nearFull = $derived(ratio >= 0.8);
	const usedLabel = $derived(
		Number.isInteger(usedGb) ? String(usedGb) : usedGb.toFixed(1)
	);
	const limitLabel = $derived(
		limitGb == null
			? ''
			: Number.isInteger(limitGb)
				? String(limitGb)
				: limitGb.toFixed(1)
	);
</script>

<div class={cn('border-t border-subtle px-4 py-3', className)}>
	<p class="text-xs font-medium text-muted">My Storage</p>
	{#if unlimited}
		<p class="mt-1 text-sm font-medium text-main">Used: {usedLabel} GB</p>
	{:else}
		<p class="mt-1 text-sm font-medium text-main">{usedLabel} / {limitLabel} GB</p>
		<div class="mt-2">
			<Progress
				value={Math.min(ratio * 100, 100)}
				max={100}
				class={cn(nearFull && '[&_[data-slot=progress-indicator]]:bg-secondary')}
			/>
		</div>
	{/if}
</div>
