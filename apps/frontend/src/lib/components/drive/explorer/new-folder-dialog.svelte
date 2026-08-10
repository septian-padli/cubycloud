<script lang="ts">
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';

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
			<Dialog.Title>Folder Baru</Dialog.Title>
			<Dialog.Description>Buat folder di lokasi saat ini.</Dialog.Description>
		</Dialog.Header>
		<form
			class="grid gap-4"
			onsubmit={(e) => {
				e.preventDefault();
				submit();
			}}
		>
			<div class="grid gap-2">
				<Label for="new-folder-name">Nama folder</Label>
				<Input
					id="new-folder-name"
					bind:value={name}
					placeholder="mis. Dokumen"
					autocomplete="off"
				/>
			</div>
			<Dialog.Footer>
				<Button type="button" variant="secondary" onclick={() => (open = false)}>Batal</Button>
				<Button type="submit" disabled={!name.trim()}>Buat</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>
