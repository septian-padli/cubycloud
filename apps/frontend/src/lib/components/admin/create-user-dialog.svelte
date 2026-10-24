<script lang="ts">
	import type { AdminQuotaMode, AdminUser, DefaultQuotaPolicy } from '$lib/types/admin.js';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import * as RadioGroup from '$lib/components/ui/radio-group/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { cn } from '$lib/utils.js';
	import { locale } from '$lib/locale.svelte.js';
	type CreateQuotaChoice = 'default' | 'override_gb' | 'unlimited';

	let {
		open = $bindable(false),
		defaultQuota,
		onCreate
	}: {
		open?: boolean;
		defaultQuota: DefaultQuotaPolicy;
		onCreate: (user: Omit<AdminUser, 'id' | 'usedGb' | 'role'> & { password?: string }) => void;
	} = $props();

	let username = $state('');
	let email = $state('');
	let quotaChoice = $state<CreateQuotaChoice>('default');
	let quotaGb = $state('50');

	const defaultLabel = $derived(
		defaultQuota.mode === 'unlimited'
			? locale.language === 'id'
				? 'Tanpa batas'
				: 'Unlimited'
			: `${defaultQuota.gb} GiB`
	);

	const canSubmit = $derived(
		username.trim().length > 0 &&
			email.trim().length > 0 &&
			(quotaChoice !== 'override_gb' ||
				(String(quotaGb).trim() !== '' && !isNaN(Number(quotaGb)) && Number(quotaGb) >= 0))
	);

	$effect(() => {
		if (open) {
			username = '';
			email = '';
			quotaChoice = 'default';
			quotaGb = String(defaultQuota.gb);
		}
	});

	function submit() {
		if (!canSubmit) return;
		const mode = quotaChoice as AdminQuotaMode;
		onCreate({
			username: username.trim(),
			displayName: username.trim(),
			email: email.trim(),
			quotaMode: mode,
			quotaGb: mode === 'override_gb' ? Number(quotaGb) : undefined
		});
		open = false;
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="bg-surface text-main sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title>{locale.language === 'id' ? 'Buat Pengguna' : 'Create user'}</Dialog.Title>
			<Dialog.Description>
				{locale.language === 'id'
					? 'Tambahkan pengguna reguler. Kuota default saat ini:'
					: 'Add a regular user. Current default quota:'}
				{defaultLabel}.
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
				<Label for="create-user-username"
					>{locale.language === 'id' ? 'Nama pengguna' : 'Username'}</Label
				>
				<Input
					id="create-user-username"
					bind:value={username}
					placeholder={locale.language === 'id' ? 'mis. siti' : 'e.g. alex'}
					autocomplete="off"
				/>
			</div>

			<div class="grid gap-2">
				<Label for="create-user-email">Email</Label>
				<Input
					id="create-user-email"
					type="email"
					bind:value={email}
					placeholder={locale.language === 'id' ? 'siti@contoh.local' : 'alex@example.com'}
					autocomplete="off"
				/>
			</div>

			<div class="rounded-xl border border-subtle bg-app p-3 text-xs leading-relaxed text-muted">
				<p class="font-medium text-main">
					{locale.language === 'id' ? 'Kata sandi bawaan:' : 'Default password:'}
					<span class="font-mono font-semibold text-primary">12345678</span>
				</p>
				<p class="mt-1">
					{locale.language === 'id'
						? 'Pengguna baru dapat langsung login menggunakan kata sandi 12345678.'
						: 'New users can sign in immediately using password 12345678.'}
				</p>
			</div>

			<div class="grid gap-2">
				<p class="text-sm font-medium text-main">{locale.language === 'id' ? 'Kuota' : 'Quota'}</p>
				<RadioGroup.Root bind:value={quotaChoice} class="grid gap-2">
					<label
						class={cn(
							'flex cursor-pointer items-start gap-3 rounded-xl border border-subtle bg-app px-4 py-3',
							quotaChoice === 'default' && 'border-primary bg-primary/5'
						)}
					>
						<RadioGroup.Item value="default" id="quota-default" class="mt-0.5" />
						<div class="min-w-0">
							<Label for="quota-default" class="cursor-pointer text-sm font-medium text-main">
								{locale.language === 'id' ? 'Ikuti default' : 'Use default'} ({defaultLabel})
							</Label>
							<p class="text-xs text-muted">
								{locale.language === 'id'
									? 'Warisi kebijakan kuota sistem.'
									: 'Inherit the system quota policy.'}
							</p>
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
								{locale.language === 'id' ? 'Atur GiB' : 'Set GiB'}
							</Label>
							{#if quotaChoice === 'override_gb'}
								<div class="relative flex w-full items-center">
									<Input
										id="create-user-quota-gb"
										type="number"
										min="0"
										step="any"
										bind:value={quotaGb}
										class="w-full pe-9"
									aria-label={locale.language === 'id' ? 'Kuota dalam GiB' : 'Quota in GiB'}
									/>
									<span class="pointer-events-none absolute end-3 text-xs font-semibold text-muted">
									GiB
									</span>
								</div>
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
								{locale.language === 'id' ? 'Tanpa batas' : 'Unlimited'}
							</Label>
							<p class="text-xs text-muted">
								{locale.language === 'id'
									? 'Tanpa batas kuota My Storage.'
									: 'No quota limit for My Storage.'}
							</p>
						</div>
					</label>
				</RadioGroup.Root>
			</div>

			<Dialog.Footer>
				<Button type="button" variant="secondary" onclick={() => (open = false)}
					>{locale.language === 'id' ? 'Batal' : 'Cancel'}</Button
				>
				<Button type="submit" disabled={!canSubmit}
					>{locale.language === 'id' ? 'Buat Pengguna' : 'Create user'}</Button
				>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>
