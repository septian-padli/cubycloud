<script lang="ts">
	import type { AdminUser } from '$lib/types/admin.js';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { AlertTriangle, LoaderCircle } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import { locale } from '$lib/locale.svelte.js';
	let {
		open = $bindable(false),
		user,
		onConfirm
	}: {
		open?: boolean;
		user: AdminUser | null;
		onConfirm: (user: AdminUser, email: string) => Promise<void> | void;
	} = $props();

	let confirmEmail = $state('');
	let loading = $state(false);

	const isEmailExactMatch = $derived(user != null && confirmEmail === user.email);
	const canSubmit = $derived(isEmailExactMatch && !loading);

	$effect(() => {
		if (open) {
			confirmEmail = '';
			loading = false;
		}
	});

	function handlePaste(e: ClipboardEvent) {
		e.preventDefault();
		toast.warning(
			locale.language === 'id'
				? 'Pencegahan keamanan: Menempel (paste) tidak diizinkan. Silakan ketik email secara manual.'
				: 'Security notice: Pasting is disabled. Please type the email manually.'
		);
	}

	function handleDrop(e: DragEvent) {
		e.preventDefault();
		toast.warning(
			locale.language === 'id'
				? 'Pencegahan keamanan: Menjatuhkan teks tidak diizinkan.'
				: 'Security notice: Dropping text is disabled.'
		);
	}

	async function handleSubmit() {
		if (!canSubmit || !user) return;
		loading = true;
		try {
			await onConfirm(user, confirmEmail);
			open = false;
		} catch (err) {
			// handled by caller or toast
		} finally {
			loading = false;
		}
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="bg-surface text-main sm:max-w-lg">
		<Dialog.Header>
			<div class="flex items-center gap-2 text-destructive">
				<AlertTriangle class="size-5 stroke-2" />
				<Dialog.Title class="text-destructive"
					>{locale.language === 'id' ? 'Hapus Pengguna' : 'Delete user'}</Dialog.Title
				>
			</div>
			<Dialog.Description class="pt-2 text-sm text-muted">
				{locale.language === 'id'
					? 'Tindakan ini tidak dapat dibatalkan. Mohon baca peringatan di bawah ini dengan saksama.'
					: 'This action cannot be undone. Please read the warning below carefully.'}
			</Dialog.Description>
		</Dialog.Header>

		{#if user}
			<div class="flex flex-col gap-4 py-2">
				<div
					class="rounded-lg border border-destructive bg-white p-3.5 text-xs leading-relaxed text-destructive dark:bg-surface"
				>
					<p class="font-semibold text-destructive">
						{locale.language === 'id'
							? 'Konsekuensi Penghapusan Akun:'
							: 'Account deletion consequences:'}
					</p>
					<ul class="mt-1.5 list-disc space-y-1 pl-4 text-destructive">
						<li>
							{locale.language === 'id'
								? 'Seluruh berkas dan folder di'
								: 'All files and folders in'} <span class="font-semibold">My Storage</span>
							{locale.language === 'id' ? 'pribadi (termasuk yang ada di' : ' (including those in'}
							<span class="font-semibold">Trash</span>) {locale.language === 'id'
								? 'akan'
								: 'will be'}
							<span class="font-semibold underline"
								>{locale.language === 'id' ? 'dihapus permanen' : 'permanently deleted'}</span
							>.
						</li>
						<li>
							{locale.language === 'id' ? 'Berkas di ruang' : 'Files in the'}
							<span class="font-semibold">Organization</span>
							{locale.language === 'id' ? 'akan' : 'will'}
							<span class="font-semibold"
								>{locale.language === 'id' ? 'tetap tersimpan' : 'remain stored'}</span
							>
							{locale.language === 'id'
								? 'dan status pembuat diubah menjadi'
								: 'and the creator will be marked as'}
							<span class="font-mono font-semibold">“deleted user”</span>.
						</li>
						<li>
							{locale.language === 'id' ? 'Akun' : 'The account'}
							<span class="font-semibold">{user.username}</span>
							{locale.language === 'id'
								? 'akan dinonaktifkan (tombstone) dan seluruh sesi aktifnya langsung dibatalkan.'
								: 'will be disabled (tombstoned), and all active sessions will be immediately revoked.'}
						</li>
					</ul>
				</div>

				<div class="flex flex-col gap-2">
					<label for="confirm-email-input" class="block text-xs leading-normal text-muted">
						{locale.language === 'id' ? 'Ketik ulang email pengguna' : 'Re-enter the user email'}
						<span class="font-bold text-main">{user.email}</span>
						{locale.language === 'id' ? 'persis sama untuk konfirmasi:' : 'exactly to confirm:'}
					</label>
					<Input
						id="confirm-email-input"
						type="text"
						bind:value={confirmEmail}
						placeholder={user.email}
						autocomplete="off"
						autocorrect="off"
						spellcheck={false}
						onpaste={handlePaste}
						ondrop={handleDrop}
						class="border-subtle bg-surface font-mono focus:border-destructive focus:ring-destructive"
					/>
					{#if confirmEmail.length > 0 && !isEmailExactMatch}
						<p class="text-xs text-destructive">
							{locale.language === 'id'
								? 'Email yang Anda ketik belum cocok persis.'
								: 'The email you entered does not match exactly.'}
						</p>
					{/if}
				</div>
			</div>
		{/if}

		<Dialog.Footer class="gap-2 sm:gap-0">
			<Button type="button" variant="outline" disabled={loading} onclick={() => (open = false)}>
				{locale.language === 'id' ? 'Batal' : 'Cancel'}
			</Button>
			<Button type="button" variant="destructive" disabled={!canSubmit} onclick={handleSubmit}>
				{#if loading}
					<LoaderCircle class="size-4 animate-spin stroke-2" />
					{locale.language === 'id' ? 'Menghapus...' : 'Deleting...'}
				{:else}
					{locale.language === 'id' ? 'Hapus Pengguna' : 'Delete user'}
				{/if}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
