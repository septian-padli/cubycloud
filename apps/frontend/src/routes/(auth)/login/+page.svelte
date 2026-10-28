<script lang="ts">
	import LoginForm from '$lib/components/auth/login-form.svelte';
	import { locale } from '$lib/locale.svelte.js';
</script>

<svelte:head>
	<title>{locale.language === 'id' ? 'Masuk' : 'Sign in'} — CubyCloud</title>
	<meta
		name="description"
		content={locale.language === 'id'
			? 'Masuk ke CubyCloud dengan akun Anda.'
			: 'Sign in to CubyCloud with your account.'}
	/>
</svelte:head>

<LoginForm />
