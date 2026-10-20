<script lang="ts">
	import { onMount } from 'svelte';
	import { toast } from 'svelte-sonner';
	import type { AdminUser, DefaultQuotaPolicy } from '$lib/types/admin.js';
	import { MOCK_DEFAULT_QUOTA } from '$lib/mocks/admin-users.js';
	import { adminAPI } from '$lib/api/admin.js';
	import DrivePageHeader from '$lib/components/drive/drive-page-header.svelte';
	import AdminUsersTable from '$lib/components/admin/admin-users-table.svelte';
	import CreateUserDialog from '$lib/components/admin/create-user-dialog.svelte';
	import EditQuotaDialog from '$lib/components/admin/edit-quota-dialog.svelte';
	import DeleteUserDialog from '$lib/components/admin/delete-user-dialog.svelte';
	import { Button } from '$lib/components/ui/button/index.js';
	import { UserPlus } from '@lucide/svelte';

	let users = $state<AdminUser[]>([]);
	let defaultQuota = $state<DefaultQuotaPolicy>(MOCK_DEFAULT_QUOTA);
	let loading = $state(true);

	let createOpen = $state(false);
	let editOpen = $state(false);
	let editUser = $state<AdminUser | null>(null);
	let deleteOpen = $state(false);
	let deleteUserTarget = $state<AdminUser | null>(null);

	async function loadData() {
		loading = true;
		try {
			const [settingsRes, usersRes] = await Promise.all([
				adminAPI.getSettings(),
				adminAPI.getUsers()
			]);

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
						createdAt: u.created_at
					};
				});
			} else if (usersRes.error) {
				toast.error(`Gagal memuat pengguna: ${usersRes.error}`);
			}
		} catch (err) {
			toast.error('Gagal menghubungi server.');
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		void loadData();
	});

	async function onCreate(
		payload: Omit<AdminUser, 'id' | 'usedGb' | 'role'> & { password: string }
	) {
		let mode = 'DEFAULT';
		if (payload.quotaMode === 'unlimited') {
			mode = 'UNLIMITED';
		} else if (payload.quotaMode === 'override_gb' || payload.quotaMode === 'CUSTOM') {
			mode = 'CUSTOM';
		}

		const res = await adminAPI.createUser({
			username: payload.username,
			email: payload.email,
			password: payload.password,
			quota_mode: mode,
			quota_gb: payload.quotaGb
		});

		if (res.error) {
			toast.error(`Gagal membuat pengguna: ${res.error}`);
			return;
		}

		toast.success(`Pengguna “${payload.username}” berhasil dibuat.`);
		await loadData();
	}

	function onEditQuota(user: AdminUser) {
		if (String(user.role).toLowerCase() === 'super_admin') {
			toast.info('Kuota Super Admin selalu Unlimited.');
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
			toast.error(`Gagal memperbarui kuota: ${res.error}`);
			return;
		}

		toast.success(`Kuota “${username}” berhasil diperbarui.`);
		editUser = null;
		await loadData();
	}

	function onRequestDelete(user: AdminUser) {
		if (String(user.role).toLowerCase() === 'super_admin') {
			toast.info('Super Admin tidak dapat dihapus.');
			return;
		}
		deleteUserTarget = user;
		deleteOpen = true;
	}

	async function onConfirmDelete(user: AdminUser, email: string) {
		const res = await adminAPI.deleteUser(user.id, { email });
		if (res.error) {
			toast.error(`Gagal menghapus pengguna: ${res.error}`);
			throw new Error(res.error);
		}

		toast.success(`Pengguna “${user.username}” berhasil dihapus.`);
		deleteUserTarget = null;
		await loadData();
	}
</script>

<svelte:head>
	<title>Users & Quotas — CubyCloud</title>
	<meta name="description" content="Kelola pengguna dan kuota My Storage." />
</svelte:head>

<div class="flex flex-col gap-6">
	<DrivePageHeader
		title="Users & Quotas"
		description="Kelola pengguna reguler dan kuota My Storage. Super Admin selalu Unlimited."
	>
		{#snippet actions()}
			<Button type="button" onclick={() => (createOpen = true)}>
				<UserPlus class="size-4 stroke-2" />
				Buat Pengguna
			</Button>
		{/snippet}
	</DrivePageHeader>

	<AdminUsersTable
		{users}
		{defaultQuota}
		{onEditQuota}
		onDelete={onRequestDelete}
	/>
</div>

<CreateUserDialog bind:open={createOpen} {defaultQuota} {onCreate} />
<EditQuotaDialog
	bind:open={editOpen}
	user={editUser}
	{defaultQuota}
	onSave={onSaveQuota}
/>
<DeleteUserDialog
	bind:open={deleteOpen}
	user={deleteUserTarget}
	onConfirm={onConfirmDelete}
/>
