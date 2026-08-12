<script lang="ts">
	import * as Card from '$lib/components/ui/card/index.js';
	import { Button } from '$lib/components/ui/button/index.js';

	let {
		loginHref
	}: {
		loginHref: string;
	} = $props();
</script>

<div class="flex flex-1 items-center justify-center py-8">
	<Card.Root class="w-full max-w-md rounded-xl border-subtle bg-surface shadow-none">
		<Card.Header>
			<Card.Title class="text-lg font-semibold text-main">Perlu masuk</Card.Title>
			<Card.Description class="text-sm text-muted">
				Tautan ini restricted. Masuk ke akun CubyCloud untuk melihat isinya.
			</Card.Description>
		</Card.Header>
		<Card.Content class="flex flex-col gap-3">
			<Button href={loginHref}>Masuk</Button>
			<Button href="/" variant="secondary">Kembali ke beranda</Button>
		</Card.Content>
	</Card.Root>
</div>
