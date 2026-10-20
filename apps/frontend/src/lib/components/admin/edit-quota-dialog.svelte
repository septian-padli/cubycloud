<script lang="ts">
	import type { AdminQuotaMode, AdminUser, DefaultQuotaPolicy } from '$lib/types/admin.js';
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import * as RadioGroup from '$lib/components/ui/radio-group/index.js';
	import { ConfirmDialog } from '$lib/components/ui/confirm-dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { cn } from '$lib/utils.js';

	type EditQuotaChoice = 'default' | 'override_gb' | 'unlimited';

	let {
		open = $bindable(false),
		user = null,
		defaultQuota,
		onSave
	}: {
		open?: boolean;
		user?: AdminUser | null;
		defaultQuota: DefaultQuotaPolicy;
		onSave: (next: Pick<AdminUser, 'quotaMode' | 'quotaGb'>) => void;
	} = $props();

	let quotaChoice = $state<EditQuotaChoice>('default');
	let quotaGb = $state('50');
	let confirmOpen = $state(false);

	const isSuperAdmin = $derived(String(user?.role).toLowerCase() === 'super_admin');

	const defaultLabel = $derived(
		defaultQuota.mode === 'unlimited' ? 'Unlimited' : `${defaultQuota.gb} GB`
	);

	const canSubmit = $derived(
		!isSuperAdmin &&
			user != null &&
			(quotaChoice !== 'override_gb' ||
				(String(quotaGb).trim() !== '' && !isNaN(Number(quotaGb)) && Number(quotaGb) >= 0))
	);

	$effect(() => {
		if (open && user) {
			const mode = String(user.quotaMode).toLowerCase();
			if (mode === 'unlimited') {
				quotaChoice = 'unlimited';
			} else if (mode === 'override_gb' || mode === 'custom') {
				quotaChoice = 'override_gb';
			} else {
				quotaChoice = 'default';
			}
			quotaGb = String(
				user.quotaGb != null
					? user.quotaGb
					: defaultQuota.mode === 'limited'
						? defaultQuota.gb
						: '50'
			);
		}
	});

	function getTargetQuotaGb(): number {
		if (quotaChoice === 'unlimited') return Infinity;
		if (quotaChoice === 'default') {
			return defaultQuota.mode === 'unlimited' ? Infinity : defaultQuota.gb;
		}
		return Number(quotaGb);
	}

	function getTargetLabel(): string {
		if (quotaChoice === 'unlimited') return 'Unlimited';
		if (quotaChoice === 'default') return defaultLabel;
		return `${Number(quotaGb)} GB`;
	}

	function executeSave() {
		if (!user) return;
		onSave({
			quotaMode: quotaChoice,
			quotaGb: quotaChoice === 'override_gb' ? Number(quotaGb) : undefined
		});
		open = false;
	}

	function submit() {
		if (!canSubmit || !user) return;
		const targetQuota = getTargetQuotaGb();
		if (targetQuota < user.usedGb) {
			confirmOpen = true;
			return;
		}
		executeSave();
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="bg-surface text-main sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title>Edit Kuota</Dialog.Title>
			<Dialog.Description>
				{#if isSuperAdmin}
					Super Admin selalu Unlimited dan tidak dapat diubah menjadi terbatas.
				{:else if user}
					Atur kuota untuk {user.username}. Default sistem: {defaultLabel}.
				{:else}
					Pilih pengguna dari tabel.
				{/if}
			</Dialog.Description>
		</Dialog.Header>

		{#if isSuperAdmin}
			<p class="rounded-xl border border-subtle bg-app px-4 py-3 text-sm text-muted">
				Kuota Super Admin terkunci ke Unlimited.
			</p>
			<Dialog.Footer>
				<Button type="button" variant="secondary" onclick={() => (open = false)}>Tutup</Button>
			</Dialog.Footer>
		{:else}
			<form
				class="grid gap-4"
				onsubmit={(e) => {
					e.preventDefault();
					submit();
				}}
			>
				<RadioGroup.Root bind:value={quotaChoice} class="grid gap-2">
					<label
						class={cn(
							'flex cursor-pointer items-start gap-3 rounded-xl border border-subtle bg-app px-4 py-3',
							quotaChoice === 'override_gb' && 'border-primary bg-primary/5'
						)}
					>
						<RadioGroup.Item value="override_gb" id="edit-quota-gb" class="mt-0.5" />
						<div class="min-w-0 flex-1 space-y-2">
							<Label for="edit-quota-gb" class="cursor-pointer text-sm font-medium text-main">
								Atur GB
							</Label>
							{#if quotaChoice === 'override_gb'}
								<div class="relative flex w-full items-center">
									<Input
										id="edit-quota-gb-value"
										type="number"
										min="0"
										step="any"
										bind:value={quotaGb}
										class="w-full pe-9"
										aria-label="Kuota dalam GB"
									/>
									<span
										class="pointer-events-none absolute end-3 text-xs font-semibold text-muted"
									>
										GB
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
						<RadioGroup.Item value="unlimited" id="edit-quota-unlimited" class="mt-0.5" />
						<div class="min-w-0">
							<Label
								for="edit-quota-unlimited"
								class="cursor-pointer text-sm font-medium text-main"
							>
								Unlimited
							</Label>
							<p class="text-xs text-muted">Tanpa batas kuota My Storage.</p>
						</div>
					</label>

					<label
						class={cn(
							'flex cursor-pointer items-start gap-3 rounded-xl border border-subtle bg-app px-4 py-3',
							quotaChoice === 'default' && 'border-primary bg-primary/5'
						)}
					>
						<RadioGroup.Item value="default" id="edit-quota-default" class="mt-0.5" />
						<div class="min-w-0">
							<Label for="edit-quota-default" class="cursor-pointer text-sm font-medium text-main">
								Reset ke Default ({defaultLabel})
							</Label>
							<p class="text-xs text-muted">Kembali ke kebijakan kuota sistem.</p>
						</div>
					</label>
				</RadioGroup.Root>

				<Dialog.Footer>
					<Button type="button" variant="secondary" onclick={() => (open = false)}>Batal</Button>
					<Button type="submit" disabled={!canSubmit}>Simpan</Button>
				</Dialog.Footer>
			</form>
		{/if}
	</Dialog.Content>
</Dialog.Root>

<ConfirmDialog
	bind:open={confirmOpen}
	title="Konfirmasi Penurunan Kuota"
	description={`Kuota baru (${getTargetLabel()}) lebih kecil dari pemakaian saat ini (${user?.usedGb.toFixed(1)} GB). Pengguna tidak akan dapat mengunggah file baru sampai pemakaian berada di bawah kuota. Apakah Anda yakin ingin melanjutkan?`}
	confirmText="Ya, Simpan"
	cancelText="Batal"
	variant="destructive"
	onConfirm={executeSave}
	onCancel={() => {
		confirmOpen = false;
	}}
/>
