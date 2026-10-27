<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { authAPI, type SetupPasswordValidation } from '$lib/api/auth.js';
	import { locale } from '$lib/locale.svelte.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import PasswordInput from '$lib/components/password-input.svelte';
	import { Mail } from '@lucide/svelte';

	const tx = (id: string, en: string) => (locale.language === 'id' ? id : en);
	let token = $state('');
	let validation = $state<SetupPasswordValidation | null>(null);
	let loading = $state(true);
	let submitting = $state(false);
	let success = $state(false);
	let error = $state('');
	let password = $state('');
	let confirmation = $state('');
	let submitted = $state(false);

	onMount(() => {
		const url = new URL(window.location.href);
		const hashParams = new URLSearchParams(url.hash.slice(1));
		token = hashParams.get('token') ?? url.searchParams.get('token') ?? '';
		url.hash = '';
		url.searchParams.delete('token');
		history.replaceState(null, '', `${url.pathname}${url.search}`);
		void validate();
	});
	async function validate() {
		if (!token) {
			error = tx('Tautan tidak valid atau sudah tidak berlaku.', 'This link is invalid or has expired.');
			loading = false;
			return;
		}
		const result = await authAPI.validateSetupPassword(token);
		validation = result.data?.valid ? result.data : null;
		error = result.error ?? (!validation ? tx('Tautan tidak valid, kedaluwarsa, atau sudah digunakan.', 'This link is invalid, expired, or already used.') : '');
		loading = false;
	}

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		submitted = true;
		error = '';
		if (!token || password.length < 15 || new TextEncoder().encode(password).length > 72 || password !== confirmation || submitting) return;
		submitting = true;
		const result = await authAPI.setupPassword({ token, password, confirm_password: confirmation });
		submitting = false;
		if (result.error) { error = result.error; return; }
		success = true;
	}
</script>

<svelte:head>
	<title>{tx('Atur kata sandi', 'Set password')} | CubyCloud</title>
	<meta name="description" content={tx('Atur kata sandi akun CubyCloud.', 'Set your CubyCloud account password.')} />
</svelte:head>

<Card.Root class="rounded-xl border-subtle bg-surface shadow-none">
	<Card.Header>
		<Card.Title class="text-lg font-semibold text-main">{success ? tx('Kata sandi berhasil diatur', 'Password set successfully') : tx('Atur kata sandi', 'Set your password')}</Card.Title>
		<Card.Description class="text-sm text-muted">
			{#if validation}
				<div class="mt-2.5 flex items-center gap-2 rounded-lg border border-subtle bg-surface-muted/60 px-3 py-2 text-xs text-muted">
					<Mail class="size-4 shrink-0 text-primary" />
					<div class="min-w-0 flex-1 truncate">
						<span class="font-medium text-main">{validation.username}</span>
						<span class="mx-1 text-muted/60">·</span>
						<span class="font-mono font-medium text-primary select-all">{validation.email}</span>
					</div>
				</div>
			{:else}
				{tx('Gunakan tautan undangan atau reset untuk melanjutkan.', 'Use your invitation or reset link to continue.')}
			{/if}
		</Card.Description>
	</Card.Header>
	<Card.Content class="space-y-4">
		{#if loading}
			<p role="status" class="text-sm text-muted">{tx('Memvalidasi tautan…', 'Validating link…')}</p>
		{:else if success}
			<p role="status" class="text-sm text-main">{tx('Kata sandi baru tersimpan. Silakan masuk.', 'Your new password is saved. Sign in to continue.')}</p>
			<Button class="w-full" onclick={() => goto('/login')}>{tx('Ke Halaman Masuk', 'Go to Sign in')}</Button>
		{:else if !validation}
			<p role="alert" class="text-sm text-danger">{error}</p>
			<p class="text-center text-sm text-muted">
				<a href="/login" class="font-medium text-primary hover:underline">{tx('Kembali ke Masuk', 'Back to Sign in')}</a>
			</p>
		{:else}
			<form class="space-y-4" onsubmit={submit}>
				<div class="space-y-2">
					<Label for="setup-password">{tx('Kata sandi baru', 'New password')}</Label>
					<PasswordInput
						id="setup-password"
						autocomplete="new-password"
						bind:value={password}
						aria-invalid={submitted && (password.length < 15 || new TextEncoder().encode(password).length > 72)}
					/>
					<p id="password-rule" class="text-xs text-muted">
						{tx('Minimal 15 karakter.', 'At least 15 characters.')}{#if password.length > 0}{password.length >= 15 ? ' ✓' : ` (${password.length}/15)`}{/if}
					</p>
					{#if submitted && password.length < 15}
						<p class="text-xs text-danger">{tx('Kata sandi harus minimal 15 karakter.', 'Password must be at least 15 characters.')}</p>
					{/if}
					{#if submitted && new TextEncoder().encode(password).length > 72}
						<p class="text-xs text-danger">{tx('Kata sandi tidak boleh melebihi 72 byte.', 'Password must not exceed 72 bytes.')}</p>
					{/if}
				</div>

				<div class="space-y-2">
					<Label for="setup-confirm">{tx('Konfirmasi kata sandi', 'Confirm password')}</Label>
					<PasswordInput
						id="setup-confirm"
						autocomplete="new-password"
						bind:value={confirmation}
						aria-invalid={submitted && confirmation !== password}
					/>
					{#if submitted && confirmation !== password}
						<p class="text-xs text-danger">{tx('Kata sandi tidak cocok.', 'Passwords do not match.')}</p>
					{/if}
				</div>

				{#if error}
					<p role="alert" class="text-xs text-danger">{error}</p>
				{/if}

				<Button type="submit" class="w-full" disabled={submitting}>
					{submitting ? tx('Menyimpan…', 'Saving…') : tx('Simpan Kata Sandi', 'Save Password')}
				</Button>
			</form>

			<p class="text-center text-sm text-muted">
				<a href="/login" class="font-medium text-primary hover:underline">{tx('Kembali ke Masuk', 'Back to Sign in')}</a>
			</p>
		{/if}
	</Card.Content>
</Card.Root>
