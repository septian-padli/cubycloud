<script lang="ts">
	import type { AdminQuotaMode, AdminUser, DefaultQuotaPolicy } from '$lib/types/admin.js';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import * as RadioGroup from '$lib/components/ui/radio-group/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import PasswordInput from '$lib/components/password-input.svelte';
	import { cn } from '$lib/utils.js';

	type CreateQuotaChoice = 'default' | 'override_gb' | 'unlimited';

	let {
		open = $bindable(false),
		defaultQuota,
		onCreate
	}: {
		open?: boolean;
		defaultQuota: DefaultQuotaPolicy;
		onCreate: (user: Omit<AdminUser, 'id' | 'usedGb' | 'role'> & { password: string }) => void;
	} = $props();

	let username = $state('');
	let email = $state('');
	let password = $state('');
	let quotaChoice = $state<CreateQuotaChoice>('default');
	let quotaGb = $state('50');

	const defaultLabel = $derived(
		defaultQuota.mode === 'unlimited' ? 'Unlimited' : `${defaultQuota.gb} GB`
	);

	const canSubmit = $derived(
		username.trim().length > 0 &&
			email.trim().length > 0 &&
			password.length > 0 &&
			(quotaChoice !== 'override_gb' || Number(quotaGb) > 0)
	);

	$effect(() => {
		if (open) {
			username = '';
			email = '';
			password = '';
			quotaChoice = 'default';
			quotaGb = String(defaultQuota.gb);
		}
	});

	function submit() {
		if (!canSubmit) return;
		const mode = quotaChoice as AdminQuotaMode;
		onCreate({
			username: username.trim(),
			email: email.trim(),
			password,
			quotaMode: mode,
			quotaGb: mode === 'override_gb' ? Number(quotaGb) : undefined
		});
		open = false;
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="bg-surface text-main sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title>Buat Pengguna</Dialog.Title>
			<Dialog.Description>
				Tambah pengguna reguler. Kuota default saat ini: {defaultLabel}.
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
				<Label for="create-user-username">Nama pengguna</Label>
				<Input
					id="create-user-username"
					bind:value={username}
					placeholder="mis. siti"
					autocomplete="off"
				/>
			</div>

			<div class="grid gap-2">
				<Label for="create-user-email">Email</Label>
				<Input
					id="create-user-email"
					type="email"
					bind:value={email}
					placeholder="siti@contoh.local"
					autocomplete="off"
				/>
			</div>

			<div class="grid gap-2">
				<Label for="create-user-password">Kata sandi</Label>
				<PasswordInput
					id="create-user-password"
					bind:value={password}
					placeholder="••••••••"
					autocomplete="new-password"
				/>
			</div>

			<div class="grid gap-2">
				<p class="text-sm font-medium text-main">Kuota</p>
				<RadioGroup.Root
					bind:value={quotaChoice}
					class="grid gap-2"
				>
					<label
						class={cn(
							'flex cursor-pointer items-start gap-3 rounded-xl border border-subtle bg-app px-4 py-3',
							quotaChoice === 'default' && 'border-primary bg-primary/5'
						)}
					>
						<RadioGroup.Item value="default" id="quota-default" class="mt-0.5" />
						<div class="min-w-0">
							<Label for="quota-default" class="cursor-pointer text-sm font-medium text-main">
								Ikuti default ({defaultLabel})
							</Label>
							<p class="text-xs text-muted">Warisi kebijakan kuota sistem.</p>
						</div>
					</label>

					<label
						class={cn(
							'flex cursor-pointer items-start gap-3 rounded-xl border border-subtle bg-app px-4 py-3',
							quotaChoice === 'override_gb' && 'border-primary bg-primary/5'
						)}
					>
						<RadioGroup.Item value="override_gb" id="quota-gb" class="mt-0.5" />
						<div class="min-w-0 flex-1 space-y-2">
							<Label for="quota-gb" class="cursor-pointer text-sm font-medium text-main">
								Atur GB
							</Label>
							{#if quotaChoice === 'override_gb'}
								<Input
									id="create-user-quota-gb"
									type="number"
									min="1"
									step="1"
									bind:value={quotaGb}
									class="max-w-32"
									aria-label="Kuota dalam GB"
								/>
							{/if}
						</div>
					</label>

					<label
						class={cn(
							'flex cursor-pointer items-start gap-3 rounded-xl border border-subtle bg-app px-4 py-3',
							quotaChoice === 'unlimited' && 'border-primary bg-primary/5'
						)}
					>
						<RadioGroup.Item value="unlimited" id="quota-unlimited" class="mt-0.5" />
						<div class="min-w-0">
							<Label for="quota-unlimited" class="cursor-pointer text-sm font-medium text-main">
								Unlimited
							</Label>
							<p class="text-xs text-muted">Tanpa batas kuota My Storage.</p>
						</div>
					</label>
				</RadioGroup.Root>
			</div>

			<Dialog.Footer>
				<Button type="button" variant="secondary" onclick={() => (open = false)}>Batal</Button>
				<Button type="submit" disabled={!canSubmit}>Buat Pengguna</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>
