<script lang="ts">
	import { page } from '$app/state';
	import { toast } from 'svelte-sonner';
	import { Loader2 } from '@lucide/svelte';
	import UserProfileView from '$lib/components/drive/user-profile-view.svelte';
	import { getDriveChrome } from '$lib/drive-context.svelte.js';
	import { authAPI } from '$lib/api/auth.js';
	import type { AdminUser } from '$lib/types/admin.js';
	import { locale } from '$lib/locale.svelte.js';
	import { ConfirmDialog } from '$lib/components/ui/confirm-dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import { Link2 } from '@lucide/svelte';

	const tx = (id: string, en: string) => (locale.language === 'id' ? id : en);
	const chrome = getDriveChrome();
	let loading = $state(true);
	let errorState = $state<string | null>(null);
	type ProfileUser = AdminUser & { quotaBytes: number };

	let profileUser = $state<ProfileUser | null>(null);

	const targetUserId = $derived(page.params.userId);
	const isSelf = $derived(chrome.user.id === targetUserId);
	let googleStatus = $state<{ linked: boolean; email: string }>({ linked: false, email: '' });
	let showUnlinkConfirm = $state(false);
	async function loadGoogleStatus() {
		if (!isSelf) return;
		const result = await authAPI.getGoogleAuthStatus();
		if (result.data) googleStatus = result.data;
	}
	function startGoogleLink() {
		window.location.href = `/api/v1/auth/google/start?action=link&redirect_uri=${encodeURIComponent(`${window.location.origin}/api/v1/auth/google/callback`)}`;
	}
	async function unlinkGoogle() {
		const result = await authAPI.unlinkGoogle();
		if (result.error) toast.error(result.error);
		else { googleStatus = { linked: false, email: '' }; toast.success(tx('Akun Google diputus', 'Google account disconnected')); }
	}

	async function loadProfile(id: string) {
		loading = true;
		errorState = null;
		try {
			const res = await authAPI.getUserProfile(id);
			if (res.error) {
				errorState = res.error;
				return;
			}
			if (res.data) {
				const u = res.data;
				const role = u.role.toLowerCase() === 'super_admin' ? 'super_admin' : 'user';
				const usedGb = (u.used_bytes ?? 0) / (1024 * 1024 * 1024);
				const quotaGb =
					role === 'super_admin' || u.quota_bytes < 0
						? undefined
						: u.quota_bytes / (1024 * 1024 * 1024);
				profileUser = {
					id: u.id,
					username: u.username,
					displayName: u.display_name || u.username,
					email: u.email,
					role,
					quotaMode: role === 'super_admin' || u.quota_bytes < 0 ? 'unlimited' : 'override_gb',
					quotaGb,
					quotaBytes: u.quota_bytes,
					usedGb,
					createdAt: ''
				};
			}
		} catch (e) {
			errorState = tx('Gagal memuat profil pengguna', 'Failed to load user profile');
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		if (targetUserId) void loadProfile(targetUserId).then(() => void loadGoogleStatus());
	});

	const viewUser = $derived(
		profileUser
			? isSelf
				? {
						...profileUser,
						displayName: chrome.user.displayName || profileUser.displayName,
						email: chrome.user.email || profileUser.email
					}
				: profileUser
			: null
	);

	const quota = $derived(
		viewUser
			? {
					usedGb: viewUser.usedGb,
					limitGb:
						viewUser.role === 'super_admin' || viewUser.quotaBytes < 0
							? null
							: viewUser.quotaBytes / (1024 * 1024 * 1024)
				}
			: { usedGb: 0, limitGb: null }
	);

	async function onSaveProfile(next: {
		displayName: string;
		email: string;
		currentPassword?: string;
	}): Promise<boolean> {
		const res = await authAPI.updateProfile({
			display_name: next.displayName,
			email: next.email,
			current_password: next.currentPassword
		});
		if (res.error || !res.data) {
			toast.error(res.error || tx('Gagal memperbarui profil', 'Failed to update profile'));
			return false;
		}
		chrome.updateProfile({
			displayName: res.data.display_name || next.displayName,
			email: res.data.email
		});
		if (profileUser) {
			profileUser.displayName = res.data.display_name || next.displayName;
			profileUser.email = res.data.email;
		}
		toast.success(tx('Profil berhasil diperbarui', 'Profile updated successfully'));
		return true;
	}
</script>

<svelte:head>
	<title>{viewUser?.displayName || tx('Profil', 'Profile')} — CubyCloud</title>
	<meta
		name="description"
		content={tx(
			'Profil pengguna dan statistik penyimpanan My Storage.',
			'User profile and My Storage usage statistics.'
		)}
	/>
</svelte:head>

{#if loading}
	<div class="flex items-center justify-center py-16 text-muted">
		<Loader2 class="mr-2 size-6 animate-spin" />
		<span>{tx('Memuat profil...', 'Loading profile...')}</span>
	</div>
{:else if errorState}
	<div
		class="rounded-xl border border-destructive/20 bg-destructive/5 p-8 text-center text-destructive"
	>
		<p class="text-base font-semibold">
			{tx('Akses Ditolak atau Pengguna Tidak Ditemukan', 'Access Denied or User Not Found')}
		</p>
		<p class="mt-1 text-sm text-muted">{errorState}</p>
	</div>
	{:else if viewUser}
		<div class="space-y-5">
			<UserProfileView user={viewUser} {quota} editable={isSelf} onSaveProfile={isSelf ? onSaveProfile : undefined} />
			{#if isSelf}
				<Card.Root class="rounded-xl border-subtle bg-surface shadow-none">
					<Card.Header><Card.Title>{tx('Akun Google', 'Google Account')}</Card.Title><Card.Description>{tx('Hubungkan akun Google untuk masuk dengan Google.', 'Connect a Google account to sign in with Google.')}</Card.Description></Card.Header>
					<Card.Content class="flex flex-wrap items-center justify-between gap-4">
						{#if googleStatus.linked}
							<div><p class="font-medium text-main">{googleStatus.email}</p><p class="text-sm text-success">{tx('Terhubung', 'Connected')}</p></div>
							<Button variant="destructive" onclick={() => (showUnlinkConfirm = true)}>{tx('Putuskan Google', 'Disconnect Google')}</Button>
						{:else}
							<Button class="gap-2" onclick={startGoogleLink}><Link2 class="size-4" />{tx('Hubungkan Google', 'Connect Google')}</Button>
						{/if}
					</Card.Content>
				</Card.Root>
			{/if}
		</div>
		<ConfirmDialog bind:open={showUnlinkConfirm} title={tx('Putuskan akun Google?', 'Disconnect Google account?')} description={tx('Anda harus memiliki kata sandi atau akses pemulihan untuk mempertahankan akses.', 'You must have a password or recovery access to retain account access.')} confirmText={tx('Putuskan', 'Disconnect')} cancelText={tx('Batal', 'Cancel')} variant="destructive" onConfirm={unlinkGoogle} />
	{/if}
