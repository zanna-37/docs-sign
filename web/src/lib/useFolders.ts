import { useCallback, useState } from "react";
import { api } from "../api/client";
import type { Folder, FolderKind, FolderListResponse } from "../api/types";

// useFolders tracks the current folder within a per-kind tree and loads its subfolders and
// breadcrumb. Item loading stays with the page, which calls loadFolders as part of its reload
// and re-runs that reload whenever folderId changes.
export function useFolders(kind: FolderKind) {
  const [folderId, setFolderId] = useState<string | null>(null);
  const [path, setPath] = useState<Folder[]>([]);
  const [folders, setFolders] = useState<Folder[]>([]);

  const loadFolders = useCallback(async () => {
    const q = folderId
      ? `?kind=${kind}&parent=${encodeURIComponent(folderId)}`
      : `?kind=${kind}`;
    const res = await api.get<FolderListResponse>(`/folders${q}`);
    setFolders(res.folders ?? []);
    setPath(res.path ?? []);
  }, [kind, folderId]);

  return { kind, folderId, setFolderId, path, folders, loadFolders };
}
