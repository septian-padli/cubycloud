<script lang="ts">
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';

	let {
		open = $bindable(false),
		displayName = '',
		email = '',
		username = '',
		onSave
	}: {
		open?: boolean;
		displayName?: string;
		email?: string;
		username?: string;
		onSave: (next: { displayName: string; email: string }) => void;
	} = $props();

	let nameDraft = $state('');
	let emailDraft = $state('');

	const canSubmit = $derived(nameDraft.trim().length > 0 && emailDraft.trim().length > 0);

	$effect(() => {
		if (open) {
			nameDraft = displayName;
			emailDraft = email;
		}
	});

	function submit() {
		if (!canSubmit) return;
		onSave({ displayName: nameDraft.trim(), email: emailDraft.trim() });
		open = false;
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="bg-surface text-main sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title>Edit Profil</Dialog.Title>
			<Dialog.Description>
				Perbarui nama tampilan dan email. Nama pengguna (@{username}) tidak dapat diubah.
			</Dialog.Description>
		</Dialog.Header>

		<form
			class="grid gap-4"
			onsubmit={(e) => {
				e.preventDefault();
				submit();
			}}
		>
			<div class="grid gap-2">
				<Label for="profile-display-name">Nama tampilan</Label>
				<Input id="profile-display-name" bind:value={nameDraft} autocomplete="name" />
			</div>

			<div class="grid gap-2">
				<Label for="profile-email">Email</Label>
				<Input id="profile-email" type="email" bind:value={emailDraft} autocomplete="email" />
			</div>

			<div class="grid gap-2">
				<Label for="profile-username">Nama pengguna</Label>
				<Input id="profile-username" value={username} disabled class="text-muted" />
			</div>

			<Dialog.Footer>
				<Button type="button" variant="secondary" onclick={() => (open = false)}>Batal</Button>
				<Button type="submit" disabled={!canSubmit}>Simpan</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>
