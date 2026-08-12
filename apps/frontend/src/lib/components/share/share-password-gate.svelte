<script lang="ts">
	import * as Card from '$lib/components/ui/card/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import PasswordInput from '$lib/components/password-input.svelte';

	let {
		onUnlock
	}: {
		onUnlock: (password: string) => boolean;
	} = $props();

	let password = $state('');
	let error = $state('');

	function submit(e: Event) {
		e.preventDefault();
		if (password.trim().length === 0) {
			error = 'Masukkan kata sandi tautan.';
			return;
		}
		const ok = onUnlock(password);
		if (!ok) error = 'Kata sandi tidak sesuai.';
	}
</script>

<div class="flex flex-1 items-center justify-center py-8">
	<Card.Root class="w-full max-w-md rounded-xl border-subtle bg-surface shadow-none">
		<Card.Header>
			<Card.Title class="text-lg font-semibold text-main">Tautan dilindungi</Card.Title>
			<Card.Description class="text-sm text-muted">
				Masukkan kata sandi yang diberikan pemilik berkas.
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
				<Button type="submit">Buka tautan</Button>
			</form>
		</Card.Content>
	</Card.Root>
</div>
