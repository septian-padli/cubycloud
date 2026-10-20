<script lang="ts">
	import * as Card from '$lib/components/ui/card/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import PasswordInput from '$lib/components/password-input.svelte';
	import { Loader2 } from '@lucide/svelte';

	let {
		onUnlock
	}: {
		onUnlock: (password: string) => boolean | Promise<boolean>;
	} = $props();

	let password = $state('');
	let error = $state('');
	let loading = $state(false);

	async function submit(e: Event) {
		e.preventDefault();
		if (password.trim().length === 0) {
			error = 'Masukkan kata sandi tautan.';
			return;
		}
		error = '';
		loading = true;
		try {
			const ok = await onUnlock(password.trim());
			if (!ok) {
				error = 'Kata sandi tidak sesuai.';
			}
		} catch (err) {
			error = err instanceof Error ? err.message : 'Gagal membuka tautan.';
		} finally {
			loading = false;
		}
	}
</script>

<div class="flex flex-1 items-center justify-center py-8">
	<Card.Root class="w-full max-w-md rounded-xl border-subtle bg-surface shadow-none">
		<Card.Header>
			<Card.Title class="text-lg font-semibold text-main">Tautan dilindungi</Card.Title>
			<Card.Description class="text-sm text-muted">
				Masukkan kata sandi yang diberikan pemilik berkas untuk mengakses tautan ini.
			</Card.Description>
		</Card.Header>
		<Card.Content>
			<form class="grid gap-4" onsubmit={submit}>
				<div class="grid gap-2">
					<Label for="share-gate-password">Kata sandi</Label>
					<PasswordInput
						id="share-gate-password"
						bind:value={password}
						autocomplete="off"
						aria-invalid={error ? true : undefined}
					/>
					{#if error}
						<p class="text-xs text-danger">{error}</p>
					{/if}
				</div>
				<Button type="submit" disabled={loading} class="w-full">
					{#if loading}
						<Loader2 class="size-4 animate-spin stroke-2" />
					{/if}
					Buka tautan
				</Button>
			</form>
		</Card.Content>
	</Card.Root>
</div>
