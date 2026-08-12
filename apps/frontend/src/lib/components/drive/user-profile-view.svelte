<script lang="ts">
	import type { AdminUser } from '$lib/types/admin.js';
	import type { CategoryUsage, QuotaState } from '$lib/types/drive.js';
	import { effectiveQuotaLabel, roleLabel } from '$lib/mocks/user-profiles.js';
	import { MOCK_DEFAULT_QUOTA } from '$lib/mocks/admin-users.js';
	import DrivePageHeader from '$lib/components/drive/drive-page-header.svelte';
	import StorageQuotaDetail from '$lib/components/drive/storage-quota-detail.svelte';
	import ProfileStackedCategoryBar from '$lib/components/drive/profile-stacked-category-bar.svelte';
	import EditProfileDialog from '$lib/components/drive/edit-profile-dialog.svelte';
	import * as Card from '$lib/components/ui/card/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Pencil } from '@lucide/svelte';

	let {
		user,
		quota,
		categories,
		editable = false,
		onSaveProfile
	}: {
		user: AdminUser;
		quota: QuotaState;
		categories: CategoryUsage[];
		editable?: boolean;
		onSaveProfile?: (next: { displayName: string; email: string }) => void;
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
			{#if editable}
				<Button type="button" variant="secondary" onclick={() => (editOpen = true)}>
					<Pencil class="size-4 stroke-2" />
					Edit Profil
				</Button>
			{/if}
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
						{roleLabel(user.role)}
					</span>
				</div>
				<div class="space-y-1 sm:col-span-2">
					<p class="text-xs font-medium text-muted">Kuota efektif</p>
					<p class="text-sm text-main">
						{effectiveQuotaLabel(user, MOCK_DEFAULT_QUOTA)}
						{#if quota.limitGb == null}
							<span class="text-muted"> · terpakai {user.usedGb.toFixed(1)} GB</span>
						{/if}
					</p>
				</div>
			</Card.Content>
		</Card.Root>
	</section>

	<StorageQuotaDetail {quota} />

	<section class="space-y-3">
		<h2 class="text-sm font-semibold text-main">Pemakaian per kategori</h2>
		<Card.Root class="rounded-xl border-subtle bg-surface shadow-none">
			<Card.Content>
				<ProfileStackedCategoryBar {categories} />
			</Card.Content>
		</Card.Root>
	</section>
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
