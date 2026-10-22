<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import * as Alert from '$lib/components/ui/alert/index.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import PasswordInput from '$lib/components/password-input.svelte';
	import { authAPI } from '$lib/api/auth.js';
	import { setMockLoggedIn } from '$lib/mocks/auth-session.js';
	import { locale } from '$lib/locale.svelte.js';
	import { toast } from 'svelte-sonner';
	import { KeyRound } from '@lucide/svelte';
	const tx = (id: string, en: string) => (locale.language === 'id' ? id : en);

	let identifier = $state('');
	let password = $state('');
	let submitted = $state(false);
	let loading = $state(false);
	let errorMessage = $state('');
	let forgotInfo = $state(false);
	let googleEnabled = $state(false);

	onMount(async () => {
		googleEnabled = (await authAPI.getProviders()).data?.google.enabled ?? false;
	});
	const identifierEmpty = $derived(submitted && identifier.trim().length === 0);
	const passwordEmpty = $derived(submitted && password.length === 0);
	const canSubmit = $derived(identifier.trim().length > 0 && password.length > 0);

	async function onSubmit(e: Event) {
		e.preventDefault();
		submitted = true;
		errorMessage = '';
		if (!canSubmit || loading) return;

		loading = true;
		try {
			const res = await authAPI.login(identifier.trim(), password);
			if (res.error) {
				errorMessage = res.error;
				toast.error(tx('Gagal masuk', 'Sign-in failed'), { description: res.error });
				return;
			}

			locale.setAccount(res.data?.user.id ?? null);
			setMockLoggedIn(true);
			toast.success(tx('Berhasil masuk', 'Signed in successfully'));
			const redirect = page.url.searchParams.get('redirect');
			const safe =
				redirect && redirect.startsWith('/') && !redirect.startsWith('//')
					? redirect
					: '/drive/home';
			void goto(safe);
		} finally {
			loading = false;
		}
	}
</script>

<Card.Root class="rounded-xl border-subtle bg-surface shadow-none">
	<Card.Header>
		<Card.Title class="text-lg font-semibold text-main">{tx('Masuk', 'Sign in')}</Card.Title>
		<Card.Description class="text-sm text-muted">
			{tx(
				'Masuk ke CubyCloud dengan nama pengguna atau email dan kata sandi.',
				'Sign in to CubyCloud with your username or email and password.'
			)}
		</Card.Description>
	</Card.Header>
	<Card.Content class="space-y-4">
		{#if forgotInfo}
			<Alert.Root variant="default">
				<KeyRound class="size-4 stroke-2" />
				<Alert.Title>{tx('Pemulihan tanpa email', 'Recovery without email')}</Alert.Title>
				<Alert.Description>
					{tx(
						'CubyCloud v1.0 tidak mengirim email untuk pengaturan ulang kata sandi melalui SMTP. Super Admin memulihkan akses dengan Emergency Recovery Key di halaman Pulihkan Akses.',
						'CubyCloud v1.0 does not send password resets by email. The Super Admin can restore access with the Emergency Recovery Key on the Access Recovery page.'
					)}
				</Alert.Description>
			</Alert.Root>
		{/if}

		{#if page.url.searchParams.get('error') === 'google_not_linked'}
			<Alert.Root variant="destructive">
				<Alert.Title>{tx('Akun Google belum terhubung', 'Google account not linked')}</Alert.Title>
				<Alert.Description>{tx('Masuk dengan kata sandi, lalu hubungkan akun Google dari halaman Profil.', 'Sign in with your password, then connect your Google account from your Profile.')}</Alert.Description>
			</Alert.Root>
		{/if}
		{#if googleEnabled}
			<Button type="button" variant="secondary" class="w-full gap-2" onclick={() => (window.location.href = `/api/v1/auth/google/start?action=login&redirect_uri=${encodeURIComponent(`${window.location.origin}/api/v1/auth/google/callback`)}`)}>
				<svg aria-hidden="true" viewBox="0 0 48 48" class="size-5"><path fill="#4285F4" d="M43.6 24.5c0-1.4-.1-2.8-.4-4.1H24v7.8h11a9.4 9.4 0 0 1-4.1 6.2v5.1h6.6c3.9-3.6 6.1-8.8 6.1-15Z"/><path fill="#34A853" d="M24 44c5.5 0 10.1-1.8 13.5-4.8l-6.6-5.1c-1.8 1.2-4.1 2-6.9 2-5.3 0-9.8-3.6-11.4-8.4H5.8v5.3A20 20 0 0 0 24 44Z"/><path fill="#FBBC05" d="M12.6 27.7a12 12 0 0 1 0-7.4V15H5.8a20 20 0 0 0 0 18l6.8-5.3Z"/><path fill="#EA4335" d="M24 11.9c3 0 5.7 1 7.8 3.1l5.8-5.8C34.1 5.8 29.5 4 24 4A20 20 0 0 0 5.8 15l6.8 5.3c1.6-4.8 6.1-8.4 11.4-8.4Z"/></svg>
				{tx('Masuk dengan Google', 'Sign in with Google')}
			</Button>
		{/if}
		<form class="space-y-4" onsubmit={onSubmit}>
			<div class="space-y-2">
				<Label for="login-identifier">{tx('Nama pengguna atau email', 'Username or email')}</Label>
				<Input
					id="login-identifier"
					aria-label="Username atau Email"
					autocomplete="username"
					bind:value={identifier}
					placeholder={tx('admin atau admin@contoh.local', 'admin or admin@example.local')}
					aria-invalid={identifierEmpty}
				/>
				{#if identifierEmpty}
					<p class="text-xs text-danger">
						{tx('Nama pengguna atau email wajib diisi.', 'Username or email is required.')}
					</p>
				{/if}
			</div>

			<div class="space-y-2">
				<div class="flex items-center justify-between gap-2">
					<Label for="login-password">{tx('Kata sandi', 'Password')}</Label>
					<button
						type="button"
						class="text-xs font-medium text-primary hover:underline"
						onclick={() => {
							forgotInfo = !forgotInfo;
						}}
					>
						{tx('Lupa kata sandi?', 'Forgot password?')}
					</button>
				</div>
				<PasswordInput
					id="login-password"
					autocomplete="current-password"
					bind:value={password}
					aria-invalid={passwordEmpty}
				/>
				{#if passwordEmpty}
					<p class="text-xs text-danger">{tx('Kata sandi wajib diisi.', 'Password is required.')}</p>
				{/if}
			</div>

			{#if errorMessage}
				<p class="text-xs text-danger">{errorMessage}</p>
			{/if}

			<Button type="submit" class="w-full" disabled={loading}>
				{loading ? tx('Sedang masuk...', 'Signing in...') : tx('Masuk', 'Sign in')}
			</Button>
		</form>

		<p class="text-center text-sm text-muted">
			{tx('Lupa akses Super Admin?', 'Forgot Super Admin access?')}
			<a href="/recovery" class="font-medium text-primary hover:underline"
				>{tx('Pulihkan Akses Admin', 'Recover Admin Access')}</a
			>
		</p>
	</Card.Content>
</Card.Root>
