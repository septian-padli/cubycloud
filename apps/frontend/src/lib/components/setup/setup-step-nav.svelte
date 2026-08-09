<script lang="ts">
	import { Button } from '$lib/components/ui/button/index.js';
	import { ArrowLeft, ArrowRight } from '@lucide/svelte';

	let {
		canBack = true,
		canNext = true,
		isLast = false,
		showSkip = false,
		nextLabel = 'Lanjut',
		onBack,
		onNext,
		onSkip
	}: {
		canBack?: boolean;
		canNext?: boolean;
		isLast?: boolean;
		showSkip?: boolean;
		nextLabel?: string;
		onBack: () => void;
		onNext: () => void;
		onSkip?: () => void;
	} = $props();
</script>

<div class="flex flex-wrap items-center justify-between gap-3 ">
	<div>
		{#if canBack}
			<Button variant="secondary" class="rounded-lg gap-1.5" onclick={onBack}>
				<ArrowLeft class="size-4 stroke-2" />
				Kembali
			</Button>
		{/if}
	</div>

	<div class="flex flex-wrap items-center gap-2">
		{#if showSkip && onSkip}
			<Button variant="secondary" class="rounded-lg" onclick={onSkip}>Lewati</Button>
		{/if}
		<Button class="rounded-lg gap-1.5" disabled={!canNext} onclick={onNext}>
			{isLast ? nextLabel : 'Lanjut'}
			{#if !isLast}
				<ArrowRight class="size-4 stroke-2" />
			{/if}
		</Button>
	</div>
</div>
