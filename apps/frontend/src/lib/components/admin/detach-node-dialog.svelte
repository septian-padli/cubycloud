<script lang="ts">
	import type { StorageNode } from '$lib/types/setup.js';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { toast } from 'svelte-sonner';
	import { locale } from '$lib/locale.svelte.js';
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
		toast.success(
			locale.language === 'id'
				? `Node “${node.name}” dilepaskan (simulasi)`
				: `Node “${node.name}” detached (mock)`,
			{
				description:
					locale.language === 'id'
						? 'Node dinonaktifkan di pool lokal.'
						: 'The node was disabled in the local pool.'
			}
		);
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="bg-surface text-main">
		<Dialog.Header>
			<Dialog.Title>{locale.language === 'id' ? 'Lepaskan node?' : 'Detach node?'}</Dialog.Title>
			<Dialog.Description>
				{#if node}
					Node <span class="font-medium text-main">{node.name}</span>
					({node.path}) {locale.language === 'id'
						? 'akan dinonaktifkan dari pool. Berkas yang sudah ada tetap di disk, tetapi node tidak menerima alokasi baru.'
						: 'will be disabled from the pool. Existing files will remain on disk, but the node will not receive new allocations.'}
				{:else}
					{locale.language === 'id'
						? 'Pilih node yang ingin dilepaskan.'
						: 'Select a node to detach.'}
				{/if}
			</Dialog.Description>
		</Dialog.Header>
		<Dialog.Footer>
			<Button type="button" variant="secondary" onclick={() => (open = false)}
				>{locale.language === 'id' ? 'Batal' : 'Cancel'}</Button
			>
			<Button type="button" variant="destructive" disabled={!node} onclick={confirm}>
				{locale.language === 'id' ? 'Lepaskan' : 'Detach'}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
