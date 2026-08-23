<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { toast } from 'svelte-sonner';
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
	let recoveryKey = $state('');
	let recoveryAcknowledged = $state(false);

	const activeNodeCount = $derived(nodes.filter((n) => n.enabled).length);

	const step1Valid = $derived(
		admin.username.trim().length > 0 &&
			admin.email.trim().length > 0 &&
			admin.password.length >= 8 &&
			admin.password === admin.confirmPassword
	);
	const step2Valid = $derived(activeNodeCount >= 1);
	const step3Valid = $derived(Boolean(strategy));
	const step4Valid = $derived(
		!cloud.enabled ||
			(cloud.endpoint.trim().length > 0 &&
				cloud.bucket.trim().length > 0 &&
				cloud.accessKey.trim().length > 0 &&
				cloud.secretKey.trim().length > 0)
	);
	const step5Valid = $derived(recoveryAcknowledged);

	const canNext = $derived(
		currentStep === 1
			? step1Valid
			: currentStep === 2
				? step2Valid
				: currentStep === 3
					? step3Valid
					: currentStep === 4
						? step4Valid
						: step5Valid
	);

	function goBack() {
		if (currentStep > 1) {
			currentStep = (currentStep - 1) as SetupStepId;
		}
	}

	async function goNext() {
		if (!canNext) return;
		if (currentStep < 5) {
			currentStep = (currentStep + 1) as SetupStepId;
			return;
		}
		// Step 5: submit
		try {
			const req: CompleteSetupRequest = {
				admin: {
					username: admin.username,
					email: admin.email,
					password: admin.password
				},
				storage_nodes: nodes
					.filter((n) => n.enabled)
					.map((n) => ({
						path: n.path,
						capacity_bytes: n.totalGb * 1e9,
						enabled: true
					})),
				placement_strategy: strategy
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
			const res = await completeSetup(req);
			recoveryKey = res.recovery_key;
			successOpen = true;
		} catch (err: any) {
			if (err.status === 409) {
				toast.error('Setup already completed');
			} else if (err.status === 400) {
				toast.error(err.message || 'Validation error');
			} else if (err.status === 500) {
				toast.error('Server error. Please try again.');
			} else {
				toast.error('Cannot connect to server');
			}
		}
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

	onMount(async () => {
		try {
			const data = await scanStorageNodes();
			nodes = data.nodes.map((n, i) => ({
				id: `node-${i}`,
				name: n.path.split('/').pop() || n.path,
				path: n.path,
				kind: 'SSD' as const,
				totalGb: Math.round(n.capacity_bytes / 1e9),
				freeGb: Math.round(n.free_bytes / 1e9),
				enabled: true,
				capacityCapGb: Math.round(n.capacity_bytes / 1e9)
			}));
		} catch {
			toast.error('Failed to scan storage nodes');
		} finally {
			loadingNodes = false;
		}
	});
</script>

<div class="flex flex-col gap-6 md:flex-row md:items-start">
	<SetupStepper {currentStep} />

	<div class="min-w-0 flex-1 space-y-4">
		<div>
			<h1 class="text-2xl font-bold leading-tight text-main">Setup CubyCloud</h1>
			<p class="mt-1 text-sm text-muted">
				{SETUP_STEPS[currentStep - 1]?.description}
			</p>
		</div>


		{#if currentStep === 1}
			<StepAdmin bind:admin />
		{:else if currentStep === 2}
			<StepStorage bind:nodes bind:devices loading={loadingNodes} />
		{:else if currentStep === 3}
			<StepStrategy bind:strategy />
		{:else if currentStep === 4}
			<StepCloud bind:cloud />
		{:else}
			<StepRecovery {recoveryKey} bind:acknowledged={recoveryAcknowledged} />
		{/if}

		<div class="rounded-xl border border-subtle bg-surface px-4 py-4 md:px-5">
			<SetupStepNav
				canBack={currentStep > 1}
				{canNext}
				isLast={currentStep === 5}
				showSkip={currentStep === 4}
				nextLabel="Selesaikan Setup & Masuk ke CubyCloud"
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
