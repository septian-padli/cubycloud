<script lang="ts">
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';

	let {
		open = $bindable(false),
		initialName = '',
		onConfirm
	}: {
		open?: boolean;
		initialName?: string;
		onConfirm: (name: string) => void;
	} = $props();

	let name = $state('');

	$effect(() => {
		if (open) name = initialName;
	});

	function submit() {
		const trimmed = name.trim();
		if (!trimmed || trimmed === initialName) {
			open = false;
			return;
		}
		onConfirm(trimmed);
		open = false;
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="bg-surface text-main">
		<Dialog.Header>
			<Dialog.Title>Ganti nama</Dialog.Title>
			<Dialog.Description>Masukkan nama baru untuk item ini.</Dialog.Description>
		</Dialog.Header>
		<form
			class="grid gap-4"
			onsubmit={(e) => {
				e.preventDefault();
				submit();
			}}
		>
			<div class="grid gap-2">
				<Label for="rename-name">Nama</Label>
				<Input id="rename-name" bind:value={name} autocomplete="off" />
			</div>
			<Dialog.Footer>
				<Button type="button" variant="secondary" onclick={() => (open = false)}>Batal</Button>
				<Button type="submit" disabled={!name.trim()}>Simpan</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>
