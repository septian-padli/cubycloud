<script lang="ts">
	import type { AdminQuotaMode, DefaultQuotaPolicy } from '$lib/types/admin.js';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import * as RadioGroup from '$lib/components/ui/radio-group/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { cn } from '$lib/utils.js';
	import { toast } from 'svelte-sonner';
	import { adminAPI } from '$lib/api/admin.js';
	let invitationUrl = $state('');
	let creating = $state(false);
	async function copyInvitation() {
		try { await navigator.clipboard.writeText(invitationUrl); toast.success(locale.language === 'id' ? 'Tautan disalin.' : 'Link copied.'); }
		catch { window.prompt(locale.language === 'id' ? 'Salin tautan:' : 'Copy link:', invitationUrl); }
	}
	import { locale } from '$lib/locale.svelte.js';
	type CreateQuotaChoice = 'default' | 'override_gb' | 'unlimited';

	let {
		open = $bindable(false),
		defaultQuota,
		onCreated
	}: {
		open?: boolean;
		defaultQuota: DefaultQuotaPolicy;
		onCreated?: () => void;
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

	async function submit() {
		if (!canSubmit || creating) return;
		creating = true;
		const mode = quotaChoice as AdminQuotaMode;
		try {
			const result = await adminAPI.createUser({
				username: username.trim(),
				email: email.trim(),
				quota_mode: mode === 'unlimited' ? 'UNLIMITED' : mode === 'override_gb' ? 'CUSTOM' : 'DEFAULT',
				quota_gb: mode === 'override_gb' ? Number(quotaGb) : undefined
			});
			if (result.error || !result.data?.invitation_url) {
				toast.error(result.error ?? (locale.language === 'id' ? 'Undangan tidak tersedia.' : 'Invitation link unavailable.'));
				return;
			}
			invitationUrl = new URL(result.data.invitation_url, window.location.origin).toString();
			onCreated?.();
			open = false;
		} finally {
			creating = false;
		}
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

			<div class="rounded-xl border border-subtle bg-app p-3 text-sm leading-relaxed text-main">
				{locale.language === 'id' ? 'Pengguna akan menerima undangan untuk mengatur kata sandi.' : 'User will receive an invitation to set a password.'}
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
				<Button type="submit" disabled={!canSubmit || creating}>
					{creating ? (locale.language === 'id' ? 'Membuat…' : 'Creating…') : (locale.language === 'id' ? 'Buat Pengguna' : 'Create user')}
				</Button>
			</Dialog.Footer>
		</form>
	</Dialog.Content>
</Dialog.Root>

{#if invitationUrl}
	<Dialog.Root open={!!invitationUrl} onOpenChange={(value) => { if (!value) invitationUrl = ''; }}>
		<Dialog.Content class="bg-surface text-main sm:max-w-md">
			<Dialog.Header>
				<Dialog.Title>{locale.language === 'id' ? 'Undangan pengguna siap' : 'User invitation ready'}</Dialog.Title>
				<Dialog.Description>{locale.language === 'id' ? 'Salin tautan ini sekarang. Tautan hanya dapat digunakan sekali dan berlaku 24 jam.' : 'Copy this link now. It can be used once and expires in 24 hours.'}</Dialog.Description>
			</Dialog.Header>
			<div class="flex flex-col gap-3">
				<Input readonly value={invitationUrl} aria-label={locale.language === 'id' ? 'Tautan undangan' : 'Invitation link'} />
				<Button type="button" class="min-h-11 focus-visible:outline focus-visible:outline-2" onclick={copyInvitation}>{locale.language === 'id' ? 'Salin Tautan' : 'Copy Link'}</Button>
			</div>
		</Dialog.Content>
	</Dialog.Root>
{/if}
