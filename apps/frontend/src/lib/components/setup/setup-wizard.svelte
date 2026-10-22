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
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { Switch } from '$lib/components/ui/switch/index.js';
	import SetupStepper from './setup-stepper.svelte';
	import SetupStepNav from './setup-step-nav.svelte';
	import SetupSuccessDialog from './setup-success-dialog.svelte';
	import StepAdmin from './steps/step-admin.svelte';
	import StepStorage from './steps/step-storage.svelte';
	import StepStrategy from './steps/step-strategy.svelte';
	import StepCloud from './steps/step-cloud.svelte';
	import StepRecovery from './steps/step-recovery.svelte';
	import { Copy, Info } from '@lucide/svelte';
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
		provider: 'R2',
		endpoint: '',
		bucket: '',
		accessKey: '',
		secretKey: ''
	});
	let googleEnabled = $state(false);
	let googleClientId = $state('');
	let googleClientSecret = $state('');
	let showGoogleHelp = $state(false);
	const googleRedirectUri = $derived(browser ? `${window.location.origin}/api/v1/auth/google/callback` : '');
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
	const cloudValid = $derived(
		!cloud.enabled ||
			(cloud.endpoint.trim() !== '' &&
				cloud.bucket.trim() !== '' &&
				cloud.accessKey.trim() !== '' &&
				cloud.secretKey.trim() !== '')
	);
	const googleValid = $derived(
		!googleEnabled || (googleClientId.trim().length > 0 && googleClientSecret.trim().length > 0)
	);

	const canNext = $derived.by(() => {
		switch (currentStep) {
			case 1:
				return adminValid;
			case 2:
				return googleValid;
			case 3:
				return true;
			case 4:
				return cloudValid && (cloud.enabled || activeNodeCount > 0);
			case 5:
				return strategy !== null;
			case 6:
				return recoveryDownloaded && adminValid && cloudValid && strategy !== null && googleValid;
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

		if (currentStep === 5) {
			generateRecoveryKey();
			currentStep = 6;
			return;
		}

		if (currentStep < 6) {
			currentStep = (currentStep + 1) as SetupStepId;
			return;
		}

		// Step 6: submit with pre-generated recovery key
		try {
			const req: CompleteSetupRequest = {
				admin: {
					username: admin.username,
					email: admin.email,
					password: admin.password,
					recovery_key: recoveryKey
				},
				storage_nodes: nodes
					.filter((n) => n.enabled)
					.map((n) => ({ path: n.path, capacity_bytes: n.capacityCapGb * 1e9, enabled: true })),
				placement_strategy: strategy,
				google_auth: {
					enabled: googleEnabled,
					client_id: googleEnabled ? googleClientId.trim() : '',
					client_secret: googleEnabled ? googleClientSecret : ''
				}
			};
			if (cloud.enabled) {
				req.cloud = {
					enabled: true,
					provider: cloud.provider,
					endpoint: cloud.endpoint,
					bucket: cloud.bucket,
					access_key: cloud.accessKey,
					secret_key: cloud.secretKey
				};
			}
			await completeSetup(req);
			clearLocalStorage();
			goto('/drive/home');
		} catch (err: any) {
			if (err.status === 409) toast.error(tx('Setup sudah selesai', 'Setup already completed'));
			else if (err.status === 400) toast.error(err.message || tx('Kesalahan validasi', 'Validation error'));
			else if (err.status === 500) toast.error(tx('Kesalahan server. Silakan coba lagi.', 'Server error. Please try again.'));
			else toast.error(tx('Tidak dapat terhubung ke server', 'Cannot connect to server'));
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
			endpoint: '',
			bucket: '',
			accessKey: '',
			secretKey: ''
		};
		currentStep = 5;
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
				admin: { username: admin.username, email: admin.email, password: '', confirmPassword: '' },
				nodes,
				strategy,
				googleEnabled,
				googleClientId,
				cloud: { ...cloud, accessKey: '', secretKey: '' },
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
			googleEnabled = draft.googleEnabled || false;
			googleClientId = draft.googleClientId || '';
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
		void googleEnabled;
		void googleClientId;
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
				kind: (n.kind === 'SSD' || n.kind === 'HDD' ? n.kind : 'HDD') as 'SSD' | 'HDD',
				totalGb: Math.round(n.capacity_bytes / 1e9),
				freeGb: Math.round(n.free_bytes / 1e9),
				enabled: true,
				usedBytes: 0,
				capacityCapGb: Math.round(n.capacity_bytes / 1e9)
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
							'Configure optional Google Sign-In.',
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
			<div class="space-y-5 rounded-xl border border-subtle bg-surface p-5">
				<div class="flex items-center justify-between gap-4">
					<div>
						<h2 class="font-semibold text-main">{tx('Google Authentication (Opsional)', 'Google Authentication (Optional)')}</h2>
						<p class="text-sm text-muted">{tx('Aktifkan Google Sign-In untuk akun yang telah terhubung. Kredensial ini hanya diperlukan untuk mengaktifkan fitur Sign-In dengan Google, bukan untuk menggunakan CubyCloud.', 'Enable Google Sign-In for accounts that have been linked. These credentials are only required to enable Sign in with Google, not to use CubyCloud.')}</p>
					</div>
					<Switch bind:checked={googleEnabled} aria-label={tx('Aktifkan Google Sign-In', 'Enable Google Sign-In')} />
				</div>
				{#if googleEnabled}
					<div class="space-y-2">
						<Label for="google-client-id">Client ID</Label>
						<Input id="google-client-id" bind:value={googleClientId} autocomplete="off" />
						{#if !googleClientId.trim()}<p class="text-xs text-destructive">{tx('Client ID wajib diisi.', 'Client ID is required.')}</p>{/if}
					</div>
					<div class="space-y-2">
						<Label for="google-client-secret">Client Secret</Label>
						<Input id="google-client-secret" type="password" bind:value={googleClientSecret} autocomplete="new-password" />
						{#if !googleClientSecret}<p class="text-xs text-destructive">{tx('Client Secret wajib diisi.', 'Client Secret is required.')}</p>{/if}
					</div>
					<div class="rounded-lg border border-subtle bg-app p-3 text-sm">
						<p class="font-medium text-main">{tx('URI Pengalihan Resmi', 'Authorized Redirect URI')}</p>
						<p class="break-all text-muted">{googleRedirectUri}</p>
						<Button variant="secondary" size="sm" class="mt-2 gap-2" onclick={() => void navigator.clipboard.writeText(googleRedirectUri)}><Copy class="size-4" />{tx('Salin', 'Copy')}</Button>
					</div>
					<Button variant="secondary" onclick={() => (showGoogleHelp = true)}><Info class="mr-2 size-4" />{tx('Cara mendapatkan kredensial Google OAuth', 'How to get Google OAuth credentials')}</Button>
				{/if}
			</div>
		{:else if currentStep === 3}
			<StepStorage bind:nodes bind:devices loading={loadingNodes} />
		{:else if currentStep === 4}
			<StepCloud bind:cloud noLocalNodes={nodes.filter((n) => n.enabled).length === 0} />
		{:else if currentStep === 5}
			<StepStrategy bind:strategy />
		{:else}
			<StepRecovery bind:recoveryKey bind:downloaded={recoveryDownloaded} />
		{/if}

		<div class="rounded-xl border border-subtle bg-surface px-4 py-4 md:px-5">
			<SetupStepNav canBack={currentStep > 1} {canNext} isLast={currentStep === 6} showSkip={currentStep === 4} nextLabel={tx('Selesaikan Setup & Masuk ke CubyCloud', 'Complete Setup & Enter CubyCloud')} onBack={goBack} onNext={goNext} onSkip={skipCloud} />
		</div>
	</div>
</div>

	<Dialog.Root bind:open={showGoogleHelp}>
		<Dialog.Content>
			<Dialog.Header><Dialog.Title>{tx('Cara mendapatkan kredensial Google OAuth', 'How to get Google OAuth credentials')}</Dialog.Title></Dialog.Header>
		<Dialog.Description>
			<ol class="list-decimal space-y-2 pl-5">
				<li>{tx('Buka Google Cloud Console dan pilih atau buat project.', 'Open Google Cloud Console and select or create a project.')}</li>
				<li>{tx('Buka menu "APIs & Services" > "OAuth consent screen" (atau "Google Auth Platform"). Konfigurasikan consent screen dengan nama aplikasi dan domain Anda.', 'Go to "APIs & Services" > "OAuth consent screen" (or "Google Auth Platform"). Configure the consent screen with your application name and domain.')}</li>
				<li>{tx('Buat OAuth Client ID tipe "Web application" di halaman Credentials.', 'Create an OAuth Client ID of type "Web application" on the Credentials page.')}</li>
				<li>{tx('Tambahkan Authorized Redirect URI yang ditampilkan di atas ke konfigurasi client.', 'Add the Authorized Redirect URI shown above to the client configuration.')}</li>
				<li>{tx('Salin Client ID dan Client Secret yang dihasilkan.', 'Copy the generated Client ID and Client Secret.')}</li>
				<li>{tx('Jika aplikasi masih dalam mode "Testing", tambahkan alamat email pengguna yang akan mencoba Google Sign-In ke daftar Test Users di consent screen.', 'If your app is still in "Testing" mode, add the email addresses of users who will try Google Sign-In to the Test Users list on the consent screen.')}</li>
			</ol>
		</Dialog.Description>
		</Dialog.Content>
	</Dialog.Root>

<SetupSuccessDialog
	bind:open={successOpen}
	username={admin.username}
	nodeCount={activeNodeCount}
	{strategy}
	{recoveryKey}
	onContinue={onContinueHome}
/>
