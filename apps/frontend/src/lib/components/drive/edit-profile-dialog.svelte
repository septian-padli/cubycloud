<script lang="ts">
	import { locale } from '$lib/locale.svelte.js';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import * as Tabs from '$lib/components/ui/tabs/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { authAPI } from '$lib/api/auth.js';
	import { goto } from '$app/navigation';
	import { toast } from 'svelte-sonner';
	const tx = (id: string, en: string) => (locale.language === 'id' ? id : en);

	let {
		open = $bindable(false),
		displayName = '',
		email = '',
		username = '',
		onSave
	}: {
		open?: boolean;
		displayName?: string;
		email?: string;
		username?: string;
		onSave: (next: {
			displayName: string;
			email: string;
			currentPassword?: string;
		}) => Promise<boolean | void> | void;
	} = $props();

	let activeTab = $state('profile');

	// Profile fields
	let nameDraft = $state('');
	let emailDraft = $state('');
	let currentPasswordDraft = $state('');
	let isSavingProfile = $state(false);

	// Password fields
	let currentPass = $state('');
	let newPass = $state('');
	let confirmPass = $state('');
	let isChangingPassword = $state(false);

	const emailChanged = $derived(emailDraft.trim().toLowerCase() !== email.trim().toLowerCase());

	const canSubmitProfile = $derived(
		nameDraft.trim().length > 0 &&
			emailDraft.trim().length > 0 &&
			(!emailChanged || currentPasswordDraft.length > 0) &&
			!isSavingProfile
	);

	const canSubmitPassword = $derived(
		currentPass.length > 0 && newPass.length >= 8 && newPass === confirmPass && !isChangingPassword
	);

	$effect(() => {
		if (open) {
			nameDraft = displayName;
			emailDraft = email;
			currentPasswordDraft = '';
			currentPass = '';
			newPass = '';
			confirmPass = '';
			activeTab = 'profile';
		}
	});

	async function submitProfile() {
		if (!canSubmitProfile) return;
		isSavingProfile = true;
		try {
			const saved = await onSave({
				displayName: nameDraft.trim(),
				email: emailDraft.trim(),
				currentPassword: emailChanged ? currentPasswordDraft : undefined
			});
			if (saved !== false) open = false;
		} finally {
			isSavingProfile = false;
		}
	}

	async function submitChangePassword() {
		if (!canSubmitPassword) return;
		isChangingPassword = true;
		try {
			const res = await authAPI.changePassword({
				current_password: currentPass,
				new_password: newPass,
				confirm_password: confirmPass
			});
			if (res.error) {
				toast.error(res.error);
				return;
			}
			toast.success(
				tx(
					'Kata sandi berhasil diubah. Sesi Anda dihentikan, silakan masuk kembali.',
					'Password changed successfully. Your session has ended; please sign in again.'
				)
			);
			open = false;
			await goto('/login');
		} catch (e) {
			toast.error(tx('Gagal mengubah kata sandi', 'Failed to change password'));
		} finally {
			isChangingPassword = false;
		}
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="bg-surface text-main sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title>{tx('Pengaturan Akun', 'Account settings')}</Dialog.Title>
			<Dialog.Description
				>{tx(
					'Kelola profil akun dan keamanan Anda.',
					'Manage your account profile and security.'
				)}</Dialog.Description
			>
		</Dialog.Header>

		<Tabs.Root bind:value={activeTab} class="w-full">
			<Tabs.List variant="line" class="grid w-full grid-cols-2">
				<Tabs.Trigger value="profile">{tx('Profil', 'Profile')}</Tabs.Trigger>
				<Tabs.Trigger value="password">{tx('Kata Sandi', 'Password')}</Tabs.Trigger>
			</Tabs.List>

			<Tabs.Content value="profile" class="mt-4">
				<form
					class="grid gap-4"
					onsubmit={(e) => {
						e.preventDefault();
						submitProfile();
					}}
				>
					<div class="grid gap-2">
						<Label for="profile-username">{tx('Nama pengguna', 'Username')}</Label>
						<Input id="profile-username" value={username} disabled class="text-muted" />
					</div>

					<div class="grid gap-2">
						<Label for="profile-display-name">{tx('Nama tampilan', 'Display name')}</Label>
						<Input
							id="profile-display-name"
							bind:value={nameDraft}
							autocomplete="name"
							maxlength={100}
						/>
					</div>

					<div class="grid gap-2">
						<Label for="profile-email">Email</Label>
						<Input
							id="profile-email"
							type="email"
							bind:value={emailDraft}
							autocomplete="email"
							maxlength={254}
						/>
					</div>

					{#if emailChanged}
						<div class="grid gap-2">
							<Label for="profile-current-password"
								>{tx('Kata sandi saat ini (diperlukan)', 'Current password (required)')}</Label
							>
							<Input
								id="profile-current-password"
								type="password"
								bind:value={currentPasswordDraft}
								placeholder={tx('Masukkan kata sandi saat ini', 'Enter your current password')}
								autocomplete="current-password"
							/>
							<p class="text-xs text-muted">
								{tx(
									'Diperlukan untuk memverifikasi perubahan alamat email.',
									'Required to verify your email address change.'
								)}
							</p>
						</div>
					{/if}

					<Dialog.Footer class="mt-2">
						<Button type="button" variant="secondary" onclick={() => (open = false)}
							>{tx('Batal', 'Cancel')}</Button
						>
						<Button type="submit" disabled={!canSubmitProfile}>
							{isSavingProfile ? tx('Menyimpan...', 'Saving...') : tx('Simpan', 'Save')}
						</Button>
					</Dialog.Footer>
				</form>
			</Tabs.Content>

			<Tabs.Content value="password" class="mt-4">
				<form
					class="grid gap-4"
					onsubmit={(e) => {
						e.preventDefault();
						submitChangePassword();
					}}
				>
					<div class="grid gap-2">
						<Label for="change-current-pass">{tx('Kata sandi saat ini', 'Current password')}</Label>
						<Input
							id="change-current-pass"
							type="password"
							bind:value={currentPass}
							placeholder={tx('Masukkan kata sandi saat ini', 'Enter your current password')}
							autocomplete="current-password"
						/>
					</div>

					<div class="grid gap-2">
						<Label for="change-new-pass">{tx('Kata sandi baru', 'New password')}</Label>
						<Input
							id="change-new-pass"
							type="password"
							bind:value={newPass}
							placeholder={tx('Minimal 8 karakter', 'At least 8 characters')}
							autocomplete="new-password"
						/>
					</div>

					<div class="grid gap-2">
						<Label for="change-confirm-pass"
							>{tx('Konfirmasi kata sandi baru', 'Confirm new password')}</Label
						>
						<Input
							id="change-confirm-pass"
							type="password"
							bind:value={confirmPass}
							placeholder={tx('Ulangi kata sandi baru', 'Re-enter your new password')}
							autocomplete="new-password"
						/>
						{#if confirmPass.length > 0 && newPass !== confirmPass}
							<p class="text-xs text-destructive">
								{tx('Kata sandi tidak cocok', 'Passwords do not match')}
							</p>
						{/if}
					</div>

					<Dialog.Footer class="mt-2">
						<Button type="button" variant="secondary" onclick={() => (open = false)}
							>{tx('Batal', 'Cancel')}</Button
						>
						<Button type="submit" disabled={!canSubmitPassword}>
							{isChangingPassword
								? tx('Mengubah...', 'Changing...')
								: tx('Ubah Kata Sandi', 'Change password')}
						</Button>
					</Dialog.Footer>
				</form>
			</Tabs.Content>
		</Tabs.Root>
	</Dialog.Content>
</Dialog.Root>
