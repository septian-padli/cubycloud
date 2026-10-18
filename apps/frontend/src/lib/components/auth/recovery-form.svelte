<script lang="ts">
	import { goto } from '$app/navigation';
	import * as Alert from '$lib/components/ui/alert/index.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import PasswordInput from '$lib/components/password-input.svelte';
	import RecoverySuccessDialog from './recovery-success-dialog.svelte';
	import { authAPI } from '$lib/api/auth.js';
	import { toast } from 'svelte-sonner';
	import { KeyRound } from '@lucide/svelte';

	let recoveryKey = $state('');
	let newRecoveryKey = $state('');
	let password = $state('');
	let confirmPassword = $state('');
	let submitted = $state(false);
	let loading = $state(false);
	let errorMessage = $state('');
	let successOpen = $state(false);

	const KEY_FORMAT = /^[A-Z0-9]{5}(?:-[A-Z0-9]{5}){5}$/;
	const keyInvalid = $derived(submitted && !KEY_FORMAT.test(recoveryKey));
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
		KEY_FORMAT.test(recoveryKey) &&
			password.length >= 8 &&
			password === confirmPassword
	);

	function onKeyInput(e: Event) {
		const target = e.currentTarget as HTMLInputElement;
		const raw = target.value.replace(/[-\s]/g, '').replace(/[^A-Z0-9]/gi, '').slice(0, 30).toUpperCase();
		recoveryKey = raw.match(/.{1,5}/g)?.join('-') ?? '';
		if (target.value !== recoveryKey) target.value = recoveryKey;
	}

	async function onSubmit(e: Event) {
		e.preventDefault();
		submitted = true;
		errorMessage = '';
		if (!canSubmit || loading) return;

		loading = true;
		try {
			const res = await authAPI.resetPassword({
				username: 'admin',
				recovery_key: recoveryKey,
				new_password: password
			});

			if (res.error || !res.data) {
				errorMessage = res.error ?? 'Respons pemulihan tidak valid';
				toast.error('Gagal memulihkan akses', { description: errorMessage });
				return;
			}

			newRecoveryKey = res.data.new_recovery_key;
			toast.success('Password berhasil direset');
			successOpen = true;
		} finally {
			loading = false;
		}
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
			Reset password Super Admin dengan Emergency Recovery Key 6 × 5 karakter.
		</Card.Description>
	</Card.Header>
	<Card.Content class="space-y-4">
		<Alert.Root variant="destructive">
			<KeyRound class="size-4 stroke-2" />
			<Alert.Title>Tanpa email recovery</Alert.Title>
			<Alert.Description>
			CubyCloud tidak mengirim email pemulihan. Gunakan Emergency Recovery Key terbaru yang disimpan saat setup.
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
					maxlength={35}
					placeholder="XXXXX-XXXXX-XXXXX-XXXXX-XXXXX-XXXXX"
					value={recoveryKey}
					aria-invalid={keyInvalid}
					oninput={onKeyInput}
				/>
				<p class="text-xs text-muted">{recoveryKey.replaceAll('-', '').length}/30 karakter</p>
				{#if keyInvalid}
					<p class="text-xs text-danger">Masukkan kunci 6 kelompok, masing-masing 5 karakter alfanumerik.</p>
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

			{#if errorMessage}
				<p class="text-xs text-danger">{errorMessage}</p>
			{/if}

			<Button type="submit" class="w-full" disabled={loading}>
				{loading ? 'Memproses reset...' : 'Reset Password Super Admin'}
			</Button>
		</form>

		<p class="text-center text-sm text-muted">
			Sudah ingat password?
			<a href="/login" class="font-medium text-primary hover:underline">Kembali ke Masuk</a>
		</p>
	</Card.Content>
</Card.Root>

<RecoverySuccessDialog bind:open={successOpen} recoveryKey={newRecoveryKey} {onContinue} />
