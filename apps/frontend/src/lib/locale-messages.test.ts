import { expect, test } from 'bun:test';
import { messages } from './locale-messages.js';

test('English and Indonesian locale keys match', () => {
	expect(Object.keys(messages.id).sort()).toEqual(Object.keys(messages.en).sort());
});
