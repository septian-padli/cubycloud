<script lang="ts">
	import {
		SETUP_STEPS,
		type AdminForm,
		type CloudForm,
		type PlacementStrategy,
		type SetupStepId
	} from '$lib/types/setup.js';
	import {
		cloneMockDevices,
		cloneMockNodes,
		generateMockRecoveryKey
	} from '$lib/mocks/setup.js';
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
	let homeNotice = $state(false);

	let admin = $state<AdminForm>({
		username: '',
		email: '',
		password: '',
		confirmPassword: ''
	});
	let nodes = $state(cloneMockNodes());
	let devices = $state(cloneMockDevices());
	let strategy = $state<PlacementStrategy>('FILL_FIRST');
	let cloud = $state<CloudForm>({
		enabled: false,
		provider: 'R2',
		endpoint: '',
		bucket: '',
		accessKey: '',
		secretKey: ''
	});
	let recoveryKey = $state(generateMockRecoveryKey());
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
			homeNotice = false;
		}
	}

	function goNext() {
		if (!canNext) return;
		if (currentStep < 5) {
			currentStep = (currentStep + 1) as SetupStepId;
			return;
		}
		successOpen = true;
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
		homeNotice = true;
		// /drive/home belum tersedia — tampilkan notice inline (UI-only).
	}
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

		{#if homeNotice}
			<Alert.Root class="border-subtle bg-surface">
				<Info class="size-4 stroke-2 text-secondary-accent" />
				<Alert.Title class="text-main">Beranda belum tersedia</Alert.Title>
				<Alert.Description>
					Modul Drive (/drive/home) akan tersedia setelah diimplementasi. Setup UI sudah selesai.
				</Alert.Description>
			</Alert.Root>
		{/if}

		{#if currentStep === 1}
			<StepAdmin bind:admin />
		{:else if currentStep === 2}
			<StepStorage bind:nodes bind:devices />
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
	onContinue={onContinueHome}
/>
