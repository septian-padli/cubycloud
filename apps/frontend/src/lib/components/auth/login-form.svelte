<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import * as Alert from '$lib/components/ui/alert/index.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import PasswordInput from '$lib/components/password-input.svelte';
	import { setMockLoggedIn } from '$lib/mocks/auth-session.js';
	import { KeyRound } from '@lucide/svelte';

	let identifier = $state('');
	let password = $state('');
	let submitted = $state(false);
	let forgotInfo = $state(false);

	const identifierEmpty = $derived(submitted && identifier.trim().length === 0);
	const passwordEmpty = $derived(submitted && password.length === 0);
	const canSubmit = $derived(identifier.trim().length > 0 && password.length > 0);

	function onSubmit(e: Event) {
		e.preventDefault();
		submitted = true;
		if (!canSubmit) return;
		setMockLoggedIn(true);
		const redirect = page.url.searchParams.get('redirect');
		const safe =
			redirect && redirect.startsWith('/') && !redirect.startsWith('//') ? redirect : '/drive/home';
		void goto(safe);
	}
</script>

<Card.Root class="rounded-xl border-subtle bg-surface shadow-none">
	<Card.Header>
		<Card.Title class="text-lg font-semibold text-main">Masuk</Card.Title>
		<Card.Description class="text-sm text-muted">
			Masuk ke CubyCloud dengan username atau email dan password.
		</Card.Description>
	</Card.Header>
	<Card.Content class="space-y-4">
		{#if forgotInfo}
			<Alert.Root class="border-subtle bg-app">
				<KeyRound class="size-4 stroke-2 text-secondary-accent" />
				<Alert.Title class="text-main">Pemulihan tanpa email</Alert.Title>
				<Alert.Description>
					CubyCloud v1.0 tidak mengirim reset password lewat SMTP. Super Admin memulihkan akses
					dengan Emergency Recovery Key di halaman Pulihkan Akses.
				</Alert.Description>
			</Alert.Root>
		{/if}

		<form class="space-y-4" onsubmit={onSubmit}>
			<div class="space-y-2">
				<Label for="login-identifier">Username atau Email</Label>
				<Input
					id="login-identifier"
					autocomplete="username"
					bind:value={identifier}
					placeholder="admin atau admin@contoh.local"
					aria-invalid={identifierEmpty}
				/>
				{#if identifierEmpty}
					<p class="text-xs text-danger">Username atau email wajib diisi.</p>
				{/if}
			</div>

			<div class="space-y-2">
				<div class="flex items-center justify-between gap-2">
					<Label for="login-password">Password</Label>
					<button
						type="button"
						class="text-xs font-medium text-primary hover:underline"
						onclick={() => {
							forgotInfo = !forgotInfo;
						}}
					>
						Lupa password?
					</button>
				</div>
				<PasswordInput
					id="login-password"
					autocomplete="current-password"
					bind:value={password}
					aria-invalid={passwordEmpty}
				/>
				{#if passwordEmpty}
					<p class="text-xs text-danger">Password wajib diisi.</p>
				{/if}
			</div>

			<Button type="submit" class="w-full">Masuk</Button>
		</form>

		<p class="text-center text-sm text-muted">
			Lupa akses Super Admin?
			<a href="/recovery" class="font-medium text-primary hover:underline">Pulihkan Akses Admin</a>
		</p>
	</Card.Content>
</Card.Root>
