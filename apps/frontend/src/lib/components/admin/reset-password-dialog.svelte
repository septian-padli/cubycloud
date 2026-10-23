<script lang="ts">
	import type { AdminUser } from '$lib/types/admin.js';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { KeyRound, LoaderCircle } from '@lucide/svelte';
	import { locale } from '$lib/locale.svelte.js';

	let {
		open = $bindable(false),
		user,
		onConfirm
	}: {
		open?: boolean;
		user: AdminUser | null;
		onConfirm: (user: AdminUser) => Promise<void> | void;
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
			await onConfirm(user);
			open = false;
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
			<div class="flex flex-col gap-3 py-2">
				<div class="rounded-xl border border-subtle bg-app p-3.5 text-xs leading-relaxed text-muted">
					<p class="font-medium text-main">
						{locale.language === 'id' ? 'Kata sandi baru:' : 'New password:'}
						<span class="font-mono font-semibold text-primary">12345678</span>
					</p>
					<p class="mt-1">
						{locale.language === 'id'
							? 'Kata sandi akun akan segera diubah menjadi 12345678 dan sesi aktif pengguna ini akan dihentikan.'
							: 'The account password will immediately be changed to 12345678 and any active sessions for this user will be invalidated.'}
					</p>
				</div>
			</div>
		{/if}

		<Dialog.Footer class="gap-2 sm:gap-0">
			<Button
				type="button"
				variant="outline"
				disabled={loading}
				onclick={() => (open = false)}
			>
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
