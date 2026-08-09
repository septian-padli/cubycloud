<script lang="ts">
	import * as Card from '$lib/components/ui/card/index.js';
	import * as Alert from '$lib/components/ui/alert/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Checkbox } from '$lib/components/ui/checkbox/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { Check, Copy, Download, KeyRound } from '@lucide/svelte';

	let {
		recoveryKey,
		acknowledged = $bindable()
	}: {
		recoveryKey: string;
		acknowledged: boolean;
	} = $props();

	let copied = $state(false);

	async function copyKey() {
		try {
			await navigator.clipboard.writeText(recoveryKey);
			copied = true;
			setTimeout(() => {
				copied = false;
			}, 2000);
		} catch {
			copied = false;
		}
	}

	function downloadKey() {
		const blob = new Blob([recoveryKey + '\n'], { type: 'text/plain' });
		const url = URL.createObjectURL(blob);
		const a = document.createElement('a');
		a.href = url;
		a.download = 'cubycloud-recovery-key.txt';
		a.click();
		URL.revokeObjectURL(url);
	}
</script>

<Card.Root class="rounded-xl border-subtle bg-surface shadow-none">
	<Card.Header>
		<Card.Title class="text-lg font-semibold text-main">Kunci Pemulihan Darurat</Card.Title>
		<Card.Description class="text-sm text-muted">
			Simpan kunci 64 karakter ini. Digunakan untuk memulihkan akses Super Admin jika lupa password.
		</Card.Description>
	</Card.Header>
	<Card.Content class="space-y-4">
		<Alert.Root variant="destructive" class="border-danger/30">
			<KeyRound class="size-4 stroke-2" />
			<Alert.Title>Jangan hilangkan kunci ini</Alert.Title>
			<Alert.Description>
				CubyCloud v1.0 tidak mengirim email pemulihan. Tanpa kunci ini, akun Super Admin tidak dapat
				dipulihkan.
			</Alert.Description>
		</Alert.Root>

		<div class="space-y-2">
			<Label for="recovery-key">Emergency Recovery Key</Label>
			<Input
				id="recovery-key"
				class="font-mono text-xs"
				readonly
				value={recoveryKey}
			/>
		</div>

		<div class="flex flex-wrap gap-2">
			<Button variant="outline" class="gap-1.5" onclick={copyKey}>
				{#if copied}
					<Check class="size-4 stroke-2" />
					Disalin
				{:else}
					<Copy class="size-4 stroke-2" />
					Salin kunci
				{/if}
			</Button>
			<Button variant="outline" class="gap-1.5" onclick={downloadKey}>
				<Download class="size-4 stroke-2" />
				Unduh .txt
			</Button>
		</div>

		<label class="flex cursor-pointer items-start gap-3 rounded-xl border border-subtle bg-app px-4 py-3">
			<Checkbox
				id="recovery-ack"
				class="mt-0.5"
				checked={acknowledged}
				onCheckedChange={(v) => {
					acknowledged = v === true;
				}}
			/>
			<Label for="recovery-ack" class="cursor-pointer text-sm leading-normal text-main">
				Saya sudah menyimpan kunci pemulihan (salin atau unduh) dan memahami risikonya.
			</Label>
		</label>
	</Card.Content>
</Card.Root>
