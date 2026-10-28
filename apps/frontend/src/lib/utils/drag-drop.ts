export interface DroppedEntriesResult {
	files: File[];
	paths: string[]; // relative paths matching each file, e.g. "my-folder/sub/doc.pdf" or "loose-file.png"
	emptyFolders: string[]; // relative paths of empty directories, e.g. "my-folder/empty-sub"
}

/**
 * Common operating system junk files that should not be uploaded.
 */
const OS_JUNK_NAMES: Record<string, true> = {
	'.ds_store': true,
	'thumbs.db': true,
	'desktop.ini': true,
	'ehthumbs.db': true,
	'ehthumbs_vista.db': true,
	'.spotlight-v100': true,
	'.trashes': true,
	'.git': true,
	__macosx: true,
	'.svn': true,
	'.hg': true
};

/**
 * Checks if a file or directory name is an OS-generated junk file.
 */
export function isOsJunkFile(name: string): boolean {
	if (!name) return true;
	const lower = name.toLowerCase();
	if (OS_JUNK_NAMES[lower]) {
		return true;
	}
	// macOS AppleDouble / resource fork files (e.g. ._filename)
	if (name.startsWith('._')) {
		return true;
	}
	return false;
}

/**
 * Reads all entries from a FileSystemDirectoryReader by invoking readEntries in a loop
 * until an empty batch is returned (browsers return at most 100 entries per batch).
 */
function readAllDirectoryEntries(reader: FileSystemDirectoryReader): Promise<FileSystemEntry[]> {
	const { promise, resolve } = Promise.withResolvers<FileSystemEntry[]>();
	const allEntries: FileSystemEntry[] = [];
	const MAX_BATCHES = 10000; // safety ceiling against non-compliant infinite loop mocks
	let batchCount = 0;

	function readNextBatch() {
		try {
			reader.readEntries(
				(entries) => {
					if (!entries || entries.length === 0 || batchCount++ >= MAX_BATCHES) {
						resolve(allEntries);
					} else {
						allEntries.push(...entries);
						readNextBatch();
					}
				},
				(error) => {
					console.warn('Error reading directory entries batch:', error);
					resolve(allEntries);
				}
			);
		} catch (err) {
			console.warn('Exception in readEntries call:', err);
			resolve(allEntries);
		}
	}

	readNextBatch();
	return promise;
}

/**
 * Reads a File object from a FileSystemFileEntry.
 * Returns null if reading fails or error callback is invoked.
 */
function readFileEntry(fileEntry: FileSystemFileEntry): Promise<File | null> {
	const { promise, resolve } = Promise.withResolvers<File | null>();
	try {
		fileEntry.file(
			(file) => resolve(file),
			(error) => {
				console.warn(`Error reading file entry "${fileEntry.name}":`, error);
				resolve(null);
			}
		);
	} catch (err) {
		console.warn(`Exception reading file entry "${fileEntry.name}":`, err);
		resolve(null);
	}
	return promise;
}

/**
 * Extracts dropped files, relative paths, and empty directories from a DataTransfer object.
 *
 * Supports directory traversal recursively via webkitGetAsEntry() with 100-item batch looping.
 * Filters out OS junk files (.DS_Store, Thumbs.db, desktop.ini, ._*).
 * Falls back to dataTransfer.files if items/entries cannot be resolved.
 * Never throws errors / promise rejections (resolves valid entries found).
 */
export async function extractDroppedEntries(
	dataTransfer: DataTransfer
): Promise<DroppedEntriesResult> {
	const files: File[] = [];
	const paths: string[] = [];
	const emptyFolders: string[] = [];

	try {
		if (!dataTransfer) {
			return { files, paths, emptyFolders };
		}

		let hasEntrySupport = false;
		const topEntries: FileSystemEntry[] = [];
		const directFiles: { file: File; path: string }[] = [];

		if (dataTransfer.items && dataTransfer.items.length > 0) {
			for (let i = 0; i < dataTransfer.items.length; i++) {
				const item = dataTransfer.items[i];
				if (item.kind !== 'file') continue;

				let entry: FileSystemEntry | null = null;
				try {
					if (typeof item.webkitGetAsEntry === 'function') {
						entry = item.webkitGetAsEntry();
					}
				} catch {
					entry = null;
				}

				if (entry) {
					hasEntrySupport = true;
					topEntries.push(entry);
				} else {
					try {
						const file = item.getAsFile();
						if (file && !isOsJunkFile(file.name)) {
							directFiles.push({ file, path: file.name });
						}
					} catch {
						// ignore error getting file
					}
				}
			}
		}

		// Fallback to dataTransfer.files if webkitGetAsEntry is unsupported or no entries were resolved
		if (!hasEntrySupport) {
			const fallbackFiles: File[] = [];
			const fallbackPaths: string[] = [];
			if (dataTransfer.files && dataTransfer.files.length > 0) {
				for (let i = 0; i < dataTransfer.files.length; i++) {
					const file = dataTransfer.files[i];
					if (file && !isOsJunkFile(file.name)) {
						fallbackFiles.push(file);
						fallbackPaths.push(file.name);
					}
				}
			}
			return {
				files: fallbackFiles,
				paths: fallbackPaths,
				emptyFolders: []
			};
		}

		/**
		 * Recursively traverses a directory entry.
		 */
		async function traverseDirectory(
			dirEntry: FileSystemDirectoryEntry,
			currentPath: string
		): Promise<void> {
			const reader = dirEntry.createReader();
			const entries = await readAllDirectoryEntries(reader);
			const nonJunkEntries = entries.filter((e) => !isOsJunkFile(e.name));

			if (nonJunkEntries.length === 0) {
				emptyFolders.push(currentPath);
				return;
			}

			for (const child of nonJunkEntries) {
				const childPath = `${currentPath}/${child.name}`;
				if (child.isDirectory) {
					// Known FileSystemEntry type refinement
					const dirChild = child as FileSystemDirectoryEntry;
					await traverseDirectory(dirChild, childPath);
				} else if (child.isFile) {
					// Known FileSystemEntry type refinement
					const fileChild = child as FileSystemFileEntry;
					const file = await readFileEntry(fileChild);
					if (file) {
						files.push(file);
						paths.push(childPath);
					}
				}
			}
		}

		// Process resolved top-level entries
		for (const entry of topEntries) {
			if (isOsJunkFile(entry.name)) continue;

			if (entry.isDirectory) {
				// Known FileSystemEntry type refinement
				const dirEntry = entry as FileSystemDirectoryEntry;
				await traverseDirectory(dirEntry, entry.name);
			} else if (entry.isFile) {
				// Known FileSystemEntry type refinement
				const fileEntry = entry as FileSystemFileEntry;
				const file = await readFileEntry(fileEntry);
				if (file) {
					files.push(file);
					paths.push(entry.name);
				}
			}
		}

		// Append any direct files resolved when an individual item didn't return an entry
		for (const { file, path } of directFiles) {
			files.push(file);
			paths.push(path);
		}
	} catch (err) {
		console.warn('Unhandled exception in extractDroppedEntries:', err);
	}

	return { files, paths, emptyFolders };
}
