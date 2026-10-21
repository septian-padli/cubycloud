<script lang="ts">
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import * as Tabs from '$lib/components/ui/tabs/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { authAPI } from '$lib/api/auth.js';
	import { goto } from '$app/navigation';
	import { toast } from 'svelte-sonner';

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
			toast.success('Kata sandi berhasil diubah. Sesi Anda dihentikan, silakan masuk kembali.');
			open = false;
			await goto('/login');
		} catch (e) {
			toast.error('Gagal mengubah kata sandi');
		} finally {
			isChangingPassword = false;
		}
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="bg-surface text-main sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title>Pengaturan Akun</Dialog.Title>
			<Dialog.Description>Kelola profil akun dan keamanan Anda.</Dialog.Description>
		</Dialog.Header>

		<Tabs.Root bind:value={activeTab} class="w-full">
			<Tabs.List variant="line" class="grid w-full grid-cols-2">
				<Tabs.Trigger value="profile">Profil</Tabs.Trigger>
				<Tabs.Trigger value="password">Kata Sandi</Tabs.Trigger>
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
						<Label for="profile-username">Nama pengguna</Label>
						<Input id="profile-username" value={username} disabled class="text-muted" />
					</div>

					<div class="grid gap-2">
						<Label for="profile-display-name">Nama tampilan</Label>
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
							<Label for="profile-current-password">Kata sandi saat ini (diperlukan)</Label>
							<Input
								id="profile-current-password"
								type="password"
								bind:value={currentPasswordDraft}
								placeholder="Masukkan kata sandi saat ini"
								autocomplete="current-password"
							/>
							<p class="text-xs text-muted">
								Diperlukan untuk memverifikasi perubahan alamat email.
							</p>
						</div>
					{/if}

					<Dialog.Footer class="mt-2">
						<Button type="button" variant="secondary" onclick={() => (open = false)}>Batal</Button>
						<Button type="submit" disabled={!canSubmitProfile}>
							{isSavingProfile ? 'Menyimpan...' : 'Simpan'}
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
						<Label for="change-current-pass">Kata sandi saat ini</Label>
						<Input
							id="change-current-pass"
							type="password"
							bind:value={currentPass}
							placeholder="Masukkan kata sandi saat ini"
							autocomplete="current-password"
						/>
					</div>

					<div class="grid gap-2">
						<Label for="change-new-pass">Kata sandi baru</Label>
						<Input
							id="change-new-pass"
							type="password"
							bind:value={newPass}
							placeholder="Minimal 8 karakter"
							autocomplete="new-password"
						/>
					</div>

					<div class="grid gap-2">
						<Label for="change-confirm-pass">Konfirmasi kata sandi baru</Label>
						<Input
							id="change-confirm-pass"
							type="password"
							bind:value={confirmPass}
							placeholder="Ulangi kata sandi baru"
							autocomplete="new-password"
						/>
						{#if confirmPass.length > 0 && newPass !== confirmPass}
							<p class="text-xs text-destructive">Kata sandi tidak cocok</p>
						{/if}
					</div>

					<Dialog.Footer class="mt-2">
						<Button type="button" variant="secondary" onclick={() => (open = false)}>Batal</Button>
						<Button type="submit" disabled={!canSubmitPassword}>
							{isChangingPassword ? 'Mengubah...' : 'Ubah Kata Sandi'}
						</Button>
					</Dialog.Footer>
				</form>
			</Tabs.Content>
		</Tabs.Root>
	</Dialog.Content>
</Dialog.Root>
