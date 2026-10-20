import { afterEach, expect, test } from 'bun:test';
import { myStorageAPI } from './my-storage.js';

const originalDocument = globalThis.document;
const originalWindow = globalThis.window;
const originalFetch = globalThis.fetch;

afterEach(() => {
	globalThis.document = originalDocument;
	globalThis.window = originalWindow;
	globalThis.fetch = originalFetch;
});

test('ZIP without native file picker submits repeated ids to the browser download manager, not JS chunks', async () => {
	const fields: { name: string; value: string }[] = [];
	let submitted: { action: string; method: string; target: string; enctype: string } | undefined;
	const frame = {
		isConnected: false, name: '', title: '', hidden: false,
		contentDocument: { URL: 'about:blank', body: { textContent: '' } },
		onload: null as (() => void) | null
	};
	const form = {
		action: '', method: '', target: '', enctype: '', style: { display: '' },
		append(input: { name: string; value: string }) { fields.push({ name: input.name, value: input.value }); },
		submit() {
			submitted = { action: this.action, method: this.method, target: this.target, enctype: this.enctype };
		},
		remove() {}
	};
	globalThis.document = {
		createElement(tag: string) {
			if (tag === 'iframe') return frame;
			if (tag === 'form') return form;
			if (tag === 'input') return { name: '', value: '', type: '' };
			throw new Error(`Unexpected element: ${tag}`);
		},
		body: { append(element: unknown) { if (element === frame) frame.isConnected = true; } }
	} as unknown as Document;
	globalThis.window = {} as Window & typeof globalThis;
	globalThis.fetch = (async () => { throw new Error('ZIP must not be fetched into JS memory'); }) as unknown as typeof fetch;

	const result = await myStorageAPI.downloadBatchZip(['one', 'two']);
	expect(result).toBe('started');
	expect(submitted).toEqual({
		action: '/api/v1/drive/my-storage/batch/download-zip',
		method: 'POST', target: 'cubycloud-zip-download', enctype: 'application/x-www-form-urlencoded'
	});
	expect(fields).toEqual([{ name: 'ids', value: 'one' }, { name: 'ids', value: 'two' }]);

	let errorMessage = '';
	myStorageAPI.downloadBatchZip(['one'], (message) => { errorMessage = message; });
	frame.contentDocument.URL = '/api/v1/drive/my-storage/batch/download-zip';
	frame.contentDocument.body.textContent = JSON.stringify({ error: 'ids is required' });
	(frame.onload as (() => void) | null)?.();
	expect(errorMessage).toBe('ids is required');
	frame.isConnected = false;
});

test('overlapping ZIP downloads keep each response and error callback bound to its own frame', () => {
	const frames: Array<{
		isConnected: boolean;
		name: string;
		contentDocument: { URL: string; body: { textContent: string } };
		onload: (() => void) | null;
		remove: () => void;
	}> = [];
	globalThis.document = {
		createElement(tag: string) {
			if (tag === 'iframe') {
				const frame = {
					isConnected: false,
					name: '',
					contentDocument: { URL: 'about:blank', body: { textContent: '' } },
					onload: null as (() => void) | null,
					remove() { this.isConnected = false; }
				};
				frames.push(frame);
				return frame;
			}
			if (tag === 'form') return {
				action: '', method: '', enctype: '', target: '', style: { display: '' },
				append() {}, submit() {}, remove() {}
			};
			if (tag === 'input') return { name: '', value: '', type: '' };
			throw new Error(`Unexpected element: ${tag}`);
		},
		body: { append(element: { isConnected?: boolean }) { element.isConnected = true; } }
	} as unknown as Document;

	const errorsA: string[] = [];
	const errorsB: string[] = [];
	myStorageAPI.downloadBatchZip(['one'], (message) => { errorsA.push(message); });
	myStorageAPI.downloadBatchZip(['two'], (message) => { errorsB.push(message); });
	expect(frames).toHaveLength(2);
	expect(frames[0].name).not.toBe(frames[1].name);

	frames[0].contentDocument.URL = '/api/v1/drive/my-storage/batch/download-zip';
	frames[0].contentDocument.body.textContent = JSON.stringify({ error: 'first download failed' });
	frames[0].onload?.();
	expect(errorsA).toEqual(['first download failed']);
	expect(errorsB).toEqual([]);

	frames[1].contentDocument.URL = '/api/v1/drive/my-storage/batch/download-zip';
	frames[1].contentDocument.body.textContent = JSON.stringify({ error: 'second download failed' });
	frames[1].onload?.();
	expect(errorsB).toEqual(['second download failed']);
});
