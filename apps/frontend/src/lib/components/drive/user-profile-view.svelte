<script lang="ts">
	import type { AdminUser } from '$lib/types/admin.js';
	import type { QuotaState } from '$lib/types/drive.js';
	import DrivePageHeader from '$lib/components/drive/drive-page-header.svelte';
	import StorageQuotaDetail from '$lib/components/drive/storage-quota-detail.svelte';
	import EditProfileDialog from '$lib/components/drive/edit-profile-dialog.svelte';
	import * as Card from '$lib/components/ui/card/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Pencil } from '@lucide/svelte';

	let {
		user,
		quota,
		editable = false,
		onSaveProfile
	}: {
		user: AdminUser;
		quota: QuotaState;
		editable?: boolean;
		onSaveProfile?: (next: { displayName: string; email: string; currentPassword?: string }) => void | Promise<boolean | void>;
	} = $props();

	let editOpen = $state(false);

	const isSuperAdmin = $derived(user.role === 'super_admin');
</script>

<div class="flex flex-col gap-8">
	<DrivePageHeader
		title={user.displayName}
		description="Profil pengguna dan ringkasan pemakaian My Storage."
		eyebrow="@{user.username}"
	>
		{#snippet actions()}
			<Button
				type="button"
				variant="secondary"
				disabled={!editable}
				onclick={() => {
					if (editable) editOpen = true;
				}}
				class={!editable ? 'opacity-50 cursor-not-allowed' : ''}
			>
				<Pencil class="size-4 stroke-2" />
				Edit Profil
			</Button>
		{/snippet}
	</DrivePageHeader>

	<section class="space-y-3">
		<h2 class="text-sm font-semibold text-main">Informasi akun</h2>
		<Card.Root class="rounded-xl border-subtle bg-surface shadow-none">
			<Card.Content class="grid gap-4 sm:grid-cols-2">
				<div class="space-y-1">
					<p class="text-xs font-medium text-muted">Nama pengguna</p>
					<p class="text-sm text-main">@{user.username}</p>
				</div>
				<div class="space-y-1">
					<p class="text-xs font-medium text-muted">Nama tampilan</p>
					<p class="text-sm text-main">{user.displayName}</p>
				</div>
				<div class="space-y-1">
					<p class="text-xs font-medium text-muted">Email</p>
					<p class="text-sm text-main">{user.email}</p>
				</div>
				<div class="space-y-1">
					<p class="text-xs font-medium text-muted">Peran</p>
					<span
						class="inline-flex rounded-full px-2.5 py-1 text-xs font-medium {isSuperAdmin
							? 'bg-primary/15 text-primary'
							: 'bg-app text-muted'}"
					>
						{user.role === 'super_admin' ? 'Super Admin' : 'User'}
					</span>
				</div>
				<div class="space-y-1 sm:col-span-2">
					<p class="text-xs font-medium text-muted">Kuota efektif</p>
					<p class="text-sm text-main">
						{#if quota.limitGb == null}Tidak terbatas{:else}{quota.limitGb.toFixed(1)} GB{/if}
						<span class="text-muted"> · terpakai {user.usedGb.toFixed(1)} GB</span>
					</p>
				</div>
			</Card.Content>
		</Card.Root>
	</section>

	<StorageQuotaDetail {quota} />

</div>

{#if editable && onSaveProfile}
	<EditProfileDialog
		bind:open={editOpen}
		displayName={user.displayName}
		email={user.email}
		username={user.username}
		onSave={onSaveProfile}
	/>
{/if}
