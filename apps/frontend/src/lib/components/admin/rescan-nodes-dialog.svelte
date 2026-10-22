<script lang="ts">
	import { simulateScan } from '$lib/mocks/setup.js';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Progress } from '$lib/components/ui/progress/index.js';
	import { toast } from 'svelte-sonner';
	import { locale } from '$lib/locale.svelte.js';
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
		toast.success(locale.language === 'id' ? 'Pindai ulang selesai' : 'Rescan complete', {
			description:
				locale.language === 'id'
					? 'Daftar node diperbarui dari mock.'
					: 'The node list was refreshed from mock data.'
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
			<Dialog.Title
				>{locale.language === 'id' ? 'Pindai ulang node?' : 'Rescan nodes?'}</Dialog.Title
			>
			<Dialog.Description>
				{locale.language === 'id' ? 'Memindai ulang volume di' : 'Rescans volumes in'}
				<code class="text-xs">/app/data/storage/</code>
				{locale.language === 'id'
					? 'dan menyegarkan kapasitas node (mock).'
					: 'and refreshes node capacity (mock).'}
			</Dialog.Description>
		</Dialog.Header>

		{#if scanning}
			<div class="space-y-2 rounded-xl border border-subtle bg-app px-4 py-3">
				<p class="text-xs font-medium text-muted">
					{locale.language === 'id' ? 'Memindai volume…' : 'Scanning volumes…'}
				</p>
				<Progress value={66} max={100} />
			</div>
		{/if}

		<Dialog.Footer>
			<Button type="button" variant="secondary" disabled={scanning} onclick={cancel}>
				{locale.language === 'id' ? 'Batal' : 'Cancel'}
			</Button>
			<Button type="button" disabled={scanning} onclick={confirm}>
				{scanning
					? locale.language === 'id'
						? 'Memindai…'
						: 'Scanning…'
					: locale.language === 'id'
						? 'Pindai ulang'
						: 'Rescan'}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
