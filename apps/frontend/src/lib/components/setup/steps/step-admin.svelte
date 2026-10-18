<script lang="ts">
	import type { AdminForm } from '$lib/types/setup.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import * as Alert from '$lib/components/ui/alert/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import PasswordInput from '$lib/components/password-input.svelte';
	import { ShieldAlert } from '@lucide/svelte';

	let {
		admin = $bindable()
	}: {
		admin: AdminForm;
	} = $props();

	const passwordTooShort = $derived(
		admin.password.length > 0 && admin.password.length < 8
	);
	const passwordMismatch = $derived(
		admin.confirmPassword.length > 0 && admin.password !== admin.confirmPassword
	);
</script>

<Card.Root class="rounded-xl border-subtle bg-surface shadow-none">
	<Card.Header>
		<Card.Title class="text-lg font-semibold text-main">Akun Super Admin</Card.Title>
		<Card.Description class="text-sm text-muted">
			Buat akun administrator pertama. Password minimal 8 karakter.
		</Card.Description>
	</Card.Header>
	<Card.Content class="space-y-4">
		<div class="space-y-2">
			<Label for="setup-username">Username</Label>
			<Input
				id="setup-username"
				autocomplete="username"
				bind:value={admin.username}
				placeholder="admin"
			/>
		</div>
		<div class="space-y-2">
			<Label for="setup-email">Email</Label>
			<Input
				id="setup-email"
				type="email"
				autocomplete="email"
				bind:value={admin.email}
				placeholder="admin@contoh.local"
			/>
		</div>
		<div class="space-y-2">
			<Label for="setup-password">Password</Label>
			<PasswordInput
				id="setup-password"
				autocomplete="new-password"
				bind:value={admin.password}
				aria-invalid={passwordTooShort}
			/>
			{#if passwordTooShort}
				<p class="text-xs text-danger">Password minimal 8 karakter.</p>
			{/if}
		</div>
		<div class="space-y-2">
			<Label for="setup-confirm">Konfirmasi password</Label>
			<PasswordInput
				id="setup-confirm"
				autocomplete="new-password"
				bind:value={admin.confirmPassword}
				aria-invalid={passwordMismatch}
			/>
			{#if passwordMismatch}
				<p class="text-xs text-danger">Password tidak cocok.</p>
			{/if}
		</div>

		<Alert.Root variant="default">
			<ShieldAlert class="size-4 stroke-2" />
			<Alert.Title>Akun pertama</Alert.Title>
			<Alert.Description>
				Super Admin memiliki akses penuh ke pengaturan penyimpanan dan pengguna.
			</Alert.Description>
		</Alert.Root>
	</Card.Content>
</Card.Root>
