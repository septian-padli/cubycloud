<script lang="ts">
	import AuthSplitShell from '$lib/components/auth/auth-split-shell.svelte';

	let { children } = $props();
</script>

<AuthSplitShell>
	{@render children()}
</AuthSplitShell>
