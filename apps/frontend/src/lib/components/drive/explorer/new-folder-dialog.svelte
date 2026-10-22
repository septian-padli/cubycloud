<script lang="ts">
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { locale } from '$lib/locale.svelte.js';

	let {
		open = $bindable(false),
		onConfirm
	}: {
		open?: boolean;
		onConfirm: (name: string) => void;
	} = $props();

	let name = $state('');

	$effect(() => {
		if (open) name = '';
	});

	function submit() {
		const trimmed = name.trim();
		if (!trimmed) return;
		onConfirm(trimmed);
		open = false;
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="bg-surface text-main">
		<Dialog.Header>
			<Dialog.Title>{locale.language === 'id' ? 'Folder Baru' : 'New folder'}</Dialog.Title>
			<Dialog.Description
				>{locale.language === 'id'
					? 'Buat folder di lokasi saat ini.'
					: 'Create a folder in the current location.'}</Dialog.Description
			>
		</Dialog.Header>
		<form
			class="grid gap-4"
			onsubmit={(e) => {
				e.preventDefault();
				submit();
			}}
		>
			<div class="grid gap-2">
				<Label for="new-folder-name"
					>{locale.language === 'id' ? 'Nama folder' : 'Folder name'}</Label
				>
				<Input
					id="new-folder-name"
					bind:value={name}
					placeholder={locale.language === 'id' ? 'mis. Dokumen' : 'e.g. Documents'}
					autocomplete="off"
				/>
			</div>
			<Dialog.Footer>
				<Button type="button" variant="secondary" onclick={() => (open = false)}
					>{locale.language === 'id' ? 'Batal' : 'Cancel'}</Button
				>
				<Button type="submit" disabled={!name.trim()}
					>{locale.language === 'id' ? 'Buat' : 'Create'}</Button
				>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>
