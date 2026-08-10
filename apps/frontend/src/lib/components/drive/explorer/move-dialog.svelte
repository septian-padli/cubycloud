<script lang="ts">
	import type { FileEntry } from '$lib/types/drive.js';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Label } from '$lib/components/ui/label/index.js';

	let {
		open = $bindable(false),
		folders = [],
		currentParentId = null,
		onConfirm
	}: {
		open?: boolean;
		folders?: FileEntry[];
		currentParentId?: string | null;
		onConfirm: (targetFolderId: string | null) => void;
	} = $props();

	let target = $state<string>('root');

	$effect(() => {
		if (open) target = currentParentId ?? 'root';
	});

	function submit() {
		onConfirm(target === 'root' ? null : target);
		open = false;
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="bg-surface text-main">
		<Dialog.Header>
			<Dialog.Title>Pindahkan</Dialog.Title>
			<Dialog.Description>Pilih folder tujuan.</Dialog.Description>
		</Dialog.Header>
		<div class="grid gap-2">
			<Label for="move-target">Folder tujuan</Label>
			<select
				id="move-target"
				bind:value={target}
				class="h-9 w-full rounded-full border border-subtle bg-surface px-3 text-sm text-main shadow-none outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
			>
				<option value="root">Root (beranda drive)</option>
				{#each folders as folder (folder.id)}
					<option value={folder.id}>{folder.name}</option>
				{/each}
			</select>
		</div>
		<Dialog.Footer>
			<Button type="button" variant="secondary" onclick={() => (open = false)}>Batal</Button>
			<Button type="button" onclick={submit}>Pindahkan</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
