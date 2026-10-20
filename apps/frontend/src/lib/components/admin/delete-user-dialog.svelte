<script lang="ts">
	import type { AdminUser } from '$lib/types/admin.js';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { AlertTriangle, LoaderCircle } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';

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
		toast.warning('Pencegahan keamanan: Menempel (paste) tidak diizinkan. Silakan ketik email secara manual.');
	}

	function handleDrop(e: DragEvent) {
		e.preventDefault();
		toast.warning('Pencegahan keamanan: Menjatuhkan teks tidak diizinkan.');
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
				<Dialog.Title class="text-destructive">Hapus Pengguna</Dialog.Title>
			</div>
			<Dialog.Description class="pt-2 text-sm text-muted">
				Tindakan ini tidak dapat dibatalkan. Mohon baca peringatan di bawah ini dengan saksama.
			</Dialog.Description>
		</Dialog.Header>

		{#if user}
			<div class="flex flex-col gap-4 py-2">
				<div class="rounded-lg border border-destructive bg-white p-3.5 text-xs leading-relaxed text-destructive dark:bg-surface">
					<p class="font-semibold text-destructive">Konsekuensi Penghapusan Akun:</p>
					<ul class="mt-1.5 list-disc space-y-1 pl-4 text-destructive">
						<li>
							Seluruh berkas dan folder di <span class="font-semibold">My Storage</span> pribadi (termasuk yang ada di <span class="font-semibold">Trash</span>) akan <span class="font-semibold underline">dihapus permanen</span>.
						</li>
						<li>
							Berkas di ruang <span class="font-semibold">Organization</span> akan <span class="font-semibold">tetap tersimpan</span> dan status pembuat diubah menjadi <span class="font-semibold font-mono">“deleted user”</span>.
						</li>
						<li>
							Akun <span class="font-semibold">{user.username}</span> akan dinonaktifkan (tombstone) dan seluruh sesi aktifnya langsung dibatalkan.
						</li>
					</ul>
				</div>

				<div class="flex flex-col gap-2">
					<label for="confirm-email-input" class="block text-xs leading-normal text-muted">
						Ketik ulang email pengguna <span class="font-bold text-main">{user.email}</span> persis sama untuk konfirmasi:
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
						class="bg-surface border-subtle focus:border-destructive focus:ring-destructive font-mono"
					/>
					{#if confirmEmail.length > 0 && !isEmailExactMatch}
						<p class="text-xs text-destructive">Email yang Anda ketik belum cocok persis.</p>
					{/if}
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
				Batal
			</Button>
			<Button
				type="button"
				variant="destructive"
				disabled={!canSubmit}
				onclick={handleSubmit}
			>
				{#if loading}
					<LoaderCircle class="size-4 animate-spin stroke-2" />
					Menghapus...
				{:else}
					Hapus Pengguna
				{/if}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
