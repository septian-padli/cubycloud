<script lang="ts">
	import { getDriveChrome } from '$lib/drive-context.svelte.js';
	import ErrorPage from '$lib/components/error/error-page.svelte';

	let { children } = $props();

	const chrome = getDriveChrome();
	const isSuperAdmin = $derived(chrome?.user?.role === 'super_admin');
</script>

{#if isSuperAdmin}
	{@render children()}
{:else}
	<div class="flex-1 overflow-y-auto" data-testid="admin-forbidden-page">
		<span class="sr-only">403 Akses ditolak</span>
		<ErrorPage status={403} showHeader={false} homeHref="/drive/my-storage" />
	</div>
{/if}
