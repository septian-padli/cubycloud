<script lang="ts">
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { locale } from '$lib/locale.svelte.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { ShieldAlert } from '@lucide/svelte';
	const tx = (id: string, en: string) => (locale.language === 'id' ? id : en);

	let {
		open = $bindable(false),
		reason = tx(
			'Anda tidak memiliki izin mengunggah ke folder bersama ini',
			'You do not have permission to upload to this shared folder'
		),
		onClose
	}: {
		open?: boolean;
		reason?: string;
		onClose?: () => void;
	} = $props();

	function handleClose() {
		open = false;
		onClose?.();
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content
		class="border-subtle bg-surface text-main sm:max-w-md"
		data-testid="header-upload-denial-dialog"
	>
		<Dialog.Header class="flex flex-col gap-2">
			<div class="flex items-center gap-2 text-destructive">
				<ShieldAlert class="size-5 shrink-0" aria-hidden="true" />
				<Dialog.Title>{tx('Unggahan Tidak Diizinkan', 'Upload Not Allowed')}</Dialog.Title>
			</div>
			<Dialog.Description class="text-sm text-muted">
				{reason}
			</Dialog.Description>
		</Dialog.Header>
		<Dialog.Footer class="mt-4">
			<Button
				type="button"
				variant="default"
				onclick={handleClose}
				data-testid="header-upload-denial-close-button"
			>
				{tx('Tutup', 'Close')}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
