<script lang="ts">
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';

	let {
		open = $bindable(false),
		itemCount = 0,
		onConfirm
	}: {
		open?: boolean;
		itemCount?: number;
		onConfirm: () => void;
	} = $props();
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="bg-surface text-main">
		<Dialog.Header>
			<Dialog.Title>Kosongkan Sampah</Dialog.Title>
			<Dialog.Description>
				Hapus permanen {itemCount} item di Trash? Tindakan ini tidak dapat dibatalkan.
			</Dialog.Description>
		</Dialog.Header>
		<Dialog.Footer>
			<Button type="button" variant="secondary" onclick={() => (open = false)}>Batal</Button>
			<Button
				type="button"
				variant="destructive"
				onclick={() => {
					onConfirm();
					open = false;
				}}
			>
				Hapus permanen semua
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
