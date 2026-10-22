<script lang="ts">
	import { goto } from '$app/navigation';
	import { KeyRound } from '@lucide/svelte';
	import * as Alert from '$lib/components/ui/alert/index.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import PasswordInput from '$lib/components/password-input.svelte';
	import RecoverySuccessDialog from './recovery-success-dialog.svelte';
	import { authAPI } from '$lib/api/auth.js';
	import { toast } from 'svelte-sonner';
	import { locale } from '$lib/locale.svelte.js';
	const tx = (id: string, en: string) => (locale.language === 'id' ? id : en);

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
		KEY_FORMAT.test(recoveryKey) && password.length >= 8 && password === confirmPassword
	);

	function onKeyInput(e: Event) {
		const target = e.currentTarget as HTMLInputElement;
		const raw = target.value
			.replace(/[-\s]/g, '')
			.replace(/[^A-Z0-9]/gi, '')
			.slice(0, 30)
			.toUpperCase();
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
				errorMessage =
					res.error ?? tx('Respons pemulihan tidak valid', 'Invalid recovery response');
				toast.error(tx('Gagal memulihkan akses', 'Access recovery failed'), {
					description: errorMessage
				});
				return;
			}

			newRecoveryKey = res.data.new_recovery_key;
			toast.success(tx('Kata sandi berhasil diatur ulang', 'Password reset successfully'));
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
		<Card.Title class="text-lg font-semibold text-main"
			>{tx('Pulihkan Akses Admin', 'Recover Admin Access')}</Card.Title
		>
		<Card.Description class="text-sm text-muted">
			{tx(
				'Atur ulang kata sandi Super Admin dengan Emergency Recovery Key yang terdiri dari 6 × 5 karakter.',
				'Reset the Super Admin password with the 6 × 5-character Emergency Recovery Key.'
			)}
		</Card.Description>
	</Card.Header>
	<Card.Content class="space-y-4">
		<Alert.Root variant="destructive">
			<KeyRound class="size-4 stroke-2" />
			<Alert.Title>{tx('Tanpa email pemulihan', 'No recovery email')}</Alert.Title>
			<Alert.Description>
				{tx(
					'CubyCloud tidak mengirim email pemulihan. Gunakan Emergency Recovery Key terbaru yang disimpan saat penyiapan.',
					'CubyCloud does not send recovery emails. Use the latest Emergency Recovery Key saved during setup.'
				)}
			</Alert.Description>
		</Alert.Root>

		<form class="space-y-4" onsubmit={onSubmit}>
			<div class="space-y-2">
				<Label for="recovery-key-input"
					>{tx('Emergency Recovery Key', 'Emergency Recovery Key')}</Label
				>
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
				<p class="text-xs text-muted">
					{recoveryKey.replaceAll('-', '').length}{tx('/30 karakter', '/30 characters')}
				</p>
				{#if keyInvalid}
					<p class="text-xs text-danger">
						{tx(
							'Masukkan kunci 6 kelompok, masing-masing 5 karakter alfanumerik.',
							'Enter a key with 6 groups of 5 alphanumeric characters each.'
						)}
					</p>
				{/if}
			</div>

			<div class="space-y-2">
				<Label for="recovery-password">{tx('Kata sandi baru', 'New password')}</Label>
				<PasswordInput
					id="recovery-password"
					autocomplete="new-password"
					bind:value={password}
					aria-invalid={passwordEmpty || passwordTooShort}
				/>
				{#if passwordEmpty}
					<p class="text-xs text-danger">{tx('Kata sandi wajib diisi.', 'Password is required.')}</p>
				{:else if passwordTooShort}
					<p class="text-xs text-danger">
						{tx('Kata sandi minimal 8 karakter.', 'Password must be at least 8 characters.')}
					</p>
				{/if}
			</div>

			<div class="space-y-2">
				<Label for="recovery-confirm">{tx('Konfirmasi kata sandi', 'Confirm password')}</Label>
				<PasswordInput
					id="recovery-confirm"
					autocomplete="new-password"
					bind:value={confirmPassword}
					aria-invalid={confirmEmpty || passwordMismatch}
				/>
				{#if confirmEmpty}
					<p class="text-xs text-danger">
						{tx('Konfirmasi kata sandi wajib diisi.', 'Password confirmation is required.')}
					</p>
				{:else if passwordMismatch}
					<p class="text-xs text-danger">
						{tx('Kata sandi tidak cocok.', 'Passwords do not match.')}
					</p>
				{/if}
			</div>

			{#if errorMessage}
				<p class="text-xs text-danger">{errorMessage}</p>
			{/if}

			<Button type="submit" class="w-full" disabled={loading}>
				{loading
					? tx('Memproses pengaturan ulang...', 'Resetting password...')
					: tx('Atur Ulang Kata Sandi Super Admin', 'Reset Super Admin Password')}
			</Button>
		</form>

		<p class="text-center text-sm text-muted">
			{tx('Sudah ingat kata sandi?', 'Remember your password?')}
			<a href="/login" class="font-medium text-primary hover:underline"
				>{tx('Kembali ke Masuk', 'Back to Sign in')}</a
			>
		</p>
	</Card.Content>
</Card.Root>

<RecoverySuccessDialog bind:open={successOpen} recoveryKey={newRecoveryKey} {onContinue} />
