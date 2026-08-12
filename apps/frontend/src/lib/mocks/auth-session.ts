const KEY = 'cubycloud-logged-in';

export function isMockLoggedIn(): boolean {
	try {
		return sessionStorage.getItem(KEY) === '1';
	} catch {
		return false;
	}
}

export function setMockLoggedIn(value: boolean): void {
	try {
		if (value) sessionStorage.setItem(KEY, '1');
		else sessionStorage.removeItem(KEY);
	} catch {
		// ponytail: ignore quota / private-mode failures in mock session
	}
}
