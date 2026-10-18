<script lang="ts">
	import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { Tabs, TabsList, TabsTrigger, TabsContent } from '$lib/components/ui/tabs/index.js';
	import { Switch } from '$lib/components/ui/switch/index.js';
	import { toast } from 'svelte-sonner';
	import { adminAPI } from '$lib/api/admin.js';
	import { HardDrive, Cloud } from '@lucide/svelte';


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
	let localPath = $state('');
	let localCapacity = $state(100);
	let localEnabled = $state(true);

	// Cloud node form
	let cloudName = $state('');
	let cloudEndpoint = $state('');
	let cloudBucket = $state('');
	let cloudRegion = $state('');
	let cloudAccessKey = $state('');
	let cloudSecretKey = $state('');
	let cloudCapacity = $state(500);
	let cloudEnabled = $state(true);

	const canAddLocal = $derived(
		localPath.trim() !== '' && localCapacity >= 1
	);

	const canAddCloud = $derived(
		cloudName.trim() !== '' &&
		cloudEndpoint.trim() !== '' &&
		cloudBucket.trim() !== '' &&
		cloudAccessKey.trim() !== '' &&
		cloudSecretKey.trim() !== '' &&
		cloudCapacity >= 1
	);

	function resetLocalForm() {
		localPath = '';
		localCapacity = 100;
		localEnabled = true;
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
	}

	async function handleAddLocal() {
		if (!canAddLocal) return;

		adding = true;
		const resp = await adminAPI.addLocalNode({
			path: localPath,
			capacity_cap_gb: localCapacity,
			enabled: localEnabled
		});
		adding = false;

		if (resp.error) {
			toast.error('Gagal menambah node lokal', { description: resp.error });
		} else {
			toast.success('Node lokal berhasil ditambahkan');
			resetLocalForm();
			open = false;
			onAdded();
		}
	}

	async function handleAddCloud() {
		if (!canAddCloud) return;

		adding = true;
		const resp = await adminAPI.addCloudNode({
			name: cloudName,
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
			toast.error('Gagal menambah node cloud', { description: resp.error });
		} else {
			toast.success('Node cloud berhasil ditambahkan');
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
			<DialogTitle>Tambah Node Penyimpanan</DialogTitle>
			<DialogDescription>
				Tambahkan disk lokal atau cloud storage S3-compatible
			</DialogDescription>
		</DialogHeader>

		<Tabs bind:value={activeTab} class="w-full">
			<TabsList class="grid w-full grid-cols-2">
				<TabsTrigger value="local" class="gap-1.5">
					<HardDrive class="size-3.5" />
					Disk Lokal
				</TabsTrigger>
				<TabsTrigger value="cloud" class="gap-1.5">
					<Cloud class="size-3.5" />
					Cloud (S3)
				</TabsTrigger>
			</TabsList>

			<TabsContent value="local" class="space-y-4 pt-4">
				<div class="space-y-2">
					<Label for="local-path">Path Direktori</Label>
					<Input
						id="local-path"
						bind:value={localPath}
						placeholder="/app/data/storage/disk2"
						disabled={adding}
					/>
					<p class="text-xs text-muted">
						Path absolut ke direktori storage. Direktori harus sudah ada.
					</p>
				</div>

				<div class="space-y-2">
					<Label for="local-capacity">Kapasitas Maksimal (GB)</Label>
					<Input
						id="local-capacity"
						type="number"
						min="1"
						bind:value={localCapacity}
						disabled={adding}
					/>
					<p class="text-xs text-muted">
						Batas kapasitas yang diizinkan untuk node ini (minimal 1 GB).
					</p>
				</div>

				<div class="flex items-center justify-between">
					<div class="space-y-0.5">
						<Label for="local-enabled">Aktifkan Node</Label>
						<p class="text-xs text-muted">Node langsung digunakan setelah ditambahkan</p>
					</div>
					<Switch id="local-enabled" bind:checked={localEnabled} disabled={adding} />
				</div>
			</TabsContent>

			<TabsContent value="cloud" class="space-y-4 pt-4">
				<div class="space-y-2">
					<Label for="cloud-name">Nama Node</Label>
					<Input
						id="cloud-name"
						bind:value={cloudName}
						placeholder="S3 Production"
						disabled={adding}
					/>
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
						<Label for="cloud-region">Region</Label>
						<Input
							id="cloud-region"
							bind:value={cloudRegion}
							placeholder="us-east-1"
							disabled={adding}
						/>
					</div>
				</div>

				<div class="space-y-2">
					<Label for="cloud-access-key">Access Key</Label>
					<Input
						id="cloud-access-key"
						bind:value={cloudAccessKey}
						placeholder="AKIA..."
						disabled={adding}
					/>
				</div>

				<div class="space-y-2">
					<Label for="cloud-secret-key">Secret Key</Label>
					<Input
						id="cloud-secret-key"
						type="password"
						bind:value={cloudSecretKey}
						placeholder="••••••••"
						disabled={adding}
					/>
				</div>

				<div class="space-y-2">
					<Label for="cloud-capacity">Kapasitas Maksimal (GB)</Label>
					<Input
						id="cloud-capacity"
						type="number"
						min="1"
						bind:value={cloudCapacity}
						disabled={adding}
					/>
					<p class="text-xs text-muted">
						Batas kapasitas untuk cloud storage ini (minimal 1 GB).
					</p>
				</div>

				<div class="flex items-center justify-between">
					<div class="space-y-0.5">
						<Label for="cloud-enabled">Aktifkan Node</Label>
						<p class="text-xs text-muted">Node langsung digunakan setelah ditambahkan</p>
					</div>
					<Switch id="cloud-enabled" bind:checked={cloudEnabled} disabled={adding} />
				</div>
			</TabsContent>
		</Tabs>

		<DialogFooter>
			<Button variant="outline" onclick={() => (open = false)} disabled={adding}>
				Batal
			</Button>
			{#if activeTab === 'local'}
				<Button onclick={handleAddLocal} disabled={!canAddLocal || adding}>
					{adding ? 'Menambahkan...' : 'Tambah Node Lokal'}
				</Button>
			{:else}
				<Button onclick={handleAddCloud} disabled={!canAddCloud || adding}>
					{adding ? 'Menambahkan...' : 'Tambah Node Cloud'}
				</Button>
			{/if}
		</DialogFooter>
	</DialogContent>
</Dialog>
