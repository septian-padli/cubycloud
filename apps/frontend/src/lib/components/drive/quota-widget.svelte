<script lang="ts">
	import { Progress } from '$lib/components/ui/progress/index.js';
	import { cn } from '$lib/utils.js';

	let {
		usedGb,
		limitGb = null,
		showPercent = false,
		class: className
	}: {
		usedGb: number;
		limitGb?: number | null;
		/** Tampilkan label persen + status di bawah bar (halaman Penyimpanan). */
		showPercent?: boolean;
		class?: string;
	} = $props();

	// ponytail: SITEMAP — Super Admin always Unlimited (pass limitGb=null when auth is wired).
	// UI mock keeps limitGb set so the limited progress bar can be reviewed.
	const unlimited = $derived(limitGb == null);
	const ratio = $derived(
		!unlimited && limitGb != null && limitGb > 0 ? usedGb / limitGb : 0
	);
	const pct = $derived(Math.min(Math.round(ratio * 100), 100));
	const nearFull = $derived(ratio >= 0.8);
	const empty = $derived(ratio <= 0);
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
	const statusLabel = $derived(
		empty ? 'Kosong' : nearFull ? (pct >= 100 ? 'Penuh' : 'Hampir penuh') : 'Tersedia'
	);
</script>

<div class={cn('border-t border-subtle px-4 py-3', className)}>
	<p class="text-xs font-medium text-muted">My Storage</p>
	{#if unlimited}
		<p class="mt-1 text-sm font-medium text-main">{usedLabel} GB terpakai</p>
	{:else}
		<div class="mt-1 flex items-baseline justify-between gap-2">
			<p class="text-sm font-medium text-main">{usedLabel} / {limitLabel} GB</p>
			{#if showPercent}
				<p class="text-xs font-medium text-muted">{pct}%</p>
			{/if}
		</div>
		<div class="mt-2">
			<!-- hijau (primary) saat kosong/rendah; terakota (secondary-accent) ≥ 80% -->
			<Progress
				value={pct}
				max={100}
				aria-label="Pemakaian kuota My Storage"
				indicatorClass={nearFull ? 'bg-secondary-accent' : 'bg-primary'}
			/>
		</div>
		{#if showPercent}
			<p class="mt-1.5 text-xs text-muted">
				{statusLabel}
				{#if empty}
					— track abu-abu adalah placeholder sebelum ada pemakaian.
				{:else if nearFull}
					— pertimbangkan membersihkan Trash atau berkas besar.
				{/if}
			</p>
		{/if}
	{/if}
</div>
