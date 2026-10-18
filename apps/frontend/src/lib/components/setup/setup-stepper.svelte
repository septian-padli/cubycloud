<script lang="ts">
	import { Progress } from '$lib/components/ui/progress/index.js';
	import { cn } from '$lib/utils.js';
	import { SETUP_STEPS, type SetupStepId } from '$lib/types/setup.js';
	import { Check } from '@lucide/svelte';

	let {
		currentStep,
		onSelectStep
	}: {
		currentStep: SetupStepId;
		onSelectStep: (step: SetupStepId) => void;
	} = $props();

	const progressValue = $derived(((currentStep - 1) / SETUP_STEPS.length) * 100);
</script>

<aside class="hidden w-60 shrink-0 md:block">
	<div class="rounded-xl border border-subtle bg-surface p-5">
		<p class="text-xs font-medium text-muted">Progres setup</p>
		<p class="mt-1 text-sm font-medium text-main">
			Langkah {currentStep} dari {SETUP_STEPS.length}
		</p>
		<Progress class="mt-3" value={progressValue} max={100} />

		<nav class="mt-6 flex flex-col gap-1" aria-label="Langkah setup">
			{#each SETUP_STEPS as step (step.id)}
				{@const done = step.id < currentStep}
				{@const active = step.id === currentStep}
				<button
					type="button"
					disabled={!done}
					aria-current={active ? 'step' : undefined}
					onclick={() => onSelectStep(step.id)}
					class={cn(
						'flex w-full items-start gap-3 rounded-lg px-3 py-2.5 text-left transition-colors disabled:cursor-default',
						active && 'bg-primary/10',
						done && 'cursor-pointer hover:bg-primary/5',
						!active && !done && 'opacity-70'
					)}
				>
					<span
						class={cn(
							'mt-0.5 flex size-6 shrink-0 items-center justify-center rounded-full text-xs font-medium',
							active && 'bg-primary text-white dark:text-app',
							done && 'bg-primary/20 text-primary',
							!active && !done && 'border border-subtle bg-surface text-main'
						)}
					>
						{#if done}
							<Check class="size-3.5 stroke-2" />
						{:else}
							{step.id}
						{/if}
					</span>
					<div class="min-w-0">
						<p class={cn('text-sm font-medium', active ? 'text-main' : 'text-muted')}>
							{step.title}
						</p>
						<p class="mt-0.5 text-xs text-muted">{step.description}</p>
					</div>
				</button>
			{/each}
		</nav>
	</div>
</aside>

<!-- Mobile stepper -->
<div class="mb-4 md:hidden">
	<div class="rounded-xl border border-subtle bg-surface px-4 py-3">
		<div class="flex items-center justify-between gap-3">
			<p class="text-xs font-medium text-muted">
				Langkah {currentStep}/{SETUP_STEPS.length}
			</p>
			<p class="truncate text-sm font-medium text-main">
				{SETUP_STEPS[currentStep - 1]?.title}
			</p>
		</div>
		<Progress class="mt-3" value={progressValue} max={100} />
	</div>
</div>
