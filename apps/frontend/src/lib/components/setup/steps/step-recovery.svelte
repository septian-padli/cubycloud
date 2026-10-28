<script lang="ts">
	import { onDestroy } from 'svelte';
	import * as Card from '$lib/components/ui/card/index.js';
	import * as Alert from '$lib/components/ui/alert/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Download, KeyRound, Check, AlertTriangle } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import { locale } from '$lib/locale.svelte.js';
	const tx = (id: string, en: string) => (locale.language === 'id' ? id : en);
	let {
		recoveryKey = $bindable(),
		downloaded = $bindable()
	}: {
		recoveryKey: string;
		downloaded: boolean;
	} = $props();
	let justDownloaded = $state(false);
	let timer: ReturnType<typeof setTimeout> | undefined;

	function triggerJustDownloaded() {
		justDownloaded = true;
		if (timer) clearTimeout(timer);
		timer = setTimeout(() => {
			justDownloaded = false;
		}, 3000);
	}

	onDestroy(() => {
		if (timer) clearTimeout(timer);
	});

	function downloadKey() {
		if (!recoveryKey || recoveryKey.length !== 35) {
			toast.error(tx('Kunci pemulihan belum dibuat!', 'Recovery key has not been generated!'));
			return;
		}

		const blob = new Blob([recoveryKey], { type: 'text/plain' });
		const url = URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		a.download = `cubycloud-recovery-key-${Date.now()}.txt`;
		document.body.appendChild(a);
		a.click();
		document.body.removeChild(a);
		URL.revokeObjectURL(url);

		downloaded = true;
		triggerJustDownloaded();
		toast.success(tx('Kunci pemulihan berhasil diunduh', 'Recovery key downloaded successfully'));
	}

	function copyKey() {
		if (!recoveryKey || recoveryKey.length !== 35) {
			toast.error(tx('Kunci pemulihan belum dibuat!', 'Recovery key has not been generated!'));
			return;
		}
		navigator.clipboard.writeText(recoveryKey).then(() => {
			downloaded = true;
			toast.success(
				tx('Kunci pemulihan disalin ke papan klip', 'Recovery key copied to clipboard')
			);
			triggerJustDownloaded();
		});
	}
</script>

<Card.Root class="rounded-xl border-subtle bg-surface shadow-none">
	<Card.Header>
		<Card.Title class="flex items-center gap-2 text-lg font-semibold text-main">
			<KeyRound class="size-5 stroke-2" />
			{tx('Kunci Pemulihan Anda', 'Your Recovery Key')}
		</Card.Title>
		<Card.Description class="text-sm text-muted">
			{tx(
				'Simpan kode ini dengan aman untuk memulihkan akses jika Anda lupa kata sandi.',
				'Save this code securely to recover your password if you forget it.'
			)}
		</Card.Description>
	</Card.Header>
	<Card.Content class="space-y-4">
		<Alert.Root variant="destructive">
			<AlertTriangle class="size-4 stroke-2" />
			<Alert.Title>{tx('Penting!', 'Important!')}</Alert.Title>
			<Alert.Description>
				{tx('Kunci pemulihan ini', 'This recovery key')}
				<strong>{tx('tidak akan ditampilkan lagi', 'will not be shown again')}</strong>
				{tx('setelah penyiapan selesai.', 'after setup is complete.')}
				<strong class="mt-1 block"
					>{tx(
						'Unduh sekarang atau Anda tidak bisa memulihkan akses jika lupa kata sandi!',
						'Download it now or you will not be able to recover your password!'
					)}</strong
				>
			</Alert.Description>
		</Alert.Root>
		<!-- Recovery Key Display -->
		<div class="rounded-xl border border-subtle bg-app p-4">
			<div class="mb-2 text-xs font-medium text-muted">
				{tx('KUNCI PEMULIHAN (6 × 5 karakter)', 'RECOVERY KEY (6 × 5 characters)')}
			</div>
			{#if !recoveryKey || recoveryKey.length === 0}
				<code
					class="block rounded-lg border border-subtle bg-surface px-3 py-3 font-mono text-sm break-all text-danger"
				>
					{tx(
						'⚠️ Kunci pemulihan belum dibuat! Silakan muat ulang halaman.',
						'⚠️ Recovery key has not been generated! Please reload the page.'
					)}
				</code>
			{:else}
				<code
					class="block rounded-lg border border-subtle bg-surface px-3 py-3 font-mono text-sm break-all text-main"
				>
					{recoveryKey}
				</code>
			{/if}
		</div>

		<!-- Action Buttons -->
		<div class="flex gap-3">
			<Button
				onclick={downloadKey}
				variant={downloaded && !justDownloaded ? 'outline' : 'default'}
				class="flex-1 gap-2"
				disabled={justDownloaded}
			>
				{#if justDownloaded}
					<Check class="size-4" />
					{tx('Sudah Disimpan', 'Saved')}
				{:else if downloaded}
					<Download class="size-4" />
					{tx('Unduh lagi', 'Download again')}
				{:else}
					<Download class="size-4" />
					{tx('Unduh Kunci Pemulihan', 'Download Key')}
				{/if}
			</Button>
			<Button onclick={copyKey} variant="outline" class="gap-2">
				<svg class="size-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
					<path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path>
				</svg>
				{tx('Salin', 'Copy')}
			</Button>
		</div>

		{#if downloaded}
			<Alert.Root variant="default">
				<Check class="size-4 stroke-2" />
				<Alert.Title>{tx('Kunci pemulihan tersimpan', 'Recovery key saved')}</Alert.Title>
				<Alert.Description>
					{tx(
						'Kunci pemulihan sudah disimpan (diunduh atau disalin). Anda bisa melanjutkan penyiapan.',
						'The recovery key has been saved (downloaded or copied). You can continue setup.'
					)}
				</Alert.Description>
			</Alert.Root>
		{/if}
	</Card.Content>
</Card.Root>
