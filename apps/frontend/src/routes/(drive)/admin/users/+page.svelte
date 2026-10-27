<script lang="ts">
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import type { AdminUser, DefaultQuotaPolicy } from '$lib/types/admin.js';
	import { MOCK_DEFAULT_QUOTA } from '$lib/mocks/admin-users.js';
	import { adminAPI } from '$lib/api/admin.js';
	import { authAPI } from '$lib/api/auth.js';
	import DrivePageHeader from '$lib/components/drive/drive-page-header.svelte';
	import AdminUsersTable from '$lib/components/admin/admin-users-table.svelte';
	import CreateUserDialog from '$lib/components/admin/create-user-dialog.svelte';
	import EditQuotaDialog from '$lib/components/admin/edit-quota-dialog.svelte';
	import DeleteUserDialog from '$lib/components/admin/delete-user-dialog.svelte';
	import ResetPasswordDialog from '$lib/components/admin/reset-password-dialog.svelte';
	import { Button } from '$lib/components/ui/button/index.js';
	import { UserPlus } from '@lucide/svelte';
	import { locale } from '$lib/locale.svelte.js';
	const tx = (id: string, en: string) => (locale.language === 'id' ? id : en);

	let users = $state<AdminUser[]>([]);
	let defaultQuota = $state<DefaultQuotaPolicy>(MOCK_DEFAULT_QUOTA);
	let loading = $state(true);

	let createOpen = $state(false);
	let editOpen = $state(false);
	let editUser = $state<AdminUser | null>(null);
	let deleteOpen = $state(false);
	let deleteUserTarget = $state<AdminUser | null>(null);
	let currentUserId = $state<string | undefined>(undefined);
	let resetOpen = $state(false);
	let resetUserTarget = $state<AdminUser | null>(null);

	async function loadData() {
		loading = true;
		try {
			const [settingsRes, usersRes, meRes] = await Promise.all([
				adminAPI.getSettings(),
				adminAPI.getUsers(),
				authAPI.getMe()
			]);

			if (meRes.data?.id) {
				currentUserId = meRes.data.id;
			}

			if (settingsRes.data?.default_quota) {
				defaultQuota = {
					mode: settingsRes.data.default_quota.mode,
					gb: settingsRes.data.default_quota.gb
				};
			}

			if (usersRes.data?.users) {
				users = usersRes.data.users.map((u) => {
					const role = u.role === 'SUPER_ADMIN' ? 'super_admin' : 'user';
					let quotaMode: AdminUser['quotaMode'] = 'default';
					if (u.quota_mode === 'UNLIMITED') {
						quotaMode = 'unlimited';
					} else if (u.quota_mode === 'CUSTOM') {
						quotaMode = 'override_gb';
					}
					return {
						id: u.id,
						username: u.username,
						displayName: u.username,
						email: u.email,
						role,
						quotaMode,
						quotaGb: u.quota_gb,
						quotaBytes: u.quota_bytes,
						usedGb: u.used_gb,
						usedBytes: u.used_bytes,
						createdAt: u.created_at,
						status: u.status ?? 'ACTIVE'
					};
				});
			} else if (usersRes.error) {
				toast.error(`${tx('Gagal memuat pengguna', 'Failed to load users')}: ${usersRes.error}`);
			}
		} catch (err) {
			toast.error(tx('Gagal menghubungi server.', 'Failed to reach the server.'));
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		void loadData();
	});


	function onEditQuota(user: AdminUser) {
		if (String(user.role).toLowerCase() === 'super_admin') {
			toast.info(
				tx('Kuota Super Admin selalu Unlimited.', 'Super Admins always have unlimited quota.')
			);
			return;
		}
		editUser = user;
		editOpen = true;
	}

	async function onSaveQuota(next: Pick<AdminUser, 'quotaMode' | 'quotaGb'>) {
		if (!editUser) return;
		const id = editUser.id;
		const username = editUser.username;

		let mode = 'DEFAULT';
		if (next.quotaMode === 'unlimited') {
			mode = 'UNLIMITED';
		} else if (next.quotaMode === 'override_gb' || next.quotaMode === 'CUSTOM') {
			mode = 'CUSTOM';
		}

		const res = await adminAPI.updateUserQuota(id, {
			quota_mode: mode,
			quota_gb: next.quotaGb
		});

		if (res.error) {
			toast.error(`${tx('Gagal memperbarui kuota', 'Failed to update quota')}: ${res.error}`);
			return;
		}

		toast.success(
			tx(
				`Kuota “${username}” berhasil diperbarui.`,
				`Quota for “${username}” updated successfully.`
			)
		);
		editUser = null;
		await loadData();
	}

	function onRequestDelete(user: AdminUser) {
		if (String(user.role).toLowerCase() === 'super_admin') {
			toast.info(tx('Super Admin tidak dapat dihapus.', 'Super Admins cannot be deleted.'));
			return;
		}
		deleteUserTarget = user;
		deleteOpen = true;
	}

	async function onConfirmDelete(user: AdminUser, email: string) {
		const res = await adminAPI.deleteUser(user.id, { email });
		if (res.error) {
			toast.error(`${tx('Gagal menghapus pengguna', 'Failed to delete user')}: ${res.error}`);
			throw new Error(res.error);
		}

		toast.success(
			tx(
				`Pengguna “${user.username}” berhasil dihapus.`,
				`User “${user.username}” deleted successfully.`
			)
		);
		deleteUserTarget = null;
		await loadData();
	}

	function onRequestResetPassword(user: AdminUser) {
		if (currentUserId && user.id === currentUserId) {
			toast.info(tx('Anda tidak dapat mereset akun sendiri.', 'You cannot reset your own account.'));
			return;
		}
		resetUserTarget = user;
		resetOpen = true;
	}

	async function onConfirmResetPassword(user: AdminUser) {
		const res = await adminAPI.resetUserPassword(user.id);
		if (res.error || !res.data) {
			toast.error(`${tx('Gagal mereset kata sandi', 'Failed to reset password')}: ${res.error ?? ''}`);
			throw new Error(res.error ?? 'Reset response missing');
		}
		resetUserTarget = null;
		return res.data;
	}
</script>

<svelte:head>
	<title>{tx('Pengguna & Kuota', 'Users & Quotas')} — CubyCloud</title>
	<meta
		name="description"
		content={tx('Kelola pengguna dan kuota My Storage.', 'Manage My Storage users and quotas.')}
	/>
</svelte:head>

<div class="flex flex-col gap-6">
	<DrivePageHeader
		title={tx('Pengguna & Kuota', 'Users & Quotas')}
		description={tx(
			'Kelola pengguna reguler dan kuota My Storage. Super Admin selalu Unlimited.',
			'Manage regular users and My Storage quotas. Super Admins always have unlimited quota.'
		)}
	>
		{#snippet actions()}
			<Button type="button" onclick={() => (createOpen = true)}>
				<UserPlus class="size-4 stroke-2" />
				{tx('Buat Pengguna', 'Create User')}
			</Button>
		{/snippet}
	</DrivePageHeader>

	<AdminUsersTable
		{users}
		{defaultQuota}
		{currentUserId}
		{onEditQuota}
		onResetPassword={onRequestResetPassword}
		onDelete={onRequestDelete}
	/>
</div>

	<CreateUserDialog bind:open={createOpen} {defaultQuota} onCreated={() => loadData()} />
<EditQuotaDialog bind:open={editOpen} user={editUser} {defaultQuota} onSave={onSaveQuota} />
<DeleteUserDialog bind:open={deleteOpen} user={deleteUserTarget} onConfirm={onConfirmDelete} />
	<ResetPasswordDialog bind:open={resetOpen} user={resetUserTarget} onConfirm={onConfirmResetPassword} />
