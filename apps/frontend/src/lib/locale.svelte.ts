import { authAPI } from '$lib/api/auth.js';
import { messages } from './locale-messages.js';

export type Language = 'en' | 'id';
let language = $state<Language>('en');
let accountId: string | null = null;
let pendingLanguage = $state<Language | null>(null);
let initialized = false;
let accountRevision = 0;
const keyFor = (id: string) => `cubycloud-language:${id}`;
const DATE_FORMAT_OPTIONS: Intl.DateTimeFormatOptions = {
	day: 'numeric',
	month: 'short',
	year: 'numeric'
};
const DATE_FORMATTERS: Record<Language, Intl.DateTimeFormat> = {
	en: new Intl.DateTimeFormat('en', DATE_FORMAT_OPTIONS),
	id: new Intl.DateTimeFormat('id-ID', DATE_FORMAT_OPTIONS)
};

function browserLanguage(): Language {
	return typeof navigator !== 'undefined' && navigator.language.toLowerCase().startsWith('id')
		? 'id'
		: 'en';
}

export const locale = {
	get language() {
		return language;
	},
	async init() {
		if (initialized || typeof window === 'undefined') return;
		initialized = true;
		language = pendingLanguage ?? browserLanguage();
		const revision = accountRevision;
		const { data } = await authAPI.getMe();
		if (revision === accountRevision) this.setAccount(data?.id ?? null);
	},
	setAccount(id: string | null) {
		const previousAccountId = accountId;
		accountId = id;
		accountRevision += 1;
		if (!id) {
			if (previousAccountId) {
				pendingLanguage = null;
				language = browserLanguage();
			} else {
				language = pendingLanguage ?? browserLanguage();
			}
			return;
		}
		const saved = localStorage.getItem(keyFor(id));
		if (saved === 'en' || saved === 'id') {
			language = saved;
			pendingLanguage = null;
		} else if (pendingLanguage) {
			language = pendingLanguage;
			localStorage.setItem(keyFor(id), pendingLanguage);
			pendingLanguage = null;
		} else {
			language = browserLanguage();
		}
	},
	set(value: Language) {
		language = value;
		if (accountId) localStorage.setItem(keyFor(accountId), value);
		else pendingLanguage = value;
	},
	t(key: string) {
		return (messages[language] as Record<string, string>)[key] ?? key;
	},
	formatDate(value: string) {
		const timestamp = Date.parse(value);
		if (!Number.isFinite(timestamp)) return value;
		return DATE_FORMATTERS[language].format(timestamp);
	}
};

