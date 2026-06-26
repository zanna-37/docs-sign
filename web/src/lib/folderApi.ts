import { api } from "../api/client";
import type { Folder, FolderKind } from "../api/types";

// Thin wrappers over the folder + item-move endpoints, shared by the Documents and Signatures
// pages. A null folder id (or parent id) means the root of the tree.

export function createFolder(
  kind: FolderKind,
  parentId: string | null,
  name: string,
): Promise<Folder> {
  return api.post<Folder>("/folders", { kind, parentId: parentId ?? "", name });
}

// ensureFolderPath get-or-creates a chain of folders (named by path) under parentId and resolves
// to the leaf folder, reusing folders that already exist. Used to recreate an uploaded directory
// tree before filing its files.
export function ensureFolderPath(
  kind: FolderKind,
  parentId: string | null,
  path: string[],
): Promise<Folder> {
  return api.post<Folder>("/folders/ensure", {
    kind,
    parentId: parentId ?? "",
    path,
  });
}

export function renameFolder(id: string, name: string): Promise<unknown> {
  return api.patch(`/folders/${id}`, { name });
}

export function moveFolder(id: string, parentId: string | null): Promise<unknown> {
  return api.patch(`/folders/${id}`, { move: { parentId } });
}

export function deleteFolder(id: string): Promise<unknown> {
  return api.del(`/folders/${id}`);
}

// downloadFolder triggers a browser download of the folder's whole subtree as a ZIP archive that
// mirrors the folder structure. The endpoint streams an attachment, so a transient anchor click
// saves the file (sending the session cookie) without navigating away.
export function downloadFolder(id: string, name: string): void {
  const a = document.createElement("a");
  a.href = `/api/folders/${encodeURIComponent(id)}/download`;
  a.download = `${name}.zip`;
  document.body.appendChild(a);
  a.click();
  a.remove();
}

export function moveItem(
  kind: "document" | "signature",
  id: string,
  folderId: string | null,
): Promise<unknown> {
  return api.patch(`/${kind}s/${id}`, { move: { folderId } });
}
