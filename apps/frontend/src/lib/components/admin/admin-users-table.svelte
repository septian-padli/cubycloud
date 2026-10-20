<script lang="ts">
	import type { AdminUser, DefaultQuotaPolicy } from '$lib/types/admin.js';
	import { effectiveQuotaLabel } from '$lib/mocks/admin-users.js';
	import * as DropdownMenu from '$lib/components/ui/dropdown-menu/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { MoreVertical, Eye, Pencil, Trash2 } from '@lucide/svelte';
	import { goto } from '$app/navigation';

	let {
		users,
		defaultQuota,
		onEditQuota,
		onDelete
	}: {
		users: AdminUser[];
		defaultQuota: DefaultQuotaPolicy;
		onEditQuota: (user: AdminUser) => void;
		onDelete?: (user: AdminUser) => void;
	} = $props();

	function roleLabel(role: AdminUser['role']): string {
		return String(role).toLowerCase() === 'super_admin' ? 'Super Admin' : 'User';
	}
</script>

<div class="overflow-x-auto rounded-xl border border-subtle bg-surface">
	<table class="w-full border-collapse text-sm">
		<thead>
			<tr class="border-b border-subtle text-left text-xs font-medium text-muted">
				<th class="px-4 py-3">Nama</th>
				<th class="px-4 py-3">Email</th>
				<th class="px-4 py-3">Peran</th>
				<th class="px-4 py-3">Kuota efektif</th>
				<th class="px-4 py-3">Terpakai</th>
				<th class="px-4 py-3 text-end">Aksi</th>
			</tr>
		</thead>
		<tbody>
			{#each users as user (user.id)}
				{@const isSuperAdmin = String(user.role).toLowerCase() === 'super_admin'}
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
					<td class="px-4 py-3 text-main">{effectiveQuotaLabel(user, defaultQuota)}</td>
					<td class="px-4 py-3 text-main">{user.usedGb.toFixed(1)} GB</td>
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
											aria-label="Aksi {user.username}"
										>
											<MoreVertical class="size-4 stroke-2" />
										</Button>
									{/snippet}
								</DropdownMenu.Trigger>
								<DropdownMenu.Content align="end" class="min-w-40">
									<DropdownMenu.Item
										class="gap-2"
										onclick={() => goto(`/drive/profile/${user.id}`)}
									>
										<Eye class="size-4 stroke-2" />
										Lihat
									</DropdownMenu.Item>
									<DropdownMenu.Item
										class="gap-2"
										disabled={isSuperAdmin}
										onclick={() => onEditQuota(user)}
									>
										<Pencil class="size-4 stroke-2" />
										Edit Kuota
									</DropdownMenu.Item>
									{#if onDelete && !isSuperAdmin}
										<DropdownMenu.Item
											variant="destructive"
											class="gap-2"
											onclick={() => onDelete(user)}
										>
											<Trash2 class="size-4 stroke-2" />
											Hapus
										</DropdownMenu.Item>
									{/if}
								</DropdownMenu.Content>
							</DropdownMenu.Root>
						</div>
					</td>
				</tr>
			{:else}
				<tr>
					<td colspan="6" class="px-4 py-8 text-center text-sm text-muted">
						Belum ada pengguna.
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
</div>
