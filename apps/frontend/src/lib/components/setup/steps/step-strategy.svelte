<script lang="ts">
	import type { PlacementStrategy } from '$lib/types/setup.js';
	import { STRATEGY_OPTIONS } from '$lib/types/setup.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import * as Alert from '$lib/components/ui/alert/index.js';
	import * as RadioGroup from '$lib/components/ui/radio-group/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { Info } from '@lucide/svelte';
	import { cn } from '$lib/utils.js';

	let {
		strategy = $bindable()
	}: {
		strategy: PlacementStrategy;
	} = $props();
</script>

<Card.Root class="rounded-xl border-subtle bg-surface shadow-none">
	<Card.Header>
		<Card.Title class="text-lg font-semibold text-main">Strategi Penempatan</Card.Title>
		<Card.Description class="text-sm text-muted">
			Pilih bagaimana berkas baru dialokasikan ke node penyimpanan aktif.
		</Card.Description>
	</Card.Header>
	<Card.Content class="space-y-4">
		<RadioGroup.Root bind:value={strategy} class="gap-3">
			{#each STRATEGY_OPTIONS as option (option.value)}
				{@const selected = strategy === option.value}
				<label
					class={cn(
						'flex cursor-pointer items-start gap-3 rounded-xl border border-subtle bg-app px-4 py-4 transition-colors',
						selected && 'border-primary bg-primary/5'
					)}
				>
					<RadioGroup.Item value={option.value} id="strategy-{option.value}" class="mt-0.5" />
					<div class="min-w-0">
						<Label for="strategy-{option.value}" class="cursor-pointer text-sm font-medium text-main">
							{option.label}
						</Label>
						<p class="mt-1 text-xs leading-normal text-muted">{option.description}</p>
					</div>
				</label>
			{/each}
		</RadioGroup.Root>

		<Alert.Root class="border border-subtle bg-surface">
			<Info class="size-4 stroke-2 text-primary" />
			<Alert.Title class="text-main">Dapat diubah nanti</Alert.Title>
			<Alert.Description>
				Strategi default bisa diganti kapan saja di Admin Settings tanpa menjalankan ulang wizard.
			</Alert.Description>
		</Alert.Root>
	</Card.Content>
</Card.Root>
