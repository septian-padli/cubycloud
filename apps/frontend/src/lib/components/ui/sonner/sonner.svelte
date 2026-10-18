<script lang="ts">
	import { Toaster as Sonner, type ToasterProps as SonnerProps } from 'svelte-sonner';
	import Loader2Icon from '@lucide/svelte/icons/loader-2';
	import CircleCheckIcon from '@lucide/svelte/icons/circle-check';
	import OctagonXIcon from '@lucide/svelte/icons/octagon-x';
	import InfoIcon from '@lucide/svelte/icons/info';
	import TriangleAlertIcon from '@lucide/svelte/icons/triangle-alert';
	import { theme } from '$lib/theme.svelte.js';

	let { class: className, ...restProps }: SonnerProps = $props();

	const sonnerTheme = $derived(theme.isDark ? 'dark' : 'light');
</script>

<Sonner
	theme={sonnerTheme}
	class={['toaster group', className].filter(Boolean).join(' ')}
	style="--normal-bg: var(--surface-card); --normal-border: var(--primary); --normal-text: var(--text-main); --success-bg: var(--surface-card); --success-border: var(--primary); --success-text: var(--primary); --error-bg: var(--surface-card); --error-border: var(--danger); --error-text: var(--danger); --warning-bg: var(--surface-card); --warning-border: var(--secondary-accent); --warning-text: var(--secondary-accent); --info-bg: var(--surface-card); --info-border: var(--primary); --info-text: var(--text-main);"
	{...restProps}
>
	{#snippet loadingIcon()}
		<Loader2Icon class="size-4 animate-spin" />
	{/snippet}
	{#snippet successIcon()}
		<CircleCheckIcon class="size-4" />
	{/snippet}
	{#snippet errorIcon()}
		<OctagonXIcon class="size-4" />
	{/snippet}
	{#snippet infoIcon()}
		<InfoIcon class="size-4" />
	{/snippet}
	{#snippet warningIcon()}
		<TriangleAlertIcon class="size-4" />
	{/snippet}
</Sonner>

<style>
	:global([data-sonner-toaster] [data-sonner-toast]) {
		background: var(--surface-card) !important;
		box-shadow: none !important;
		border-radius: var(--radius, 0.5rem) !important;
		border-width: 1px !important;
		border-style: solid !important;
		font-family: var(--font-sans) !important;
		font-size: 0.875rem !important;
		line-height: 1.25rem !important;
	}

	:global([data-sonner-toaster] [data-sonner-toast][data-type='success']) {
		background: var(--surface-card) !important;
		border-color: var(--primary) !important;
		color: var(--primary) !important;
	}

	:global([data-sonner-toaster] [data-sonner-toast][data-type='success'] [data-close-button]) {
		background: var(--surface-card) !important;
		border-color: var(--primary) !important;
		color: var(--primary) !important;
	}

	:global([data-sonner-toaster] [data-sonner-toast][data-type='error']) {
		background: var(--surface-card) !important;
		border-color: var(--danger) !important;
		color: var(--danger) !important;
	}

	:global([data-sonner-toaster] [data-sonner-toast][data-type='error'] [data-close-button]) {
		background: var(--surface-card) !important;
		border-color: var(--danger) !important;
		color: var(--danger) !important;
	}

	:global([data-sonner-toaster] [data-sonner-toast][data-type='info']),
	:global([data-sonner-toaster] [data-sonner-toast][data-type='default']) {
		background: var(--surface-card) !important;
		border-color: var(--primary) !important;
		color: var(--text-main) !important;
	}

	:global([data-sonner-toaster] [data-sonner-toast][data-type='warning']) {
		background: var(--surface-card) !important;
		border-color: var(--secondary-accent) !important;
		color: var(--secondary-accent) !important;
	}

	:global([data-sonner-toaster] [data-sonner-toast] [data-title]) {
		font-weight: 500 !important;
		color: inherit !important;
	}

	:global([data-sonner-toaster] [data-sonner-toast] [data-description]) {
		font-size: 0.75rem !important;
		opacity: 0.9 !important;
		color: inherit !important;
	}

	:global([data-sonner-toaster] [data-sonner-toast] [data-close-button]:hover) {
		background: var(--surface-card) !important;
		opacity: 0.8 !important;
	}
</style>
