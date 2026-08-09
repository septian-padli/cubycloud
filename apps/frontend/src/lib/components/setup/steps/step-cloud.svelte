<script lang="ts">
	import type { CloudForm, CloudProvider } from '$lib/types/setup.js';
	import { PROVIDER_OPTIONS } from '$lib/types/setup.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import * as Alert from '$lib/components/ui/alert/index.js';
	import * as RadioGroup from '$lib/components/ui/radio-group/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { Switch } from '$lib/components/ui/switch/index.js';
	import PasswordInput from '$lib/components/password-input.svelte';
	import { Box, CheckCircle2, Cloud, LoaderCircle, Server, XCircle } from '@lucide/svelte';
	import { cn } from '$lib/utils.js';

	let {
		cloud = $bindable()
	}: {
		cloud: CloudForm;
	} = $props();

	type TestState = 'idle' | 'loading' | 'success' | 'error';
	let testState = $state<TestState>('idle');

	const providerMeta = $derived(
		PROVIDER_OPTIONS.find((p) => p.value === cloud.provider) ?? PROVIDER_OPTIONS[0]
	);

	const formReady = $derived(
		cloud.endpoint.trim().length > 0 &&
			cloud.bucket.trim().length > 0 &&
			cloud.accessKey.trim().length > 0 &&
			cloud.secretKey.trim().length > 0
	);

	function providerIcon(value: CloudProvider) {
		if (value === 'R2') return Cloud;
		if (value === 'S3') return Box;
		return Server;
	}

	async function testConnection() {
		if (!formReady || testState === 'loading') return;
		testState = 'loading';
		await new Promise((resolve) => setTimeout(resolve, 1000));
		// Mock UI — always succeed when fields are filled
		testState = 'success';
	}

	function onProviderChange(value: string | undefined) {
		if (!value) return;
		cloud.provider = value as CloudProvider;
		testState = 'idle';
	}
</script>

<Card.Root class="rounded-xl border-subtle bg-surface shadow-none">
	<Card.Header>
		<Card.Title class="text-lg font-semibold text-main">Cloud (Opsional)</Card.Title>
		<Card.Description class="text-sm text-muted">
			Hubungkan Cloudflare R2, AWS S3, atau penyedia S3 Compatible. Langkah ini bisa dilewati.
		</Card.Description>
	</Card.Header>
	<Card.Content class="space-y-5">
		<div
			class="flex items-center justify-between gap-4 rounded-xl border border-subtle bg-app px-4 py-3"
		>
			<div>
				<p class="text-sm font-medium text-main">Aktifkan node cloud</p>
				<p class="text-xs text-muted">Tambahkan penyimpanan objek ke pool.</p>
			</div>
			<Switch
				checked={cloud.enabled}
				aria-label="Aktifkan node cloud"
				onCheckedChange={(v) => {
					cloud.enabled = v;
					testState = 'idle';
				}}
			/>
		</div>

		{#if cloud.enabled}
			<div class="space-y-4">
				<div>
					<p class="mb-2 text-sm font-medium text-main">Penyedia</p>
					<RadioGroup.Root
						value={cloud.provider}
						onValueChange={onProviderChange}
						class="grid gap-3 sm:grid-cols-3"
					>
						{#each PROVIDER_OPTIONS as option (option.value)}
							{@const selected = cloud.provider === option.value}
							{@const Icon = providerIcon(option.value)}
							<label
								class={cn(
									'flex cursor-pointer items-start gap-3 rounded-xl border border-subtle bg-app px-4 py-3',
									selected && 'border-primary bg-primary/5'
								)}
							>
								<RadioGroup.Item value={option.value} id="cloud-{option.value}" class="mt-0.5" />
								<div class="min-w-0 space-y-1">
									<div class="flex items-center gap-2">
										<Icon class="size-4 shrink-0 stroke-2 text-secondary-accent" />
										<Label
											for="cloud-{option.value}"
											class="cursor-pointer text-sm font-medium text-main"
										>
											{option.label}
										</Label>
									</div>
									<p class="text-xs leading-normal text-muted">{option.description}</p>
								</div>
							</label>
						{/each}
					</RadioGroup.Root>
				</div>

				<div class="space-y-2">
					<Label for="cloud-endpoint">Endpoint</Label>
					<Input
						id="cloud-endpoint"
						class="rounded-lg"
						placeholder={providerMeta.endpointPlaceholder}
						bind:value={cloud.endpoint}
						oninput={() => {
							testState = 'idle';
						}}
					/>
				</div>
				<div class="space-y-2">
					<Label for="cloud-bucket">Bucket name</Label>
					<Input
						id="cloud-bucket"
						class="rounded-lg"
						placeholder="cubycloud-files"
						bind:value={cloud.bucket}
						oninput={() => {
							testState = 'idle';
						}}
					/>
				</div>
				<div class="space-y-2">
					<Label for="cloud-access">Access key</Label>
					<Input
						id="cloud-access"
						class="rounded-lg"
						bind:value={cloud.accessKey}
						autocomplete="off"
						oninput={() => {
							testState = 'idle';
						}}
					/>
				</div>
				<div class="space-y-2">
					<Label for="cloud-secret">Secret key</Label>
					<PasswordInput
						id="cloud-secret"
						bind:value={cloud.secretKey}
						autocomplete="off"
						oninput={() => {
							testState = 'idle';
						}}
					/>
				</div>

				<div class="flex flex-wrap items-center gap-3">
					<Button
						type="button"
						variant="outline"
						class="rounded-lg gap-1.5"
						disabled={!formReady || testState === 'loading'}
						onclick={testConnection}
					>
						{#if testState === 'loading'}
							<LoaderCircle class="size-4 animate-spin stroke-2" />
							Menguji…
						{:else}
							Uji koneksi
						{/if}
					</Button>
					{#if !formReady}
						<p class="text-xs text-muted">Isi semua field untuk menguji koneksi.</p>
					{/if}
				</div>

				{#if testState === 'success'}
					<Alert.Root class="border border-subtle bg-surface">
						<CheckCircle2 class="size-4 stroke-2 text-primary" />
						<Alert.Title class="text-main">Koneksi berhasil</Alert.Title>
						<Alert.Description>
							Mock: kredensial diterima. Backend belum dihubungkan.
						</Alert.Description>
					</Alert.Root>
				{:else if testState === 'error'}
					<Alert.Root variant="destructive" class="border border-subtle">
						<XCircle class="size-4 stroke-2" />
						<Alert.Title>Koneksi gagal</Alert.Title>
						<Alert.Description>
							Tidak dapat menjangkau endpoint. Periksa kredensial dan coba lagi.
						</Alert.Description>
					</Alert.Root>
				{/if}

				<div class="rounded-xl border border-subtle bg-app px-4 py-3">
					<p class="text-xs font-medium text-muted">Ringkasan</p>
					<p class="mt-1 text-sm text-main">
						{providerMeta.label}
						{#if cloud.bucket.trim()}
							· bucket <span class="font-medium">{cloud.bucket.trim()}</span>
						{/if}
					</p>
				</div>
			</div>
		{/if}
	</Card.Content>
</Card.Root>
