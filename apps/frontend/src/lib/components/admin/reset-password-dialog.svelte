<script lang="ts">
	import type { AdminUser } from '$lib/types/admin.js';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { KeyRound, LoaderCircle } from '@lucide/svelte';
	import { locale } from '$lib/locale.svelte.js';

	import { toast } from 'svelte-sonner';
	import { Input } from '$lib/components/ui/input/index.js';
	import type { OneTimeLinkResponse } from '$lib/api/admin.js';
	let resetUrl = $state('');
	async function copyResetLink() {
		try {
			await navigator.clipboard.writeText(resetUrl);
			toast.success(locale.language === 'id' ? 'Tautan disalin.' : 'Link copied.');
		} catch {
			window.prompt(locale.language === 'id' ? 'Salin tautan:' : 'Copy link:', resetUrl);
		}
	}
	let {
		open = $bindable(false),
		user,
		onConfirm
	}: {
		open?: boolean;
		user: AdminUser | null;
		onConfirm: (
			user: AdminUser
		) => Promise<OneTimeLinkResponse | void> | OneTimeLinkResponse | void;
	} = $props();

	let loading = $state(false);

	$effect(() => {
		if (open) {
			loading = false;
		}
	});

	async function handleConfirm() {
		if (!user || loading) return;
		loading = true;
		try {
			const result = await onConfirm(user);
			if (result?.reset_url) {
				resetUrl = new URL(result.reset_url, window.location.origin).toString();
				open = false;
			} else open = false;
		} finally {
			loading = false;
		}
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="bg-surface text-main sm:max-w-md">
		<Dialog.Header>
			<div class="flex items-center gap-2 text-primary">
				<KeyRound class="size-5 stroke-2" />
				<Dialog.Title>
					{locale.language === 'id' ? 'Reset Kata Sandi' : 'Reset Password'}
				</Dialog.Title>
			</div>
			<Dialog.Description class="pt-2 text-sm text-muted">
				{locale.language === 'id'
					? `Apakah Anda yakin ingin mengatur ulang kata sandi untuk akun “${user?.username ?? ''}”?`
					: `Are you sure you want to reset the password for account “${user?.username ?? ''}”?`}
			</Dialog.Description>
		</Dialog.Header>

		{#if user}
			<p
				class="rounded-lg border border-amber-700 bg-amber-50 p-3 text-sm font-medium text-amber-950"
			>
				{locale.language === 'id'
					? 'Reset akan mencabut semua sesi aktif pengguna. Kata sandi lama tidak dapat digunakan sampai kata sandi baru diatur.'
					: 'Reset will revoke all of this user’s active sessions. Their old password will not work until a new password is set.'}
			</p>
		{/if}

		<Dialog.Footer class="gap-2 sm:gap-0">
			<Button type="button" variant="outline" disabled={loading} onclick={() => (open = false)}>
				{locale.language === 'id' ? 'Batal' : 'Cancel'}
			</Button>
			<Button
				type="button"
				variant="default"
				disabled={loading || !user}
				onclick={handleConfirm}
				class="gap-2"
			>
				{#if loading}
					<LoaderCircle class="size-4 animate-spin" />
				{/if}
				{locale.language === 'id' ? 'Reset Kata Sandi' : 'Reset Password'}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

{#if resetUrl}
	<Dialog.Root
		open={!!resetUrl}
		onOpenChange={(value) => {
			if (!value) resetUrl = '';
		}}
	>
		<Dialog.Content class="bg-surface text-main sm:max-w-md">
			<Dialog.Header>
				<Dialog.Title
					>{locale.language === 'id' ? 'Tautan reset siap' : 'Reset link ready'}</Dialog.Title
				>
				<Dialog.Description
					>{locale.language === 'id'
						? 'Tautan hanya dapat digunakan sekali dan berlaku 30 menit.'
						: 'This one-time link expires in 30 minutes.'}</Dialog.Description
				>
			</Dialog.Header>
			<div class="flex flex-col gap-3">
				<Input
					readonly
					value={resetUrl}
					aria-label={locale.language === 'id' ? 'Tautan reset kata sandi' : 'Password reset link'}
				/>
				<Button
					type="button"
					class="min-h-11 focus-visible:outline focus-visible:outline-2"
					onclick={copyResetLink}>{locale.language === 'id' ? 'Salin Tautan' : 'Copy Link'}</Button
				>
			</div>
		</Dialog.Content>
	</Dialog.Root>
{/if}
