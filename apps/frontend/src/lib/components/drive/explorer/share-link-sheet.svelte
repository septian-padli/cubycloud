<script lang="ts">
	import type { FileEntry } from '$lib/types/drive.js';
	import * as Sheet from '$lib/components/ui/sheet/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { Switch } from '$lib/components/ui/switch/index.js';
	import { toast } from 'svelte-sonner';

	let {
		open = $bindable(false),
		entry = null
	}: {
		open?: boolean;
		entry?: FileEntry | null;
	} = $props();

	let publicLink = $state(true);
	let usePassword = $state(false);
	let password = $state('');
	let expiresInDays = $state('7');

	$effect(() => {
		if (open) {
			publicLink = true;
			usePassword = false;
			password = '';
			expiresInDays = '7';
		}
	});

	function submit() {
		toast.success('Tautan berbagi dibuat (mock)', {
			description: entry ? entry.name : undefined
		});
		open = false;
	}
</script>

<Sheet.Root bind:open>
	<Sheet.Content side="right" class="border-subtle bg-surface sm:max-w-md">
		<Sheet.Header>
			<Sheet.Title>Bagikan tautan</Sheet.Title>
			<Sheet.Description>
				{entry?.name ?? 'Berkas'} — pengaturan berbagi (UI mock).
			</Sheet.Description>
		</Sheet.Header>

		<div class="flex flex-1 flex-col gap-5 px-4">
			<div class="flex items-center justify-between gap-3">
				<div>
					<p class="text-sm font-medium text-main">Publik</p>
					<p class="text-xs text-muted">Siapa saja dengan tautan dapat mengakses</p>
				</div>
				<Switch bind:checked={publicLink} aria-label="Tautan publik" />
			</div>

			<div class="flex items-center justify-between gap-3">
				<div>
					<p class="text-sm font-medium text-main">Lindungi kata sandi</p>
					<p class="text-xs text-muted">Opsional untuk tautan publik</p>
				</div>
				<Switch bind:checked={usePassword} aria-label="Aktifkan kata sandi" />
			</div>

			{#if usePassword}
				<div class="grid gap-2">
					<Label for="share-password">Kata sandi</Label>
					<Input
						id="share-password"
						type="password"
						bind:value={password}
						placeholder="••••••••"
					/>
				</div>
			{/if}

			<div class="grid gap-2">
				<Label for="share-expiry">Kedaluwarsa (hari)</Label>
				<select
					id="share-expiry"
					bind:value={expiresInDays}
					class="h-9 w-full rounded-full border border-subtle bg-surface px-3 text-sm text-main shadow-none outline-none focus-visible:border-ring focus-visible:ring-3 focus-visible:ring-ring/50"
				>
					<option value="1">1 hari</option>
					<option value="7">7 hari</option>
					<option value="30">30 hari</option>
					<option value="90">90 hari</option>
				</select>
			</div>
		</div>

		<Sheet.Footer>
			<Button type="button" variant="secondary" onclick={() => (open = false)}>Batal</Button>
			<Button type="button" onclick={submit}>Buat tautan</Button>
		</Sheet.Footer>
	</Sheet.Content>
</Sheet.Root>
