<script lang="ts">
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { KeyRound } from '@lucide/svelte';

	let {
		open = $bindable(false),
		onContinue
	}: {
		open?: boolean;
		onContinue: () => void;
	} = $props();
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="rounded-xl sm:max-w-md" showCloseButton={false}>
		<Dialog.Header>
			<div class="mb-2 flex size-10 items-center justify-center rounded-lg bg-primary/10 text-primary">
				<KeyRound class="size-5 stroke-2" />
			</div>
			<Dialog.Title class="text-lg font-semibold text-main">Password berhasil direset</Dialog.Title>
			<Dialog.Description class="text-sm text-muted">
				Password Super Admin sudah diperbarui. Silakan masuk kembali dengan password baru.
			</Dialog.Description>
		</Dialog.Header>

		<Dialog.Footer>
			<Button class="w-full" onclick={onContinue}>Ke halaman Masuk</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
