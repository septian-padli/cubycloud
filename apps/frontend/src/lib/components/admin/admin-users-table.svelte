<script lang="ts">
	import { goto } from '$app/navigation';
	import type { AdminUser, DefaultQuotaPolicy } from '$lib/types/admin.js';
	import { effectiveQuotaLabel } from '$lib/mocks/admin-users.js';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { MoreVertical, Eye, Pencil, Trash2, KeyRound } from '@lucide/svelte';
	import { locale } from '$lib/locale.svelte.js';
	import { adminAPI } from '$lib/api/admin.js';
	import { toast } from 'svelte-sonner';
	import { Copy } from '@lucide/svelte';
	let invitationBusy = $state<string | null>(null);

	async function regenerateInvitation(user: AdminUser) {
		invitationBusy = user.id;
		const result = await adminAPI.regenerateInvitation(user.id);
		invitationBusy = null;
		if (result.error || !result.data?.invitation_url) {
			toast.error(
				result.error ??
					(locale.language === 'id'
						? 'Tautan undangan gagal dibuat.'
						: 'Could not create invitation link.')
			);
			return;
		}
		const link = new URL(result.data.invitation_url, window.location.origin).toString();
		try {
			await navigator.clipboard.writeText(link);
			toast.success(
				locale.language === 'id' ? 'Tautan undangan disalin.' : 'Invitation link copied.'
			);
		} catch {
			window.prompt(
				locale.language === 'id' ? 'Salin tautan undangan:' : 'Copy invitation link:',
				link
			);
		}
	}

	let {
		users,
		defaultQuota,
		currentUserId,
		onEditQuota,
		onResetPassword,
		onDelete
	}: {
		users: AdminUser[];
		defaultQuota: DefaultQuotaPolicy;
		currentUserId?: string;
		onEditQuota: (user: AdminUser) => void;
		onResetPassword?: (user: AdminUser) => void;
		onDelete?: (user: AdminUser) => void;
	} = $props();

	function roleLabel(role: AdminUser['role']): string {
		return String(role).toLowerCase() === 'super_admin'
			? 'Super Admin'
			: locale.language === 'id'
				? 'Pengguna'
				: 'User';
	}
</script>

<div class="overflow-x-auto rounded-xl border border-subtle bg-surface">
	<table class="w-full border-collapse text-sm">
		<thead>
			<tr class="border-b border-subtle text-left text-xs font-medium text-muted">
				<th class="px-4 py-3">{locale.language === 'id' ? 'Nama' : 'Name'}</th>
				<th class="px-4 py-3">Email</th>
				<th class="px-4 py-3">{locale.language === 'id' ? 'Peran' : 'Role'}</th>
				<th class="px-4 py-3">{locale.language === 'id' ? 'Kuota efektif' : 'Effective quota'}</th>
				<th class="px-4 py-3">{locale.language === 'id' ? 'Terpakai' : 'Used'}</th>
				<th class="px-4 py-3">{locale.language === 'id' ? 'Status' : 'Status'}</th>
				<th class="px-4 py-3 text-end">{locale.language === 'id' ? 'Aksi' : 'Actions'}</th>
			</tr>
		</thead>
		<tbody>
			{#each users as user (user.id)}
				{@const isSuperAdmin = String(user.role).toLowerCase() === 'super_admin'}
				{@const quotaLabel = effectiveQuotaLabel(user, defaultQuota)}
				{@const isSelf = currentUserId != null && user.id === currentUserId}
				{@const isPending = user.status === 'PENDING_ACTIVATION'}
				<tr class="border-b border-subtle last:border-b-0">
					<td class="px-4 py-3 font-medium text-main">{user.username}</td>
					<td class="px-4 py-3 text-muted">{user.email}</td>
					<td class="px-4 py-3">
						<span
							class="rounded-full px-2.5 py-1 text-xs font-medium {isSuperAdmin
								? 'bg-primary/15 text-primary'
								: 'bg-app text-muted'}"
						>
							{roleLabel(user.role)}
						</span>
					</td>
					<td class="px-4 py-3 text-main"
						>{locale.language === 'id' && quotaLabel === 'Unlimited'
							? 'Tanpa batas'
							: quotaLabel}</td
					>
					<td class="px-4 py-3 text-main">{user.usedGb.toFixed(1)} GiB</td>
					<td class="px-4 py-3">
						<span
							class="rounded-full px-2.5 py-1 text-xs font-medium {isPending
								? 'bg-amber-100 text-amber-950'
								: 'bg-emerald-100 text-emerald-950'}"
						>
							{isPending
								? locale.language === 'id'
									? 'Menunggu Aktivasi'
									: 'Pending Activation'
								: locale.language === 'id'
									? 'Aktif'
									: 'Active'}
						</span>
					</td>
					<td class="px-4 py-3">
						<div class="flex items-center justify-end">
							<DropdownMenu.Root>
								<DropdownMenu.Trigger>
									{#snippet child({ props })}
										<Button
											{...props}
											type="button"
											variant="ghost"
											size="icon"
											class="bg-surface text-muted hover:bg-primary hover:text-white aria-expanded:bg-primary aria-expanded:text-white"
											aria-label={locale.language === 'id'
												? `Aksi ${user.username}`
												: `Actions for ${user.username}`}
										>
											<MoreVertical class="size-4 stroke-2" />
										</Button>
									{/snippet}
								</DropdownMenu.Trigger>
								<DropdownMenu.Content align="end" class="min-w-48">
									<DropdownMenu.Item
										class="gap-2"
										onclick={() => goto(`/drive/profile/${user.id}`)}
									>
										<Eye class="size-4 stroke-2" />
										{locale.language === 'id' ? 'Lihat' : 'View'}
									</DropdownMenu.Item>
									<DropdownMenu.Item
										class="gap-2"
										disabled={isSuperAdmin}
										onclick={() => onEditQuota(user)}
									>
										<Pencil class="size-4 stroke-2" />
										{locale.language === 'id' ? 'Edit Kuota' : 'Edit quota'}
									</DropdownMenu.Item>
									{#if onResetPassword && !isSelf}
										<DropdownMenu.Item class="gap-2" onclick={() => onResetPassword(user)}>
											<KeyRound class="size-4 stroke-2" />
											{locale.language === 'id' ? 'Reset Kata Sandi' : 'Reset password'}
										</DropdownMenu.Item>
									{/if}
									{#if isPending}
										<DropdownMenu.Item
											class="gap-2"
											disabled={invitationBusy === user.id}
											onclick={() => regenerateInvitation(user)}
										>
											<Copy class="size-4 stroke-2" />
											{locale.language === 'id'
												? 'Salin Tautan Undangan / Regenerasi'
												: 'Copy / Regenerate Invitation Link'}
										</DropdownMenu.Item>
									{/if}
									{#if onDelete && !isSuperAdmin}
										<DropdownMenu.Item
											variant="destructive"
											class="gap-2"
											onclick={() => onDelete(user)}
										>
											<Trash2 class="size-4 stroke-2" />
											{locale.language === 'id' ? 'Hapus' : 'Delete'}
										</DropdownMenu.Item>
									{/if}
								</DropdownMenu.Content>
							</DropdownMenu.Root>
						</div>
					</td>
				</tr>
			{:else}
				<tr>
					<td colspan="7" class="px-4 py-8 text-center text-sm text-muted"
						>{locale.language === 'id' ? 'Belum ada pengguna.' : 'No users yet.'}</td
					>
				</tr>
			{/each}
		</tbody>
	</table>
</div>
