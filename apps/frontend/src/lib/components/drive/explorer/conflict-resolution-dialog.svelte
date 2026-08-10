<script lang="ts">
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';

	let {
		open = $bindable(false),
		fileName = '',
		onReplace,
		onKeepBoth
	}: {
		open?: boolean;
		fileName?: string;
		onReplace: () => void;
		onKeepBoth: () => void;
	} = $props();
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="bg-surface text-main">
		<Dialog.Header>
			<Dialog.Title>Berkas sudah ada</Dialog.Title>
			<Dialog.Description>
				“{fileName}” sudah ada di folder ini. Ganti atau simpan keduanya?
			</Dialog.Description>
		</Dialog.Header>
		<Dialog.Footer class="flex-col gap-2 sm:flex-row">
			<Button type="button" variant="secondary" onclick={() => (open = false)}>Batal</Button>
			<Button
				type="button"
				variant="outline"
				onclick={() => {
					onKeepBoth();
					open = false;
				}}
			>
				Simpan keduanya
			</Button>
			<Button
				type="button"
				onclick={() => {
					onReplace();
					open = false;
				}}
			>
				Ganti
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
