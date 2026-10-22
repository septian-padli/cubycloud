<script lang="ts">
	import {
		Dialog,
		DialogContent,
		DialogDescription,
		DialogFooter,
		DialogHeader,
		DialogTitle
	} from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { Tabs, TabsList, TabsTrigger, TabsContent } from '$lib/components/ui/tabs/index.js';
	import { Switch } from '$lib/components/ui/switch/index.js';
	import { Cloud, HardDrive } from '@lucide/svelte';
	import { toast } from 'svelte-sonner';
	import { adminAPI } from '$lib/api/admin.js';
	import { locale } from '$lib/locale.svelte.js';

	let {
		open = $bindable(false),
		onAdded
	}: {
		open: boolean;
		onAdded: () => void;
	} = $props();

	let activeTab = $state<'local' | 'cloud'>('local');
	let adding = $state(false);

	// Local node form
	let localName = $state('');
	let localPath = $state('');
	let localCapacity = $state(100);
	let localEnabled = $state(true);
	let localNameError = $state('');

	// Cloud node form
	let cloudName = $state('');
	let cloudEndpoint = $state('');
	let cloudBucket = $state('');
	let cloudRegion = $state('');
	let cloudAccessKey = $state('');
	let cloudSecretKey = $state('');
	let cloudCapacity = $state(500);
	let cloudEnabled = $state(true);
	let cloudNameError = $state('');
	const canAddLocal = $derived(localPath.trim() !== '' && localCapacity >= 1);

	const canAddCloud = $derived(
		cloudEndpoint.trim() !== '' &&
			cloudBucket.trim() !== '' &&
			cloudAccessKey.trim() !== '' &&
			cloudSecretKey.trim() !== '' &&
			cloudCapacity >= 1
	);

	function resetLocalForm() {
		localName = '';
		localPath = '';
		localCapacity = 100;
		localEnabled = true;
		localNameError = '';
	}

	function resetCloudForm() {
		cloudName = '';
		cloudEndpoint = '';
		cloudBucket = '';
		cloudRegion = '';
		cloudAccessKey = '';
		cloudSecretKey = '';
		cloudCapacity = 500;
		cloudEnabled = true;
		cloudNameError = '';
	}

	async function handleAddLocal() {
		if (!canAddLocal) return;
		if (!localName.trim()) {
			localNameError = locale.language === 'id' ? 'Nama node wajib diisi' : 'Node name is required';
			return;
		}

		adding = true;
		localNameError = '';
		const resp = await adminAPI.addLocalNode({
			name: localName.trim(),
			path: localPath,
			capacity_cap_gb: localCapacity,
			enabled: localEnabled
		});
		adding = false;

		if (resp.error) {
			if (/name|required|duplicate|already exists/i.test(resp.error))
				localNameError =
					locale.language === 'id'
						? /duplicate|already exists/i.test(resp.error)
							? 'Nama node sudah digunakan'
							: 'Nama node wajib diisi'
						: /duplicate|already exists/i.test(resp.error)
							? 'Node name already exists'
							: 'Node name is required';
			else
				toast.error(
					locale.language === 'id' ? 'Gagal menambah node lokal' : 'Failed to add local node',
					{ description: resp.error }
				);
		} else {
			toast.success(
				locale.language === 'id'
					? 'Node lokal berhasil ditambahkan'
					: 'Local node added successfully'
			);
			resetLocalForm();
			open = false;
			onAdded();
		}
	}

	async function handleAddCloud() {
		if (!canAddCloud) return;
		if (!cloudName.trim()) {
			cloudNameError = locale.language === 'id' ? 'Nama node wajib diisi' : 'Node name is required';
			return;
		}
		adding = true;
		cloudNameError = '';
		const resp = await adminAPI.addCloudNode({
			name: cloudName.trim(),
			endpoint: cloudEndpoint,
			bucket: cloudBucket,
			region: cloudRegion,
			access_key: cloudAccessKey,
			secret_key: cloudSecretKey,
			capacity_cap_gb: cloudCapacity,
			enabled: cloudEnabled
		});
		adding = false;

		if (resp.error) {
			if (/name.*(?:required|already exists)|nama.*(?:wajib|sudah|digunakan)/i.test(resp.error))
				cloudNameError =
					locale.language === 'id'
						? /already exists|sudah|digunakan/i.test(resp.error)
							? 'Nama node sudah digunakan'
							: 'Nama node wajib diisi'
						: /already exists/i.test(resp.error)
							? 'Node name already exists'
							: 'Node name is required';
			else
				toast.error(
					locale.language === 'id' ? 'Gagal menambah node cloud' : 'Failed to add cloud node',
					{ description: resp.error }
				);
		} else {
			toast.success(
				locale.language === 'id'
					? 'Node cloud berhasil ditambahkan'
					: 'Cloud node added successfully'
			);
			resetCloudForm();
			open = false;
			onAdded();
		}
	}

	function handleOpenChange(newOpen: boolean) {
		if (!adding) {
			open = newOpen;
			if (!newOpen) {
				resetLocalForm();
				resetCloudForm();
			}
		}
	}
</script>

<Dialog bind:open onOpenChange={handleOpenChange}>
	<DialogContent class="max-w-lg">
		<DialogHeader>
			<DialogTitle
				>{locale.language === 'id' ? 'Tambah Node Penyimpanan' : 'Add storage node'}</DialogTitle
			>
			<DialogDescription
				>{locale.language === 'id'
					? 'Tambahkan disk lokal atau penyimpanan cloud yang kompatibel dengan S3'
					: 'Add a local disk or S3-compatible cloud storage'}</DialogDescription
			>
		</DialogHeader>

		<Tabs bind:value={activeTab} class="w-full">
			<TabsList variant="line" class="grid w-full grid-cols-2">
				<TabsTrigger value="local" class="gap-1.5">
					<HardDrive class="size-3.5" />
					{locale.language === 'id' ? 'Disk Lokal' : 'Local disk'}
				</TabsTrigger>
				<TabsTrigger value="cloud" class="gap-1.5">
					<Cloud class="size-3.5" />
					{locale.language === 'id' ? 'Cloud (S3)' : 'Cloud (S3)'}
				</TabsTrigger>
			</TabsList>

			<TabsContent value="local" class="space-y-4 pt-4">
				<div class="space-y-2">
					<Label for="local-name">{locale.language === 'id' ? 'Nama Node' : 'Node name'}</Label>
					<Input
						id="local-name"
						data-testid="input-local-node-name"
						bind:value={localName}
						oninput={() => (localNameError = '')}
						placeholder={locale.language === 'id' ? 'Disk Lokal 2' : 'Local disk 2'}
						disabled={adding}
						aria-invalid={!!localNameError}
						aria-describedby={localNameError ? 'local-name-error' : undefined}
					/>
					{#if localNameError}
						<p
							id="local-name-error"
							data-testid="error-local-node-name"
							class="text-xs text-destructive"
							role="alert"
						>
							{localNameError}
						</p>
					{/if}
				</div>

				<div class="space-y-2">
					<Label for="local-path"
						>{locale.language === 'id' ? 'Path Direktori' : 'Directory path'}</Label
					>
					<Input
						id="local-path"
						bind:value={localPath}
						placeholder="/app/data/storage/disk2"
						disabled={adding}
					/>
					<p class="text-xs text-muted">
						{locale.language === 'id'
							? 'Path absolut ke direktori penyimpanan. Direktori harus sudah ada.'
							: 'Absolute path to the storage directory. The directory must already exist.'}
					</p>
				</div>

				<div class="space-y-2">
					<Label for="local-capacity"
						>{locale.language === 'id' ? 'Kapasitas Maksimal (GB)' : 'Maximum capacity (GB)'}</Label
					>
					<Input
						id="local-capacity"
						type="number"
						min="1"
						bind:value={localCapacity}
						disabled={adding}
					/>
					<p class="text-xs text-muted">
						{locale.language === 'id'
							? 'Batas kapasitas node ini (minimal 1 GB).'
							: 'Capacity limit for this node (minimum 1 GB).'}
					</p>
				</div>

				<div class="flex items-center justify-between">
					<div class="space-y-0.5">
						<Label for="local-enabled"
							>{locale.language === 'id' ? 'Aktifkan Node' : 'Enable node'}</Label
						>
						<p class="text-xs text-muted">
							{locale.language === 'id'
								? 'Node langsung digunakan setelah ditambahkan'
								: 'The node will be used immediately after being added'}
						</p>
					</div>
					<Switch id="local-enabled" bind:checked={localEnabled} disabled={adding} />
				</div>
			</TabsContent>

			<TabsContent value="cloud" class="space-y-4 pt-4">
				<div class="space-y-2">
					<Label for="cloud-name">{locale.language === 'id' ? 'Nama Node' : 'Node name'}</Label>
					<Input
						id="cloud-name"
						data-testid="input-cloud-node-name"
						bind:value={cloudName}
						oninput={() => (cloudNameError = '')}
						placeholder={locale.language === 'id' ? 'S3 Produksi' : 'S3 Production'}
						disabled={adding}
						aria-invalid={!!cloudNameError}
						aria-describedby={cloudNameError ? 'cloud-name-error' : undefined}
					/>
					{#if cloudNameError}
						<p id="cloud-name-error" class="text-xs text-destructive" role="alert">
							{cloudNameError}
						</p>
					{/if}
				</div>

				<div class="space-y-2">
					<Label for="cloud-endpoint">Endpoint</Label>
					<Input
						id="cloud-endpoint"
						bind:value={cloudEndpoint}
						placeholder="https://s3.amazonaws.com"
						disabled={adding}
					/>
				</div>

				<div class="grid grid-cols-2 gap-3">
					<div class="space-y-2">
						<Label for="cloud-bucket">Bucket</Label>
						<Input
							id="cloud-bucket"
							bind:value={cloudBucket}
							placeholder="my-bucket"
							disabled={adding}
						/>
					</div>
					<div class="space-y-2">
						<Label for="cloud-region">{locale.language === 'id' ? 'Wilayah' : 'Region'}</Label>
						<Input
							id="cloud-region"
							bind:value={cloudRegion}
							placeholder="us-east-1"
							disabled={adding}
						/>
					</div>
				</div>

				<div class="space-y-2">
					<Label for="cloud-access-key"
						>{locale.language === 'id' ? 'Kunci Akses' : 'Access key'}</Label
					>
					<Input
						id="cloud-access-key"
						bind:value={cloudAccessKey}
						placeholder="AKIA..."
						disabled={adding}
					/>
				</div>

				<div class="space-y-2">
					<Label for="cloud-secret-key"
						>{locale.language === 'id' ? 'Kunci Rahasia' : 'Secret key'}</Label
					>
					<Input
						id="cloud-secret-key"
						type="password"
						bind:value={cloudSecretKey}
						placeholder="••••••••"
						disabled={adding}
					/>
				</div>

				<div class="space-y-2">
					<Label for="cloud-capacity"
						>{locale.language === 'id' ? 'Kapasitas Maksimal (GB)' : 'Maximum capacity (GB)'}</Label
					>
					<Input
						id="cloud-capacity"
						type="number"
						min="1"
						bind:value={cloudCapacity}
						disabled={adding}
					/>
					<p class="text-xs text-muted">
						{locale.language === 'id'
							? 'Batas kapasitas penyimpanan cloud ini (minimal 1 GB).'
							: 'Capacity limit for this cloud storage (minimum 1 GB).'}
					</p>
				</div>

				<div class="flex items-center justify-between">
					<div class="space-y-0.5">
						<Label for="cloud-enabled"
							>{locale.language === 'id' ? 'Aktifkan Node' : 'Enable node'}</Label
						>
						<p class="text-xs text-muted">
							{locale.language === 'id'
								? 'Node langsung digunakan setelah ditambahkan'
								: 'The node will be used immediately after being added'}
						</p>
					</div>
					<Switch id="cloud-enabled" bind:checked={cloudEnabled} disabled={adding} />
				</div>
			</TabsContent>
		</Tabs>

		<DialogFooter>
			<Button variant="outline" onclick={() => (open = false)} disabled={adding}
				>{locale.language === 'id' ? 'Batal' : 'Cancel'}</Button
			>
			{#if activeTab === 'local'}
				<Button onclick={handleAddLocal} disabled={!canAddLocal || adding}>
					{adding
						? locale.language === 'id'
							? 'Menambahkan...'
							: 'Adding...'
						: locale.language === 'id'
							? 'Tambah Node Lokal'
							: 'Add local node'}
				</Button>
			{:else}
				<Button onclick={handleAddCloud} disabled={!canAddCloud || adding}>
					{adding
						? locale.language === 'id'
							? 'Menambahkan...'
							: 'Adding...'
						: locale.language === 'id'
							? 'Tambah Node Cloud'
							: 'Add cloud node'}
				</Button>
			{/if}
		</DialogFooter>
	</DialogContent>
</Dialog>
