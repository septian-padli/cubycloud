// Polyfill Svelte 5 runes for standalone bun test runner
type GlobalScope = typeof globalThis & Record<string, unknown>;
const g = globalThis as GlobalScope;

if (typeof g.$state === 'undefined') {
	const stateFn = <T>(initialValue: T): T => initialValue;
	stateFn.raw = <T>(initialValue: T): T => initialValue;
	stateFn.snapshot = <T>(val: T): T => val;
	g.$state = stateFn;
}

if (typeof g.$derived === 'undefined') {
	const derivedFn = <T>(val: T): T => val;
	derivedFn.by = <T>(fn: () => T): T => fn();
	g.$derived = derivedFn;
}

if (typeof g.$props === 'undefined') {
	g.$props = () => ({});
}
