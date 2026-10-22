<script lang="ts">
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Check, Copy, Download, KeyRound } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import { locale } from '$lib/locale.svelte.js';
	const tx = (id: string, en: string) => (locale.language === 'id' ? id : en);
	let {
		open = $bindable(false),
		recoveryKey,
		onContinue
	}: {
		open?: boolean;
		recoveryKey: string;
		onContinue: () => void;
	} = $props();

	let saved = $state(false);

	function handleOpenChange(nextOpen: boolean) {
		if (nextOpen || saved) open = nextOpen;
	}

	async function copyKey() {
		try {
			await navigator.clipboard.writeText(recoveryKey);
			saved = true;
			toast.success(tx('Kunci pemulihan disalin', 'Recovery key copied'));
		} catch {
			toast.error(
				tx(
					'Tidak dapat menyalin kunci pemulihan. Coba unduh kuncinya.',
					'Could not copy the recovery key. Try downloading it.'
				)
			);
		}
	}

	function downloadKey() {
		const blob = new Blob([recoveryKey], { type: 'text/plain' });
		const url = URL.createObjectURL(blob);
		const link = document.createElement('a');
		link.href = url;
		link.download = `cubycloud-recovery-key-${Date.now()}.txt`;
		document.body.appendChild(link);
		link.click();
		link.remove();
		URL.revokeObjectURL(url);
		saved = true;
	}
</script>

<Dialog.Root bind:open onOpenChange={handleOpenChange}>
	<Dialog.Content
		class="rounded-xl sm:max-w-md"
		showCloseButton={false}
		onEscapeKeydown={(event) => {
			if (!saved) event.preventDefault();
		}}
		onInteractOutside={(event) => {
			if (!saved) event.preventDefault();
		}}
	>
		<Dialog.Header>
			<div
				class="mb-2 flex size-10 items-center justify-center rounded-lg bg-primary/10 text-primary"
			>
				<KeyRound class="size-5 stroke-2" />
			</div>
			<Dialog.Title class="text-lg font-semibold text-main"
				>{tx('Simpan Kunci Pemulihan baru', 'Save your new Recovery Key')}</Dialog.Title
			>
			<Dialog.Description class="text-sm text-muted">
				{tx(
					'Kunci lama sudah tidak berlaku. Salin atau unduh kunci baru ini sekarang; kunci ini tidak akan ditampilkan lagi.',
					'Your old key is no longer valid. Copy or download this new key now; it will not be shown again.'
				)}
			</Dialog.Description>
		</Dialog.Header>

		<code
			class="block rounded-lg border border-subtle bg-app px-3 py-3 text-center font-mono text-sm break-all text-main select-all"
		>
			{recoveryKey}
		</code>

		<div class="grid grid-cols-2 gap-3">
			<Button variant="outline" class="gap-2" onclick={copyKey}>
				{#if saved}<Check class="size-4" />{:else}<Copy class="size-4" />{/if}
				{tx('Salin', 'Copy')}
			</Button>
			<Button variant="outline" class="gap-2" onclick={downloadKey}>
				<Download class="size-4" />
				{tx('Unduh', 'Download')}
			</Button>
		</div>

		{#if saved}
			<p class="text-sm text-primary" role="status">
				{tx('Kunci sudah disimpan. Anda bisa melanjutkan.', 'Key saved. You can continue.')}
			</p>
		{/if}

		<Dialog.Footer>
			<Button class="w-full" disabled={!saved} onclick={onContinue}
				>{tx('Ke halaman Masuk', 'Go to Sign in')}</Button
			>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
