<script lang="ts">
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { locale } from '$lib/locale.svelte.js';

	let {
		open = $bindable(false),
		title,
		description,
		fileName = '',
		canReplace = true,
		replaceText,
		renameText,
		cancelText,
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

	const tx = (id: string, en: string) => (locale.language === 'id' ? id : en);
	const titleText = $derived(title ?? tx('Berkas sudah ada', 'File already exists'));
	const replaceLabel = $derived(replaceText ?? tx('Ganti', 'Replace'));
	const renameLabel = $derived(renameText ?? tx('Ganti nama', 'Keep both'));
	const cancelLabel = $derived(cancelText ?? tx('Batal', 'Cancel'));
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="border-subtle bg-surface text-main shadow-none">
		<Dialog.Header>
			<Dialog.Title>{titleText}</Dialog.Title>
			<Dialog.Description>
				{#if description}
					{description}
				{:else}
					{locale.language === 'id'
						? `“${fileName}” sudah ada di folder ini. ${canReplace ? 'Ganti atau simpan keduanya?' : 'Simpan dengan nama berikutnya atau batalkan?'}`
						: `“${fileName}” already exists in this folder. ${canReplace ? 'Replace it or keep both?' : 'Save with the next available name or cancel?'}`}
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
				{cancelLabel}
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
					{renameLabel}
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
					{replaceLabel}
				</Button>
			{/if}
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
