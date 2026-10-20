declare module 'prismjs/components/*.js';

declare module '*.mjs?url' {
	const url: string;
	export default url;
}
