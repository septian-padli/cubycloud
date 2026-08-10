<script lang="ts">
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';

	let {
		open = $bindable(false),
		fileName = '',
		onConfirm
	}: {
		open?: boolean;
		fileName?: string;
		onConfirm: () => void;
	} = $props();
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="bg-surface text-main">
		<Dialog.Header>
			<Dialog.Title>Folder asal tidak ditemukan</Dialog.Title>
			<Dialog.Description>
				Folder induk “{fileName}” sudah tidak ada. Pulihkan ke root My Storage?
			</Dialog.Description>
		</Dialog.Header>
		<Dialog.Footer>
			<Button type="button" variant="secondary" onclick={() => (open = false)}>Batal</Button>
			<Button
				type="button"
				onclick={() => {
					onConfirm();
					open = false;
				}}
			>
				Pulihkan ke Root
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
