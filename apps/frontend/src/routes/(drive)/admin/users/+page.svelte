<script lang="ts">
	import { toast } from 'svelte-sonner';
	import type { AdminUser } from '$lib/types/admin.js';
	import {
		MOCK_DEFAULT_QUOTA,
		cloneMockAdminUsers
	} from '$lib/mocks/admin-users.js';
	import DrivePageHeader from '$lib/components/drive/drive-page-header.svelte';
	import AdminUsersTable from '$lib/components/admin/admin-users-table.svelte';
	import CreateUserDialog from '$lib/components/admin/create-user-dialog.svelte';
	import EditQuotaDialog from '$lib/components/admin/edit-quota-dialog.svelte';
	import { Button } from '$lib/components/ui/button/index.js';
	import { UserPlus } from '@lucide/svelte';

	// ponytail: add +layout.server role check when API exists
	let users = $state<AdminUser[]>(cloneMockAdminUsers());
	const defaultQuota = MOCK_DEFAULT_QUOTA;

	let createOpen = $state(false);
	let editOpen = $state(false);
	let editUser = $state<AdminUser | null>(null);

	function onCreate(
		payload: Omit<AdminUser, 'id' | 'usedGb' | 'role'> & { password: string }
	) {
		users = [
			...users,
			{
				id: `user-${crypto.randomUUID().slice(0, 8)}`,
				username: payload.username,
				displayName: payload.displayName,
				email: payload.email,
				role: 'user',
				quotaMode: payload.quotaMode,
				quotaGb: payload.quotaGb,
				usedGb: 0
			}
		];
		toast.success(`Pengguna “${payload.username}” dibuat (mock)`);
	}

	function onEditQuota(user: AdminUser) {
		if (user.role === 'super_admin') {
			toast.info('Kuota Super Admin selalu Unlimited.');
			return;
		}
		editUser = user;
		editOpen = true;
	}

	function onSaveQuota(next: Pick<AdminUser, 'quotaMode' | 'quotaGb'>) {
		if (!editUser) return;
		const id = editUser.id;
		users = users.map((u) =>
			u.id === id
				? {
						...u,
						quotaMode: next.quotaMode,
						quotaGb: next.quotaMode === 'override_gb' ? next.quotaGb : undefined
					}
				: u
		);
		toast.success(`Kuota “${editUser.username}” diperbarui (mock)`);
		editUser = null;
	}

	function onDelete(user: AdminUser) {
		if (user.role === 'super_admin') {
			toast.info('Super Admin tidak dapat dihapus.');
			return;
		}
		users = users.filter((u) => u.id !== user.id);
		toast.success(`Pengguna “${user.username}” dihapus (mock)`);
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
		{onDelete}
	/>
</div>

<CreateUserDialog bind:open={createOpen} {defaultQuota} {onCreate} />
<EditQuotaDialog
	bind:open={editOpen}
	user={editUser}
	{defaultQuota}
	onSave={onSaveQuota}
/>
