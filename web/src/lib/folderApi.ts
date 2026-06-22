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

export function renameFolder(id: string, name: string): Promise<unknown> {
  return api.patch(`/folders/${id}`, { name });
}

export function moveFolder(id: string, parentId: string | null): Promise<unknown> {
  return api.patch(`/folders/${id}`, { move: { parentId } });
}

export function deleteFolder(id: string): Promise<unknown> {
  return api.del(`/folders/${id}`);
}

export function moveItem(
  kind: "document" | "signature",
  id: string,
  folderId: string | null,
): Promise<unknown> {
  return api.patch(`/${kind}s/${id}`, { move: { folderId } });
}
