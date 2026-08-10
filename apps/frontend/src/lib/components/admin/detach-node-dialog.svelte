<script lang="ts">
	import type { StorageNode } from '$lib/types/setup.js';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { toast } from 'svelte-sonner';

	let {
		open = $bindable(false),
		node = null,
		onDetach
	}: {
		open?: boolean;
		node?: StorageNode | null;
		onDetach: (nodeId: string) => void;
	} = $props();

	function confirm() {
		if (!node) return;
		onDetach(node.id);
		toast.success(`Node “${node.name}” dilepaskan (mock)`, {
			description: 'Node dinonaktifkan di pool lokal.'
		});
		open = false;
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="bg-surface text-main">
		<Dialog.Header>
			<Dialog.Title>Lepaskan node?</Dialog.Title>
			<Dialog.Description>
				{#if node}
					Node <span class="font-medium text-main">{node.name}</span>
					({node.path}) akan dinonaktifkan dari pool. Berkas yang sudah ada tetap di disk, tetapi
					node tidak menerima alokasi baru.
				{:else}
					Pilih node yang ingin dilepaskan.
				{/if}
			</Dialog.Description>
		</Dialog.Header>
		<Dialog.Footer>
			<Button type="button" variant="secondary" onclick={() => (open = false)}>Batal</Button>
			<Button type="button" variant="destructive" disabled={!node} onclick={confirm}>
				Lepaskan
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
