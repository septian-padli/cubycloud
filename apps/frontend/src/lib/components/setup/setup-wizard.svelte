<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { toast } from 'svelte-sonner';
	import { browser } from '$app/environment';
	import { scanStorageNodes, completeSetup } from '$lib/api/setup';
	import type { CompleteSetupRequest } from '$lib/api/setup';
	import {
		SETUP_STEPS,
		type AdminForm,
		type CloudForm,
		type PlacementStrategy,
		type SetupStepId,
		type StorageNode,
		type LinuxDevice
	} from '$lib/types/setup.js';
	import * as Alert from '$lib/components/ui/alert/index.js';
	import SetupStepper from './setup-stepper.svelte';
	import SetupStepNav from './setup-step-nav.svelte';
	import SetupSuccessDialog from './setup-success-dialog.svelte';
	import StepAdmin from './steps/step-admin.svelte';
	import StepStorage from './steps/step-storage.svelte';
	import StepStrategy from './steps/step-strategy.svelte';
	import StepCloud from './steps/step-cloud.svelte';
	import StepRecovery from './steps/step-recovery.svelte';
	import { Info } from '@lucide/svelte';
	import { locale } from '$lib/locale.svelte.js';
	const tx = (id: string, en: string) => (locale.language === 'id' ? id : en);

	let currentStep = $state<SetupStepId>(1);
	let successOpen = $state(false);
	let loadingNodes = $state(true);

	let admin = $state<AdminForm>({
		username: '',
		email: '',
		password: '',
		confirmPassword: ''
	});
	let nodes = $state<StorageNode[]>([]);
	let devices = $state<LinuxDevice[]>([]);
	let strategy = $state<PlacementStrategy>('FILL_FIRST');
	let cloud = $state<CloudForm>({
		enabled: false,
		provider: 'S3_COMPATIBLE',
		nodeName: '',
		endpoint: '',
		region: '',
		bucket: '',
		accessKey: '',
		secretKey: '',
		capacityCapGb: null
	});
	let recoveryKey = $state('');
	let recoveryDownloaded = $state(false);

	const activeNodeCount = $derived(nodes.filter((n) => n.enabled).length + (cloud.enabled ? 1 : 0));

	// Per-step validation — each rule surfaces in its own tab, never only at submit.
	const adminValid = $derived(
		admin.username.trim() !== '' &&
			admin.email.trim() !== '' &&
			admin.password.length >= 8 &&
			admin.password === admin.confirmPassword
	);
	const cloudCapValid = $derived(
		cloud.capacityCapGb === null ||
			cloud.capacityCapGb === undefined ||
			cloud.capacityCapGb === '' ||
			(Number(cloud.capacityCapGb) >= 1 &&
				Math.floor(Number(cloud.capacityCapGb)) === Number(cloud.capacityCapGb))
	);
	const cloudValid = $derived(
		!cloud.enabled ||
			(cloud.nodeName.trim() !== '' &&
				cloud.endpoint.trim() !== '' &&
				cloud.region.trim() !== '' &&
				cloud.bucket.trim() !== '' &&
				cloud.accessKey.trim() !== '' &&
				cloud.secretKey.trim() !== '' &&
				cloudCapValid)
	);

	const canNext = $derived.by(() => {
		switch (currentStep) {
			case 1:
				return adminValid;
			case 2:
				// Local storage is optional; "at least one node" is enforced on step 3
				return true;
			case 3:
				// Cloud is optional, but at least one storage node (local or cloud) is required
				return cloudValid && (cloud.enabled || activeNodeCount > 0);
			case 4:
				return strategy !== null;
			case 5:
				// Final gate re-checks every tab so no validation fires only at submit
				return recoveryDownloaded && adminValid && cloudValid && strategy !== null;
			default:
				return false;
		}
	});

	function goBack() {
		if (currentStep > 1) {
			currentStep = (currentStep - 1) as SetupStepId;
		}
	}
	function goToPreviousStep(step: SetupStepId) {
		if (step < currentStep) currentStep = step;
	}

	async function goNext() {
		if (!canNext) return;

		// Step 4 → Step 5: Generate recovery key
		if (currentStep === 4) {
			generateRecoveryKey();
			currentStep = 5;
			return;
		}

		if (currentStep < 5) {
			currentStep = (currentStep + 1) as SetupStepId;
			return;
		}

		// Step 5: submit with pre-generated recovery key
		try {
			const req: CompleteSetupRequest = {
				admin: {
					username: admin.username,
					email: admin.email,
					password: admin.password,
					recovery_key: recoveryKey // Send client-generated key
				},
				storage_nodes: nodes
					.filter((n) => n.enabled)
					.map((n) => ({
						path: n.path,
						capacity_bytes: n.capacityCapGb * 1024 ** 3,
						enabled: true
					})),
				placement_strategy: strategy
			};
			if (cloud.enabled) {
				const capNum = cloud.capacityCapGb ? Math.floor(Number(cloud.capacityCapGb)) : 0;
				req.cloud = {
					enabled: true,
					provider: cloud.provider,
					node_name: cloud.nodeName.trim(),
					endpoint: cloud.endpoint.trim(),
					region: cloud.region.trim(),
					bucket: cloud.bucket.trim(),
					access_key: cloud.accessKey.trim(),
					secret_key: cloud.secretKey.trim(),
					capacity_cap_gb: capNum > 0 ? capNum : 0
				};
			}
			await completeSetup(req);
			clearLocalStorage();
			// Redirect directly to home (no success dialog needed, key already downloaded)
			goto('/drive/home');
		} catch (err: any) {
			if (err.status === 409) {
				toast.error(tx('Setup sudah selesai', 'Setup already completed'));
			} else if (err.status === 400) {
				toast.error(err.message || tx('Kesalahan validasi', 'Validation error'));
			} else if (err.status === 500) {
				toast.error(tx('Kesalahan server. Silakan coba lagi.', 'Server error. Please try again.'));
			} else {
				toast.error(tx('Tidak dapat terhubung ke server', 'Cannot connect to server'));
			}
		}
	}

	function generateRecoveryKey() {
		const alphabet = 'ABCDEFGHJKLMNPQRSTUVWXYZ23456789';
		const random = crypto.getRandomValues(new Uint8Array(30));
		const characters = Array.from(random, (value) => alphabet[value & 31]);
		recoveryKey = characters.reduce(
			(key, character, index) => key + (index > 0 && index % 5 === 0 ? '-' : '') + character,
			''
		);
	}

	function skipCloud() {
		cloud = {
			...cloud,
			enabled: false,
			nodeName: '',
			endpoint: '',
			region: '',
			bucket: '',
			accessKey: '',
			secretKey: '',
			capacityCapGb: null
		};
		currentStep = 4;
	}

	function onContinueHome() {
		successOpen = false;
		goto('/drive/home');
	}

	// localStorage key
	const STORAGE_KEY = 'cubycloud_setup_draft';

	// Save form state to localStorage
	function saveToLocalStorage() {
		if (!browser) return;
		try {
			const draft = {
				currentStep,
				admin: {
					username: admin.username,
					email: admin.email,
					// NEVER save password to localStorage
					password: '',
					confirmPassword: ''
				},
				nodes,
				strategy,
				cloud: {
					...cloud,
					// NEVER save credentials to localStorage
					accessKey: '',
					secretKey: ''
				},
				timestamp: Date.now()
			};
			localStorage.setItem(STORAGE_KEY, JSON.stringify(draft));
		} catch (err) {
			console.error(tx('Gagal menyimpan draf:', 'Failed to save draft:'), err);
		}
	}

	// Restore form state from localStorage
	function restoreFromLocalStorage() {
		if (!browser) return false;
		try {
			const saved = localStorage.getItem(STORAGE_KEY);
			if (!saved) return false;

			const draft = JSON.parse(saved);

			// Expire after 24 hours
			const age = Date.now() - (draft.timestamp || 0);
			if (age > 24 * 60 * 60 * 1000) {
				localStorage.removeItem(STORAGE_KEY);
				return false;
			}

			// Restore state
			if (draft.currentStep) currentStep = draft.currentStep;
			if (draft.admin) {
				admin.username = draft.admin.username || '';
				admin.email = draft.admin.email || '';
			}
			if (draft.nodes) nodes = draft.nodes;
			if (draft.strategy) strategy = draft.strategy;
			if (draft.cloud) {
				cloud.enabled = draft.cloud.enabled || false;
				cloud.provider = draft.cloud.provider || 'R2';
				cloud.endpoint = draft.cloud.endpoint || '';
				cloud.bucket = draft.cloud.bucket || '';
			}

			toast.success(
				tx('Draf dipulihkan dari sesi sebelumnya', 'Draft restored from previous session')
			);
			return true;
		} catch (err) {
			console.error(tx('Gagal memulihkan draf:', 'Failed to restore draft:'), err);
			return false;
		}
	}

	// Clear localStorage after successful setup
	function clearLocalStorage() {
		if (!browser) return;
		try {
			localStorage.removeItem(STORAGE_KEY);
		} catch (err) {
			console.error(tx('Gagal menghapus draf:', 'Failed to clear draft:'), err);
		}
	}

	// Auto-save on state change
	$effect(() => {
		// Watch all form fields
		void admin.username;
		void admin.email;
		void nodes;
		void strategy;
		void cloud.enabled;
		void cloud.provider;
		void cloud.endpoint;
		void cloud.bucket;
		void currentStep;

		// Save after initial load
		if (!loadingNodes) {
			saveToLocalStorage();
		}
	});

	onMount(async () => {
		// Try restore first
		const restored = restoreFromLocalStorage();

		// Then scan nodes (if not restored or to refresh capacity)
		try {
			const data = await scanStorageNodes();
			const scannedNodes = data.nodes.map((n, i) => ({
				id: `node-${i}`,
				name: n.path.split('/').pop() || n.path,
				path: n.path,
				kind: (n.kind === 'SSD' || n.kind === 'HDD' || n.kind === 'UNKNOWN'
					? n.kind
					: 'UNKNOWN') as 'SSD' | 'HDD' | 'UNKNOWN',
				totalGb: Math.round(n.capacity_bytes / 1024 ** 3),
				freeGb: Math.round(n.free_bytes / 1024 ** 3),
				enabled: true,
				usedBytes: 0,
				capacityCapGb: Math.round(n.capacity_bytes / 1024 ** 3)
			}));

			// Merge: keep restored enabled state if paths match
			if (restored && nodes.length > 0) {
				scannedNodes.forEach((scanned) => {
					const existing = nodes.find((n) => n.path === scanned.path);
					if (existing) {
						scanned.enabled = existing.enabled;
						scanned.capacityCapGb = existing.capacityCapGb;
					}
				});
			}
			nodes = scannedNodes;
		} catch {
			toast.error(tx('Gagal memindai node penyimpanan', 'Failed to scan storage nodes'));
		} finally {
			loadingNodes = false;
		}
	});
</script>

<div class="flex flex-col gap-6 md:flex-row md:items-start">
	<SetupStepper {currentStep} onSelectStep={goToPreviousStep} />

	<div class="min-w-0 flex-1 space-y-4">
		<div>
			<h1 class="text-2xl leading-tight font-bold text-main">
				{tx('Setup CubyCloud', 'CubyCloud Setup')}
			</h1>
			<p class="mt-1 text-sm text-muted">
				{locale.language === 'id'
					? SETUP_STEPS[currentStep - 1]?.description
					: [
							'Create the first administrator account.',
							'Choose local storage nodes.',
							'Connect cloud storage.',
							'Choose a file placement strategy.',
							'Save your recovery key.'
						][currentStep - 1]}
			</p>
		</div>

		{#if currentStep === 1}
			<StepAdmin bind:admin />
		{:else if currentStep === 2}
			<StepStorage bind:nodes bind:devices loading={loadingNodes} />
		{:else if currentStep === 3}
			<StepCloud bind:cloud noLocalNodes={nodes.filter((n) => n.enabled).length === 0} />
		{:else if currentStep === 4}
			<StepStrategy bind:strategy />
		{:else}
			<StepRecovery bind:recoveryKey bind:downloaded={recoveryDownloaded} />
			{#if !adminValid || !cloudValid}
				<Alert.Root>
					<Info class="size-4 stroke-2" />
					<Alert.Title>{tx('Data belum lengkap', 'Incomplete information')}</Alert.Title>
					<Alert.Description>
						{!adminValid
							? tx(
									'Lengkapi dulu data akun (nama pengguna, email, kata sandi) di Langkah 1.',
									'Complete the account details (username, email, password) in Step 1 first.'
								)
							: tx(
									'Lengkapi dulu kredensial cloud di Langkah 3.',
									'Complete the cloud credentials in Step 3 first.'
								)}
					</Alert.Description>
				</Alert.Root>
			{/if}
		{/if}

		<div class="rounded-xl border border-subtle bg-surface px-4 py-4 md:px-5">
			<SetupStepNav
				canBack={currentStep > 1}
				{canNext}
				isLast={currentStep === 5}
				showSkip={currentStep === 3}
				nextLabel={tx('Selesaikan Setup & Masuk ke CubyCloud', 'Complete Setup & Enter CubyCloud')}
				onBack={goBack}
				onNext={goNext}
				onSkip={skipCloud}
			/>
		</div>
	</div>
</div>

<SetupSuccessDialog
	bind:open={successOpen}
	username={admin.username}
	nodeCount={activeNodeCount}
	{strategy}
	{recoveryKey}
	onContinue={onContinueHome}
/>
