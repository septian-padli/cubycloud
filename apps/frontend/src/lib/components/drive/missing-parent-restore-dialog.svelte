<script lang="ts">
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { locale } from '$lib/locale.svelte.js';
	const tx = (id: string, en: string) => (locale.language === 'id' ? id : en);

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
			<Dialog.Title>{tx('Folder asal tidak ditemukan', 'Original folder not found')}</Dialog.Title>
			<Dialog.Description>
				{tx(
					`Folder induk “${fileName}” sudah tidak ada. Pulihkan ke root My Storage?`,
					`The parent folder for “${fileName}” no longer exists. Restore to the My Storage root?`
				)}
			</Dialog.Description>
		</Dialog.Header>
		<Dialog.Footer>
			<Button type="button" variant="secondary" onclick={() => (open = false)}
				>{tx('Batal', 'Cancel')}</Button
			>
			<Button
				type="button"
				onclick={() => {
					onConfirm();
					open = false;
				}}
			>
				{tx('Pulihkan ke Root', 'Restore to root')}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
