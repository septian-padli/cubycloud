<script lang="ts">
	import * as Card from '$lib/components/ui/card/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { locale } from '$lib/locale.svelte.js';

	let {
		loginHref
	}: {
		loginHref: string;
	} = $props();
</script>

<div class="flex flex-1 items-center justify-center py-8">
	<Card.Root class="w-full max-w-md rounded-xl border-subtle bg-surface shadow-none">
		<Card.Header>
			<Card.Title class="text-lg font-semibold text-main">
				{locale.language === 'id' ? 'Perlu masuk' : 'Sign in required'}
			</Card.Title>
			<Card.Description class="text-sm text-muted">
				{locale.language === 'id'
					? 'Tautan ini dibatasi. Masuk ke akun CubyCloud untuk melihat isinya.'
					: 'This link is restricted. Sign in to your CubyCloud account to view its contents.'}
			</Card.Description>
		</Card.Header>
		<Card.Content class="flex flex-col gap-3">
			<Button href={loginHref}>{locale.language === 'id' ? 'Masuk' : 'Sign in'}</Button>
			<Button href="/" variant="secondary">
				{locale.language === 'id' ? 'Kembali ke beranda' : 'Back to home'}
			</Button>
		</Card.Content>
	</Card.Root>
</div>
