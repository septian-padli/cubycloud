<script lang="ts">
	import { goto } from '$app/navigation';
	import * as Alert from '$lib/components/ui/alert/index.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import PasswordInput from '$lib/components/password-input.svelte';
	import RecoverySuccessDialog from './recovery-success-dialog.svelte';
	import { KeyRound } from '@lucide/svelte';

	let recoveryKey = $state('');
	let password = $state('');
	let confirmPassword = $state('');
	let submitted = $state(false);
	let successOpen = $state(false);

	const HEX_64 = /^[0-9A-Fa-f]{64}$/;

	const keyInvalid = $derived(
		submitted && (recoveryKey.length !== 64 || !HEX_64.test(recoveryKey))
	);
	const passwordTooShort = $derived(
		(submitted || password.length > 0) && password.length > 0 && password.length < 8
	);
	const passwordEmpty = $derived(submitted && password.length === 0);
	const passwordMismatch = $derived(
		(submitted || confirmPassword.length > 0) &&
			confirmPassword.length > 0 &&
			password !== confirmPassword
	);
	const confirmEmpty = $derived(submitted && confirmPassword.length === 0);

	const canSubmit = $derived(
		HEX_64.test(recoveryKey) &&
			password.length >= 8 &&
			password === confirmPassword
	);

	function onKeyInput(e: Event) {
		const target = e.currentTarget as HTMLInputElement;
		const filtered = target.value.replace(/[^0-9A-Fa-f]/g, '').slice(0, 64).toUpperCase();
		recoveryKey = filtered;
		if (target.value !== filtered) {
			target.value = filtered;
		}
	}

	function onSubmit(e: Event) {
		e.preventDefault();
		submitted = true;
		if (!canSubmit) return;
		successOpen = true;
	}

	function onContinue() {
		successOpen = false;
		void goto('/login');
	}
</script>

<Card.Root class="rounded-xl border-subtle bg-surface shadow-none">
	<Card.Header>
		<Card.Title class="text-lg font-semibold text-main">Pulihkan Akses Admin</Card.Title>
		<Card.Description class="text-sm text-muted">
			Reset password Super Admin dengan Emergency Recovery Key 64 karakter.
		</Card.Description>
	</Card.Header>
	<Card.Content class="space-y-4">
		<Alert.Root variant="destructive" class="border-danger/30">
			<KeyRound class="size-4 stroke-2" />
			<Alert.Title>Tanpa email recovery</Alert.Title>
			<Alert.Description>
				CubyCloud v1.0 tidak mengirim email pemulihan. Hanya kunci hex dari setup yang dapat
				memulihkan akses Super Admin.
			</Alert.Description>
		</Alert.Root>

		<form class="space-y-4" onsubmit={onSubmit}>
			<div class="space-y-2">
				<Label for="recovery-key-input">Emergency Recovery Key</Label>
				<Input
					id="recovery-key-input"
					class="font-mono text-xs"
					autocomplete="off"
					spellcheck={false}
					maxlength={64}
					placeholder="64 karakter hexadecimal"
					value={recoveryKey}
					aria-invalid={keyInvalid}
					oninput={onKeyInput}
				/>
				<p class="text-xs text-muted">{recoveryKey.length}/64 karakter</p>
				{#if keyInvalid}
					<p class="text-xs text-danger">Kunci harus tepat 64 karakter hexadecimal.</p>
				{/if}
			</div>

			<div class="space-y-2">
				<Label for="recovery-password">Password baru</Label>
				<PasswordInput
					id="recovery-password"
					autocomplete="new-password"
					bind:value={password}
					aria-invalid={passwordEmpty || passwordTooShort}
				/>
				{#if passwordEmpty}
					<p class="text-xs text-danger">Password wajib diisi.</p>
				{:else if passwordTooShort}
					<p class="text-xs text-danger">Password minimal 8 karakter.</p>
				{/if}
			</div>

			<div class="space-y-2">
				<Label for="recovery-confirm">Konfirmasi password</Label>
				<PasswordInput
					id="recovery-confirm"
					autocomplete="new-password"
					bind:value={confirmPassword}
					aria-invalid={confirmEmpty || passwordMismatch}
				/>
				{#if confirmEmpty}
					<p class="text-xs text-danger">Konfirmasi password wajib diisi.</p>
				{:else if passwordMismatch}
					<p class="text-xs text-danger">Password tidak cocok.</p>
				{/if}
			</div>

			<Button type="submit" class="w-full">Reset Password Super Admin</Button>
		</form>

		<p class="text-center text-sm text-muted">
			Sudah ingat password?
			<a href="/login" class="font-medium text-primary hover:underline">Kembali ke Masuk</a>
		</p>
	</Card.Content>
</Card.Root>

<RecoverySuccessDialog bind:open={successOpen} {onContinue} />
