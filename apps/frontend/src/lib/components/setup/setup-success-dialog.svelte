<script lang="ts">
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { PartyPopper } from '@lucide/svelte';
	import type { PlacementStrategy } from '$lib/types/setup.js';
	import { STRATEGY_OPTIONS } from '$lib/types/setup.js';

	let {
		open = $bindable(false),
		username,
		nodeCount,
		strategy,
		onContinue
	}: {
		open?: boolean;
		username: string;
		nodeCount: number;
		strategy: PlacementStrategy;
		onContinue: () => void;
	} = $props();

	const strategyLabel = $derived(
		STRATEGY_OPTIONS.find((o) => o.value === strategy)?.label ?? strategy
	);
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="rounded-xl sm:max-w-md" showCloseButton={false}>
		<Dialog.Header>
			<div class="mb-2 flex size-10 items-center justify-center rounded-lg bg-primary/10 text-primary">
				<PartyPopper class="size-5 stroke-2" />
			</div>
			<Dialog.Title class="text-lg font-semibold text-main">Setup selesai</Dialog.Title>
			<Dialog.Description class="text-sm text-muted">
				CubyCloud siap digunakan. Ringkasan konfigurasi awal:
			</Dialog.Description>
		</Dialog.Header>

		<ul class="space-y-2 rounded-lg border border-subtle bg-app px-4 py-3 text-sm">
			<li class="flex justify-between gap-3">
				<span class="text-muted">Super Admin</span>
				<span class="font-medium text-main">{username || '—'}</span>
			</li>
			<li class="flex justify-between gap-3">
				<span class="text-muted">Node aktif</span>
				<span class="font-medium text-main">{nodeCount}</span>
			</li>
			<li class="flex justify-between gap-3">
				<span class="text-muted">Strategi</span>
				<span class="font-medium text-main">{strategyLabel}</span>
			</li>
		</ul>

		<Dialog.Footer>
			<Button class="w-full rounded-lg" onclick={onContinue}>Lanjut ke Beranda</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
