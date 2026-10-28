<script lang="ts">
	import type { CloudForm, CloudProvider } from '$lib/types/setup.js';
	import { PROVIDER_OPTIONS } from '$lib/types/setup.js';
	import { testCloudStorage } from '$lib/api/setup.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import * as Alert from '$lib/components/ui/alert/index.js';
	import * as RadioGroup from '$lib/components/ui/radio-group/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { Switch } from '$lib/components/ui/switch/index.js';
	import PasswordInput from '$lib/components/password-input.svelte';
	import { Box, CheckCircle2, Cloud, Info, LoaderCircle, Server, XCircle } from '@lucide/svelte';
	import { cn } from '$lib/utils.js';
	import { locale } from '$lib/locale.svelte.js';
	const tx = (id: string, en: string) => (locale.language === 'id' ? id : en);
	let {
		cloud = $bindable(),
		noLocalNodes = false
	}: {
		cloud: CloudForm;
		noLocalNodes?: boolean;
	} = $props();

	type TestState = 'idle' | 'loading' | 'success' | 'error';
	let testState = $state<TestState>('idle');
	let testErrorMessage = $state<string>('');
	const providerMeta = $derived(
		PROVIDER_OPTIONS.find((p) => p.value === cloud.provider) ?? PROVIDER_OPTIONS[0]
	);

	const formReady = $derived(
		cloud.nodeName.trim().length > 0 &&
			cloud.endpoint.trim().length > 0 &&
			cloud.region.trim().length > 0 &&
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
		testErrorMessage = '';
		const capGB =
			typeof cloud.capacityCapGb === 'number'
				? cloud.capacityCapGb
				: typeof cloud.capacityCapGb === 'string' && cloud.capacityCapGb.trim() !== ''
					? Number(cloud.capacityCapGb)
					: 0;
		try {
			await testCloudStorage({
				enabled: cloud.enabled,
				provider: cloud.provider,
				node_name: cloud.nodeName.trim(),
				endpoint: cloud.endpoint.trim(),
				bucket: cloud.bucket.trim(),
				region: cloud.region.trim(),
				access_key: cloud.accessKey.trim(),
				secret_key: cloud.secretKey.trim(),
				capacity_cap_gb: Number.isFinite(capGB) ? capGB : 0
			});
			testState = 'success';
		} catch (err: unknown) {
			testState = 'error';
			testErrorMessage =
				err instanceof Error
					? err.message
					: tx(
							'Tidak dapat menjangkau endpoint. Periksa konfigurasi dan coba lagi.',
							'Cannot reach endpoint. Check your configuration and try again.'
						);
		}
	}
	function onProviderChange(value: string | undefined) {
		if (!value) return;
		cloud.provider = value as CloudProvider;
		testState = 'idle';
		testErrorMessage = '';
	}
</script>

<Card.Root class="rounded-xl border-subtle bg-surface shadow-none">
	<Card.Header>
		<Card.Title class="text-lg font-semibold text-main"
			>{tx('Cloud (Opsional)', 'Cloud (Optional)')}</Card.Title
		>
		<Card.Description class="text-sm text-muted">
			{tx(
				'Hubungkan penyimpanan S3 Compatible. Langkah ini bisa dilewati.',
				'Connect S3-compatible storage. This step is optional.'
			)}
		</Card.Description>
	</Card.Header>
	<Card.Content class="space-y-5">
		<div
			class="flex items-center justify-between gap-4 rounded-xl border border-subtle bg-app px-4 py-3"
		>
			<div>
				<p class="text-sm font-medium text-main">
					{tx('Aktifkan node cloud', 'Enable cloud node')}
				</p>
				<p class="text-xs text-muted">
					{tx('Tambahkan penyimpanan objek ke pool.', 'Add object storage to the pool.')}
				</p>
			</div>
			<Switch
				checked={cloud.enabled}
				aria-label={tx('Aktifkan node cloud', 'Enable cloud node')}
				onCheckedChange={(v) => {
					cloud.enabled = v;
					testState = 'idle';
					testErrorMessage = '';
				}}
			/>
		</div>
		{#if noLocalNodes}
			<Alert.Root>
				<Info class="size-4 stroke-2" />
				<Alert.Title
					>{tx(
						'Minimal satu penyimpanan wajib diisi',
						'At least one storage location is required'
					)}</Alert.Title
				>
				<Alert.Description>
					{tx(
						'Tidak ada node lokal yang dipilih. Aktifkan node cloud di bawah untuk bisa menyelesaikan setup.',
						'No local nodes are selected. Enable a cloud node below to complete setup.'
					)}
				</Alert.Description>
			</Alert.Root>
		{/if}

		{#if cloud.enabled}
			<div class="space-y-4">
				<div>
					<p class="mb-2 text-sm font-medium text-main">{tx('Penyedia', 'Provider')}</p>
					<RadioGroup.Root
						value={cloud.provider}
						onValueChange={onProviderChange}
						class="grid gap-3"
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
									<p class="text-xs leading-normal text-muted">
										{locale.language === 'id'
											? option.description
											: 'Any S3-compatible storage provider (MinIO, Wasabi, Backblaze B2, Cloudflare R2, AWS S3).'}
									</p>
								</div>
							</label>
						{/each}
					</RadioGroup.Root>
				</div>

				<div class="space-y-2">
					<Label for="cloud-node-name">{tx('Nama node', 'Node name')}</Label>
					<Input
						id="cloud-node-name"
						placeholder="s3-primary"
						bind:value={cloud.nodeName}
						oninput={() => {
							testState = 'idle';
						}}
					/>
				</div>
				<div class="space-y-2">
					<Label for="cloud-endpoint">{tx('Endpoint', 'Endpoint')}</Label>
					<Input
						id="cloud-endpoint"
						placeholder={providerMeta.endpointPlaceholder}
						bind:value={cloud.endpoint}
						oninput={() => {
							testState = 'idle';
						}}
					/>
				</div>
				<div class="space-y-2">
					<Label for="cloud-region">{tx('Region', 'Region')}</Label>
					<Input
						id="cloud-region"
						placeholder="us-east-1"
						bind:value={cloud.region}
						oninput={() => {
							testState = 'idle';
						}}
					/>
				</div>
				<div class="space-y-2">
					<Label for="cloud-bucket">{tx('Nama bucket', 'Bucket name')}</Label>
					<Input
						id="cloud-bucket"
						placeholder="cubycloud-files"
						bind:value={cloud.bucket}
						oninput={() => {
							testState = 'idle';
						}}
					/>
				</div>
				<div class="space-y-2">
					<Label for="cloud-access">{tx('Access key', 'Access key')}</Label>
					<Input
						id="cloud-access"
						bind:value={cloud.accessKey}
						autocomplete="off"
						oninput={() => {
							testState = 'idle';
						}}
					/>
				</div>
				<div class="space-y-2">
					<Label for="cloud-secret">{tx('Secret key', 'Secret key')}</Label>
					<PasswordInput
						id="cloud-secret"
						bind:value={cloud.secretKey}
						autocomplete="off"
						oninput={() => {
							testState = 'idle';
						}}
					/>
				</div>
				<div class="space-y-2">
					<Label for="cloud-cap">
						{tx('Batas kapasitas node (GiB) (Opsional)', 'Node capacity limit (GiB) (Optional)')}
					</Label>
					<Input
						id="cloud-cap"
						type="number"
						min={1}
						placeholder={tx('Tanpa batas', 'Unlimited')}
						bind:value={cloud.capacityCapGb}
						oninput={() => {
							testState = 'idle';
						}}
					/>
					<p class="text-xs text-muted">
						{tx(
							'Kosongkan untuk kapasitas tanpa batas kuota.',
							'Leave blank for unlimited capacity quota.'
						)}
					</p>
				</div>

				<div class="flex flex-wrap items-center gap-3">
					<Button
						type="button"
						variant="outline"
						class="gap-1.5"
						disabled={!formReady || testState === 'loading'}
						onclick={testConnection}
					>
						{#if testState === 'loading'}
							<LoaderCircle class="size-4 animate-spin stroke-2" />
							{tx('Menguji…', 'Testing…')}
						{:else}
							{tx('Uji koneksi', 'Test connection')}
						{/if}
					</Button>
					{#if !formReady}
						<p class="text-xs text-muted">
							{tx(
								'Isi semua field untuk menguji koneksi.',
								'Fill in all fields to test the connection.'
							)}
						</p>
					{/if}
				</div>

				{#if testState === 'success'}
					<Alert.Root variant="default" class="border-emerald-500/20 bg-emerald-500/5">
						<CheckCircle2 class="size-4 stroke-2 text-emerald-600 dark:text-emerald-400" />
						<Alert.Title class="text-emerald-800 dark:text-emerald-200"
							>{tx('Koneksi berhasil', 'Connection successful')}</Alert.Title
						>
						<Alert.Description class="text-emerald-700 dark:text-emerald-300">
							{tx(
								'Kredensial dan akses bucket S3 terverifikasi.',
								'S3 credentials and bucket access verified.'
							)}
						</Alert.Description>
					</Alert.Root>
				{:else if testState === 'error'}
					<Alert.Root variant="destructive">
						<XCircle class="size-4 stroke-2" />
						<Alert.Title>{tx('Koneksi gagal', 'Connection failed')}</Alert.Title>
						<Alert.Description>
							{testErrorMessage ||
								tx(
									'Tidak dapat menjangkau endpoint. Periksa kredensial dan coba lagi.',
									'Could not reach the endpoint. Check your credentials and try again.'
								)}
						</Alert.Description>
					</Alert.Root>
				{/if}

				<div class="rounded-xl border border-subtle bg-app px-4 py-3">
					<p class="text-xs font-medium text-muted">{tx('Ringkasan', 'Summary')}</p>
					<p class="mt-1 text-sm text-main">
						{providerMeta.label}
						{#if cloud.nodeName.trim()}
							· node <span class="font-medium">{cloud.nodeName.trim()}</span>
						{/if}
						{#if cloud.bucket.trim()}
							· bucket <span class="font-medium">{cloud.bucket.trim()}</span>
						{/if}
						{#if cloud.region.trim()}
							· region <span class="font-medium">{cloud.region.trim()}</span>
						{/if}
						{#if cloud.capacityCapGb && Number(cloud.capacityCapGb) > 0}
							· <span class="font-medium">{cloud.capacityCapGb} GiB</span>
						{/if}
					</p>
				</div>
			</div>
		{/if}
	</Card.Content>
</Card.Root>
