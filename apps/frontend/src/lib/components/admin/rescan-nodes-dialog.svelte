<script lang="ts">
	import { simulateScan } from '$lib/mocks/setup.js';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Progress } from '$lib/components/ui/progress/index.js';
	import { toast } from 'svelte-sonner';

	let {
		open = $bindable(false),
		onRescanned
	}: {
		open?: boolean;
		onRescanned: () => void;
	} = $props();

	let scanning = $state(false);

	async function confirm() {
		scanning = true;
		await simulateScan();
		onRescanned();
		scanning = false;
		open = false;
		toast.success('Pindai ulang selesai', {
			description: 'Daftar node diperbarui dari mock.'
		});
	}

	function cancel() {
		if (scanning) return;
		open = false;
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="bg-surface text-main">
		<Dialog.Header>
			<Dialog.Title>Pindai ulang node?</Dialog.Title>
			<Dialog.Description>
				Memindai ulang volume di <code class="text-xs">/app/data/storage/</code> dan menyegarkan
				kapasitas node (mock).
			</Dialog.Description>
		</Dialog.Header>

		{#if scanning}
			<div class="space-y-2 rounded-xl border border-subtle bg-app px-4 py-3">
				<p class="text-xs font-medium text-muted">Memindai volume…</p>
				<Progress value={66} max={100} />
			</div>
		{/if}

		<Dialog.Footer>
			<Button type="button" variant="secondary" disabled={scanning} onclick={cancel}>
				Batal
			</Button>
			<Button type="button" disabled={scanning} onclick={confirm}>
				{scanning ? 'Memindai…' : 'Pindai ulang'}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
