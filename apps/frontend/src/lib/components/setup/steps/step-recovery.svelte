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



	function downloadKey() {
		if (!recoveryKey || recoveryKey.length !== 35) {
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
		if (!recoveryKey || recoveryKey.length !== 35) {
			toast.error('Recovery key belum di-generate!');
			return;
		}
		navigator.clipboard.writeText(recoveryKey).then(() => {
			downloaded = true;
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
		<Alert.Root variant="destructive">
			<Alert.Title>⚠️ Penting!</Alert.Title>
			<Alert.Description>
				Recovery key ini <strong>tidak akan ditampilkan lagi</strong> setelah setup selesai.
				<strong class="block mt-1">Download sekarang atau Anda tidak bisa memulihkan password!</strong>
			</Alert.Description>
		</Alert.Root>

		<!-- Recovery Key Display -->
		<div class="rounded-xl border border-subtle bg-app p-4">
			<div class="mb-2 text-xs font-medium text-muted">RECOVERY KEY (6 × 5 karakter)</div>
			{#if !recoveryKey || recoveryKey.length === 0}
				<code class="block break-all rounded-lg bg-surface px-3 py-3 font-mono text-sm text-danger border border-subtle">
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
					Sudah Disimpan
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
			<Alert.Root variant="default">
				<Check class="size-4 stroke-2" />
				<Alert.Title>Recovery key tersimpan</Alert.Title>
				<Alert.Description>
					Recovery key sudah disimpan (diunduh atau disalin). Anda bisa melanjutkan setup.
				</Alert.Description>
			</Alert.Root>
		{/if}
	</Card.Content>
</Card.Root>
