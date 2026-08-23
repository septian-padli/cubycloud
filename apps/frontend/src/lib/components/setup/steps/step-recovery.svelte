<script lang="ts">
	import * as Card from '$lib/components/ui/card/index.js';
	import * as Alert from '$lib/components/ui/alert/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Download, KeyRound, Check } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';

	let {
		recoveryKey = $bindable(),
		downloaded = $bindable()
	}: {
		recoveryKey: string;
		downloaded: boolean;
	} = $props();

	// Debug: Log when component mounts
	$effect(() => {
		console.log('[StepRecovery] recoveryKey:', recoveryKey ? `${recoveryKey.length} chars` : 'EMPTY');
	});

	function downloadKey() {
		if (!recoveryKey || recoveryKey.length !== 64) {
			toast.error('Recovery key belum di-generate!');
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
		toast.success('Recovery key berhasil diunduh');
	}

	function copyKey() {
		if (!recoveryKey || recoveryKey.length !== 64) {
			toast.error('Recovery key belum di-generate!');
			return;
		}
		navigator.clipboard.writeText(recoveryKey).then(() => {
			toast.success('Recovery key disalin ke clipboard');
		});
	}
</script>

<Card.Root class="rounded-xl border-subtle bg-surface shadow-none">
	<Card.Header>
		<Card.Title class="flex items-center gap-2 text-lg font-semibold text-main">
			<KeyRound class="size-5 stroke-2" />
			Recovery Key Anda
		</Card.Title>
		<Card.Description class="text-sm text-muted">
			Simpan kode ini dengan aman untuk memulihkan password jika lupa
		</Card.Description>
	</Card.Header>
	<Card.Content class="space-y-4">
		<Alert.Root variant="destructive" class="border-red-500/30 bg-red-500/5">
			<Alert.Title class="text-red-700">⚠️ Penting!</Alert.Title>
			<Alert.Description class="text-red-600">
				Recovery key ini <strong>tidak akan ditampilkan lagi</strong> setelah setup selesai.
				<strong class="block mt-1">Download sekarang atau Anda tidak bisa memulihkan password!</strong>
			</Alert.Description>
		</Alert.Root>

		<!-- Recovery Key Display -->
		<div class="rounded-xl border border-subtle bg-app p-4">
			<div class="mb-2 text-xs font-medium text-muted">RECOVERY KEY (64 karakter)</div>
			{#if !recoveryKey || recoveryKey.length === 0}
				<code class="block break-all rounded-lg bg-surface px-3 py-3 font-mono text-sm text-red-500 border border-subtle">
					⚠️ Recovery key belum di-generate! Silakan reload halaman.
				</code>
			{:else}
				<code class="block break-all rounded-lg bg-surface px-3 py-3 font-mono text-sm text-main border border-subtle">
					{recoveryKey}
				</code>
			{/if}
		</div>

		<!-- Action Buttons -->
		<div class="flex gap-3">
			<Button
				onclick={downloadKey}
				variant="default"
				class="flex-1 gap-2"
				disabled={downloaded}
			>
				{#if downloaded}
					<Check class="size-4" />
					Sudah Diunduh
				{:else}
					<Download class="size-4" />
					Download Key
				{/if}
			</Button>
			<Button
				onclick={copyKey}
				variant="outline"
				class="gap-2"
			>
				<svg class="size-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
					<rect x="9" y="9" width="13" height="13" rx="2" ry="2"></rect>
					<path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"></path>
				</svg>
				Salin
			</Button>
		</div>

		{#if downloaded}
			<div class="flex items-start gap-2 rounded-lg bg-green-500/10 border border-green-500/30 px-3 py-2 text-sm text-green-700">
				<Check class="size-4 mt-0.5 flex-shrink-0" />
				<span>Recovery key sudah diunduh. Anda bisa melanjutkan setup.</span>
			</div>
		{/if}
	</Card.Content>
</Card.Root>
