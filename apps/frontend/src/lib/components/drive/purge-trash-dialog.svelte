<script lang="ts">
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { locale } from '$lib/locale.svelte.js';
	import { Button } from '$lib/components/ui/button/index.js';
	const tx = (id: string, en: string) => (locale.language === 'id' ? id : en);

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
			<Dialog.Title>{tx('Kosongkan Sampah', 'Empty Trash')}</Dialog.Title>
			<Dialog.Description>
				{tx(
					`Hapus permanen ${itemCount} item di Trash? Tindakan ini tidak dapat dibatalkan.`,
					`Permanently delete ${itemCount} items in Trash? This action cannot be undone.`
				)}
			</Dialog.Description>
		</Dialog.Header>
		<Dialog.Footer>
			<Button type="button" variant="secondary" onclick={() => (open = false)}
				>{tx('Batal', 'Cancel')}</Button
			>
			<Button
				type="button"
				variant="destructive"
				onclick={() => {
					onConfirm();
					open = false;
				}}
			>
				{tx('Hapus permanen semua', 'Permanently delete all')}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
