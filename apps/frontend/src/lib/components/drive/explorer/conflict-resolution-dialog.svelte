<script lang="ts">
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';

	let {
		open = $bindable(false),
		title = 'Berkas sudah ada',
		description,
		fileName = '',
		canReplace = true,
		replaceText = 'Ganti',
		renameText = 'Ganti nama',
		cancelText = 'Batal',
		onReplace,
		onKeepBoth,
		onCancel = () => {}
	}: {
		open?: boolean;
		title?: string;
		description?: string;
		fileName?: string;
		canReplace?: boolean;
		replaceText?: string;
		renameText?: string;
		cancelText?: string;
		onReplace?: () => void;
		onKeepBoth?: () => void;
		onCancel?: () => void;
	} = $props();
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="border-subtle bg-surface text-main shadow-none">
		<Dialog.Header>
			<Dialog.Title>{title}</Dialog.Title>
			<Dialog.Description>
				{#if description}
					{description}
				{:else}
					“{fileName}” sudah ada di folder ini. {canReplace ? 'Ganti atau simpan keduanya?' : 'Simpan dengan nama berikutnya atau batalkan?'}
				{/if}
			</Dialog.Description>
		</Dialog.Header>
		<Dialog.Footer class="flex-col gap-2 sm:flex-row">
			<Button
				type="button"
				variant="secondary"
				onclick={() => {
					onCancel();
					open = false;
				}}
			>
				{cancelText}
			</Button>
			{#if onKeepBoth}
				<Button
					type="button"
					variant="outline"
					onclick={() => {
						onKeepBoth();
						open = false;
					}}
				>
					{renameText}
				</Button>
			{/if}
			{#if canReplace && onReplace}
				<Button
					type="button"
					onclick={() => {
						onReplace();
						open = false;
					}}
				>
					{replaceText}
				</Button>
			{/if}
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
