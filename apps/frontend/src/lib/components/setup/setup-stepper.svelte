<script lang="ts">
	import { Progress } from '$lib/components/ui/progress/index.js';
	import { cn } from '$lib/utils.js';
	import { SETUP_STEPS, type SetupStepId } from '$lib/types/setup.js';
	import { Check } from '@lucide/svelte';
	import { locale } from '$lib/locale.svelte.js';
	const tx = (id: string, en: string) => (locale.language === 'id' ? id : en);
	const stepText = (id: number, description = false) => {
		const labels: Record<number, [string, string, string, string]> = {
			1: ['Akun Admin', 'Administrator Account', 'Buat akun administrator pertama.', 'Create the first administrator account.'],
			2: ['Google (Opsional)', 'Google (Optional)', 'Konfigurasikan Google Sign-In.', 'Configure Google Sign-In.'],
			3: ['Penyimpanan', 'Storage', 'Pilih node penyimpanan lokal.', 'Choose local storage nodes.'],
			4: ['Cloud', 'Cloud', 'Hubungkan penyimpanan cloud.', 'Connect cloud storage.'],
			5: ['Strategi', 'Strategy', 'Pilih strategi penempatan berkas.', 'Choose a file placement strategy.'],
			6: ['Recovery Key', 'Recovery Key', 'Simpan kunci pemulihan Anda.', 'Save your recovery key.']
		};
		const row = labels[id];
		return row ? tx(row[description ? 2 : 0], row[description ? 3 : 1]) : '';
	};
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
		<p class="text-xs font-medium text-muted">{tx('Progres setup', 'Setup progress')}</p>
		<p class="mt-1 text-sm font-medium text-main">
			{tx('Langkah', 'Step')}
			{currentStep}
			{tx('dari', 'of')}
			{SETUP_STEPS.length}
		</p>
		<Progress class="mt-3" value={progressValue} max={100} />

		<nav class="mt-6 flex flex-col gap-1" aria-label={tx('Langkah setup', 'Setup steps')}>
			{#each SETUP_STEPS as step (step.id)}
				{@const active = step.id === currentStep}
				{@const done = step.id < currentStep}
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
							{stepText(step.id)}
						</p>
						<p class="mt-0.5 text-xs text-muted">{stepText(step.id, true)}</p>
					</div>
				</button>
			{/each}
		</nav>
	</div>
</aside>

<!-- Mobile stepper -->
<div class="mb-4 md:hidden">
	<div class="rounded-xl border border-subtle bg-surface px-4 py-3">
		<p class="text-xs font-medium text-muted">
			{tx('Langkah', 'Step')}
			{currentStep}/{SETUP_STEPS.length}
		</p>
		<p class="truncate text-sm font-medium text-main">{stepText(currentStep)}</p>
		<Progress class="mt-3" value={progressValue} max={100} />
	</div>
</div>
