<script lang="ts">
	import type { AddonConfig } from '$lib/types/admin.js';
	import * as Card from '$lib/components/ui/card/index.js';
	import { Switch } from '$lib/components/ui/switch/index.js';

	let {
		addon = $bindable(),
		onToggle
	}: {
		addon: AddonConfig;
		onToggle?: (enabled: boolean) => void;
	} = $props();

	function handleCheckedChange(value: boolean) {
		addon = { ...addon, enabled: value };
		onToggle?.(value);
	}
</script>

<Card.Root class="rounded-xl border border-subtle bg-surface shadow-none">
	<Card.Content class="relative flex flex-col gap-3">
		<div class="flex items-start justify-between gap-3">
			<div class="min-w-0 space-y-1 pe-2">
				<p class="text-sm font-medium text-main">{addon.name}</p>
				<p class="text-xs text-muted">{addon.description}</p>
			</div>
			<Switch
				checked={addon.enabled}
				aria-label={`Aktifkan ${addon.name}`}
				onCheckedChange={handleCheckedChange}
				class="shrink-0"
			/>
		</div>
	</Card.Content>
</Card.Root>
