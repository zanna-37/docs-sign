// Helpers that turn a file selection — whether from a drag-and-drop or an <input> — into a flat
// list of upload entries, preserving the directory each file came from so the structure can be
// recreated on the server.

// An upload candidate: the file plus the directory segments (relative to the dropped folder or
// the selected directory) it should be filed under. A loose file carries an empty dirs array.
export interface UploadEntry {
  file: File;
  dirs: string[];
}

// The slice of the FileSystem entries API we use. It is non-standard (webkit-prefixed) and not in
// the DOM typings, so we describe just the members we touch.
interface FSDirectoryReader {
  readEntries: (cb: (entries: FSEntry[]) => void, err: (e: unknown) => void) => void;
}

interface FSEntry {
  isFile: boolean;
  isDirectory: boolean;
  name: string;
  file?: (cb: (f: File) => void, err: (e: unknown) => void) => void;
  createReader?: () => FSDirectoryReader;
}

// readAllEntries drains a directory reader, which returns its children in batches until empty.
function readAllEntries(reader: FSDirectoryReader): Promise<FSEntry[]> {
  return new Promise((resolve, reject) => {
    const out: FSEntry[] = [];
    const pump = () =>
      reader.readEntries((batch) => {
        if (batch.length === 0) {
          resolve(out);
          return;
        }
        out.push(...batch);
        pump();
      }, reject);
    pump();
  });
}

// walkEntry collects every file beneath a filesystem entry, tagging each with the directory
// segments (relative to the dropped root) that contain it.
async function walkEntry(entry: FSEntry, dirs: string[], out: UploadEntry[]): Promise<void> {
  if (entry.isFile && entry.file) {
    const file = await new Promise<File>((resolve, reject) => entry.file!(resolve, reject));
    out.push({ file, dirs });
    return;
  }
  if (entry.isDirectory && entry.createReader) {
    const children = await readAllEntries(entry.createReader());
    for (const child of children) {
      await walkEntry(child, [...dirs, entry.name], out);
    }
  }
}

// entriesFromDataTransfer extracts upload entries from a drop, recreating any dropped directory
// structure. The DataTransferItemList is only valid synchronously, so each item's filesystem
// entry is captured before the first await.
export async function entriesFromDataTransfer(dt: DataTransfer): Promise<UploadEntry[]> {
  const roots: FSEntry[] = [];
  const looseFiles: File[] = [];
  for (const item of Array.from(dt.items || [])) {
    if (item.kind !== "file") continue;
    const getEntry = (item as DataTransferItem & {
      webkitGetAsEntry?: () => FSEntry | null;
    }).webkitGetAsEntry;
    const entry = getEntry?.call(item) ?? null;
    if (entry) {
      roots.push(entry);
    } else {
      const f = item.getAsFile();
      if (f) looseFiles.push(f);
    }
  }

  const out: UploadEntry[] = [];
  if (roots.length > 0) {
    for (const root of roots) await walkEntry(root, [], out);
  }
  // Fallback for browsers without the entries API: flat files only.
  if (out.length === 0 && looseFiles.length === 0) {
    for (const f of Array.from(dt.files || [])) out.push({ file: f, dirs: [] });
  } else {
    for (const f of looseFiles) out.push({ file: f, dirs: [] });
  }
  return out;
}

// entriesFromInput builds upload entries from an <input type="file"> selection. A directory
// picker (the webkitdirectory attribute) sets each file's webkitRelativePath to "dir/sub/name",
// from which the directory segments are recovered; a plain selection yields loose files.
export function entriesFromInput(files: FileList | File[]): UploadEntry[] {
  return Array.from(files).map((file) => {
    const rel = (file as File & { webkitRelativePath?: string }).webkitRelativePath;
    if (rel && rel.includes("/")) {
      const parts = rel.split("/");
      parts.pop(); // drop the filename, keep the directory chain
      return { file, dirs: parts };
    }
    return { file, dirs: [] };
  });
}
