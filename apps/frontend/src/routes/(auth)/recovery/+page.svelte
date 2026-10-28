<script lang="ts">
	import RecoveryForm from '$lib/components/auth/recovery-form.svelte';
	import { locale } from '$lib/locale.svelte.js';
</script>

<svelte:head>
	<title>{locale.language === 'id' ? 'Pulihkan Akses' : 'Recover Access'} — CubyCloud</title>
	<meta
		name="description"
		content={locale.language === 'id'
			? 'Atur ulang kata sandi Super Admin CubyCloud dengan Emergency Recovery Key.'
			: 'Reset your CubyCloud Super Admin password with an Emergency Recovery Key.'}
	/>
</svelte:head>

<RecoveryForm />
