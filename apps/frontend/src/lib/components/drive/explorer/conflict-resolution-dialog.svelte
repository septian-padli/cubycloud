<script lang="ts">
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';

	let {
		open = $bindable(false),
		fileName = '',
		canReplace = true,
		onReplace,
		onKeepBoth,
		onCancel = () => {}
	}: {
		open?: boolean;
		fileName?: string;
		canReplace?: boolean;
		onReplace: () => void;
		onKeepBoth: () => void;
		onCancel?: () => void;
	} = $props();
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="bg-surface text-main">
		<Dialog.Header>
			<Dialog.Title>Berkas sudah ada</Dialog.Title>
			<Dialog.Description>
				“{fileName}” sudah ada di folder ini. {canReplace ? 'Ganti atau simpan keduanya?' : 'Simpan dengan nama berikutnya atau batalkan?'}
			</Dialog.Description>
		</Dialog.Header>
		<Dialog.Footer class="flex-col gap-2 sm:flex-row">
			<Button type="button" variant="secondary" onclick={() => { onCancel(); open = false; }}>Batal</Button>
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
			{#if canReplace}
				<Button
					type="button"
					onclick={() => {
						onReplace();
						open = false;
					}}
				>
					Ganti
				</Button>
			{/if}
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
