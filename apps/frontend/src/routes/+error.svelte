<script lang="ts">
	import { page } from '$app/state';
	import ErrorPage from '$lib/components/error/error-page.svelte';
</script>

<ErrorPage status={page.status} message={page.error?.message} />
