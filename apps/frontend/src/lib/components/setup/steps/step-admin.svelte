<script lang="ts">
	import type { AdminForm } from '$lib/types/setup.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import * as Alert from '$lib/components/ui/alert/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import PasswordInput from '$lib/components/password-input.svelte';
	import { ShieldAlert } from '@lucide/svelte';
	import { locale } from '$lib/locale.svelte.js';
	const tx = (id: string, en: string) => (locale.language === 'id' ? id : en);

	let {
		admin = $bindable()
	}: {
		admin: AdminForm;
	} = $props();

	const passwordTooShort = $derived(admin.password.length > 0 && admin.password.length < 8);
	const passwordMismatch = $derived(
		admin.confirmPassword.length > 0 && admin.password !== admin.confirmPassword
	);
</script>

<Card.Root class="rounded-xl border-subtle bg-surface shadow-none">
	<Card.Header>
		<Card.Title class="text-lg font-semibold text-main"
			>{tx('Akun Super Admin', 'Super Admin Account')}</Card.Title
		>
		<Card.Description class="text-sm text-muted">
			{tx(
				'Buat akun administrator pertama. Kata sandi minimal 8 karakter.',
				'Create the first administrator account. Passwords must be at least 8 characters.'
			)}
		</Card.Description>
	</Card.Header>
	<div class="space-y-2">
		<Label for="setup-username">{tx('Nama pengguna', 'Username')}</Label>
		<Input
			id="setup-username"
			autocomplete="username"
			bind:value={admin.username}
			placeholder="admin"
		/>
	</div>
	<div class="space-y-2">
		<Label for="setup-email">{tx('Email', 'Email')}</Label>
		<Input
			id="setup-email"
			type="email"
			autocomplete="email"
			bind:value={admin.email}
			placeholder={tx('admin@contoh.local', 'admin@example.local')}
		/>
	</div>
	<div class="space-y-2">
		<Label for="setup-password">{tx('Kata sandi', 'Password')}</Label>
		<PasswordInput
			id="setup-password"
			autocomplete="new-password"
			bind:value={admin.password}
			aria-invalid={passwordTooShort}
		/>
		{#if passwordTooShort}
			<p class="text-xs text-danger">
				{tx('Kata sandi minimal 8 karakter.', 'Password must be at least 8 characters.')}
			</p>
		{/if}
	</div>
	<div class="space-y-2">
		<Label for="setup-confirm">{tx('Konfirmasi kata sandi', 'Confirm password')}</Label>
		<PasswordInput
			id="setup-confirm"
			autocomplete="new-password"
			bind:value={admin.confirmPassword}
			aria-invalid={passwordMismatch}
		/>
		{#if passwordMismatch}
			<p class="text-xs text-danger">{tx('Kata sandi tidak cocok.', 'Passwords do not match.')}</p>
		{/if}
	</div>

	<Alert.Root variant="default">
		<ShieldAlert class="size-4 stroke-2" />
		<Alert.Title>{tx('Akun pertama', 'First account')}</Alert.Title>
		<Alert.Description>
			{tx(
				'Super Admin memiliki akses penuh ke pengaturan penyimpanan dan pengguna.',
				'The Super Admin has full access to storage and user settings.'
			)}
		</Alert.Description>
	</Alert.Root>
</Card.Root>
