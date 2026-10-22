<script lang="ts">
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Checkbox } from '$lib/components/ui/checkbox/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { PartyPopper, Copy, Download } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import type { PlacementStrategy } from '$lib/types/setup.js';
	import { STRATEGY_OPTIONS } from '$lib/types/setup.js';
	import { locale } from '$lib/locale.svelte.js';
	const tx = (id: string, en: string) => (locale.language === 'id' ? id : en);

	let {
		open = $bindable(false),
		username,
		nodeCount,
		strategy,
		recoveryKey,
		onContinue
	}: {
		open?: boolean;
		username: string;
		nodeCount: number;
		strategy: PlacementStrategy;
		recoveryKey: string;
		onContinue: () => void;
	} = $props();

	const strategyLabel = $derived(
		STRATEGY_OPTIONS.find((o) => o.value === strategy)?.label ?? strategy
	);

	let acknowledged = $state(false);

	function copyRecoveryKey() {
		navigator.clipboard.writeText(recoveryKey).then(
			() =>
				toast.success(tx('Kunci pemulihan disalin ke papan klip', 'Recovery key copied to clipboard')),
			() => toast.error(tx('Gagal menyalin kunci pemulihan', 'Failed to copy recovery key'))
		);
	}

	function downloadRecoveryKey() {
		const blob = new Blob([recoveryKey], { type: 'text/plain' });
		const url = URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		a.download = 'cubycloud-recovery-key.txt';
		a.click();
		URL.revokeObjectURL(url);
		toast.success(tx('Kunci pemulihan diunduh', 'Recovery key downloaded'));
	}

	function handleContinue() {
		if (!acknowledged) return;
		onContinue();
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="rounded-xl sm:max-w-md" showCloseButton={false}>
		<Dialog.Header>
			<div
				class="mb-2 flex size-10 items-center justify-center rounded-lg bg-primary/10 text-primary"
			>
				<PartyPopper class="size-5 stroke-2" />
			</div>
			<Dialog.Title class="text-lg font-semibold text-main"
				>{tx('Setup selesai', 'Setup complete')}</Dialog.Title
			>
			<Dialog.Description class="text-sm text-muted">
				{tx(
					'CubyCloud siap digunakan. Ringkasan konfigurasi awal:',
					'CubyCloud is ready to use. Initial configuration summary:'
				)}
			</Dialog.Description>
		</Dialog.Header>

		<ul class="space-y-2 rounded-lg border border-subtle bg-app px-4 py-3 text-sm">
			<li class="flex justify-between gap-3">
				<span class="text-muted">Super Admin</span>
				<span class="font-medium text-main">{username || '—'}</span>
			</li>
			<li class="flex justify-between gap-3">
				<span class="text-muted">{tx('Node aktif', 'Active nodes')}</span>
				<span class="font-medium text-main">{nodeCount}</span>
			</li>
			<li class="flex justify-between gap-3">
				<span class="text-muted">{tx('Strategi', 'Strategy')}</span>
				<span class="font-medium text-main">{strategyLabel}</span>
			</li>
		</ul>

		<div class="space-y-3 rounded-lg border border-subtle bg-app p-4">
			<p class="text-sm font-medium text-main">Recovery Key</p>
			<p class="text-xs text-muted">
				{tx(
					'Simpan kunci ini dengan aman. Kunci diperlukan untuk memulihkan akses jika Anda lupa kata sandi.',
					'Store this key securely. It is required to recover access if you forget your password.'
				)}
			</p>
			<div
				class="rounded-md border border-subtle bg-surface px-3 py-2 font-mono text-xs break-all text-main"
			>
				{recoveryKey}
			</div>
			<div class="flex gap-2">
				<Button variant="outline" size="sm" class="flex-1" onclick={copyRecoveryKey}>
					<Copy class="size-4 stroke-2" />
					{tx('Salin', 'Copy')}
				</Button>
				<Button variant="outline" size="sm" class="flex-1" onclick={downloadRecoveryKey}>
					<Download class="size-4 stroke-2" />
					{tx('Unduh', 'Download')}
				</Button>
			</div>
			<div class="flex items-center gap-2 pt-2">
				<Checkbox id="ack" bind:checked={acknowledged} />
				<Label for="ack" class="cursor-pointer text-xs font-normal text-muted">
					{tx(
						'Saya sudah menyimpan kunci pemulihan dengan aman',
						'I have securely saved the recovery key'
					)}
				</Label>
			</div>
		</div>

		<Dialog.Footer>
			<Button class="w-full" disabled={!acknowledged} onclick={handleContinue}>
				{tx('Lanjut ke Beranda', 'Continue to Home')}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
