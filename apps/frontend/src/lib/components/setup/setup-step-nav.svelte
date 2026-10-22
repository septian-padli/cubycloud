<script lang="ts">
	import { Button } from '$lib/components/ui/button/index.js';
	import { ArrowLeft, ArrowRight } from '@lucide/svelte';
	import { locale } from '$lib/locale.svelte.js';
	const tx = (id: string, en: string) => (locale.language === 'id' ? id : en);

	let {
		canBack = true,
		canNext = true,
		isLast = false,
		showSkip = false,
		nextLabel = '',
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

<div class="flex flex-wrap items-center justify-between gap-3">
	<div>
		{#if canBack}
			<Button variant="secondary" class="gap-1.5" onclick={onBack}>
				<ArrowLeft class="size-4 stroke-2" />
				{tx('Kembali', 'Back')}
			</Button>
		{/if}
	</div>

	<div class="flex flex-wrap items-center gap-2">
		{#if showSkip && onSkip}
			<Button variant="secondary" onclick={onSkip}>{tx('Lewati', 'Skip')}</Button>
		{/if}
		<Button class="gap-1.5" disabled={!canNext} onclick={onNext}>
			{isLast ? nextLabel || tx('Selesaikan', 'Finish') : tx('Lanjut', 'Next')}
			{#if !isLast}
				<ArrowRight class="size-4 stroke-2" />
			{/if}
		</Button>
	</div>
</div>
