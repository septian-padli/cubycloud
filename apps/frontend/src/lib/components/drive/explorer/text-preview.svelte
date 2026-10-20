<script lang="ts">
	import { getPreviewText } from '$lib/api/preview.js';

	let { url, extension }: { url: string; extension: string } = $props();
	let text = $state('');
	let highlighted = $state('');
	let loading = $state(true);
	let error = $state('');

	$effect(() => {
		const source = url;
		const language = extension;
		const controller = new AbortController();
		text = '';
		highlighted = '';
		loading = true;
		error = '';
		void (async () => {
			try {
				const content = await getPreviewText(source, controller.signal);
				if (controller.signal.aborted) return;
				// Prism escapes untrusted source before producing markup; plain text stays in Svelte's escaped text node.
				if (['js', 'ts', 'json', 'css', 'html', 'sql', 'md'].includes(language)) {
					const Prism = (await import('prismjs')).default;
					if (language === 'js') await import('prismjs/components/prism-javascript.js');
					if (language === 'ts') await import('prismjs/components/prism-typescript.js');
					if (language === 'json') await import('prismjs/components/prism-json.js');
					if (language === 'css') await import('prismjs/components/prism-css.js');
					if (language === 'html') await import('prismjs/components/prism-markup.js');
					if (language === 'sql') await import('prismjs/components/prism-sql.js');
					if (language === 'md') await import('prismjs/components/prism-markdown.js');
					if (controller.signal.aborted) return;
					const grammarName = language === 'html' ? 'markup' : language === 'js' ? 'javascript' : language === 'ts' ? 'typescript' : language === 'md' ? 'markdown' : language;
					highlighted = Prism.highlight(content, Prism.languages[grammarName], grammarName);
				} else {
					text = content;
				}
				loading = false;
			} catch (cause) {
				if (!controller.signal.aborted) {
					error = cause instanceof Error ? cause.message : 'Gagal menampilkan teks';
					loading = false;
				}
			}
		})();
		return () => controller.abort();
	});
</script>

{#if loading}
	<p role="status" class="text-sm text-muted">Memuat teks…</p>
{:else if error}
	<p role="alert" class="text-sm text-muted">{error}</p>
{:else}
	<pre data-testid="text-preview" class="max-h-96 w-full overflow-auto rounded-lg bg-app p-4 text-left text-sm whitespace-pre-wrap break-words"><code>{#if highlighted}{@html highlighted}{:else}{text}{/if}</code></pre>
{/if}
