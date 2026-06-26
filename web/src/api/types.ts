export interface VersionInfo {
  version: string;
  commit: string;
  // HTTPS base URL of the source repo, used to link the version to its GitHub release.
  repoURL: string;
}

export interface User {
  id: string;
  username: string;
  isAdmin: boolean;
  mustChangePassword: boolean;
  language: string;
}

export type FolderKind = "document" | "signature";

export interface Folder {
  id: string;
  name: string;
  kind: FolderKind;
  parentId?: string;
  createdAt: string;
}

export interface FolderListResponse {
  folders: Folder[];
  path: Folder[];
}

export interface Signature {
  id: string;
  name: string;
  folderId?: string;
  width: number;
  height: number;
  byteSize: number;
  createdAt: string;
}

export interface DocumentItem {
  id: string;
  name: string;
  folderId?: string;
  pageCount: number;
  byteSize: number;
  // MIME type detected at upload. Any file type may be stored.
  contentType: string;
  // True only for a parseable PDF — the only kind that can be opened in the signing editor.
  signable: boolean;
  createdAt: string;
}

export interface ExportItem {
  id: string;
  name: string;
  documentId?: string;
  pageCount: number;
  byteSize: number;
  createdAt: string;
}

export interface AdminUser {
  id: string;
  username: string;
  isAdmin: boolean;
  status: string;
  mustChangePassword: boolean;
  createdAt: string;
}

export type TrashKind = "folder" | "document" | "signature" | "export";

// TrashEvent is one delete action — the top-level entry shown in the trash.
export interface TrashEvent {
  eventId: string;
  rootKind: TrashKind;
  rootId: string;
  label: string;
  byteSize: number;
  itemCount: number;
  deletedAt: string;
  purgeAt: string;
}

// TrashEntry is a child seen while walking into a trashed folder.
export interface TrashEntry {
  kind: TrashKind;
  id: string;
  name: string;
  byteSize: number;
  isFolder: boolean;
}

// RestoreConflict is a file whose name is already taken at its restore destination.
export interface RestoreConflict {
  kind: "document" | "signature";
  id: string;
  name: string;
  destPath: string;
}

export type ConflictAction = "override" | "skip" | "rename";
export interface ConflictResolution {
  action: ConflictAction;
  newName?: string;
}

// PlacementInput is sent to the server: top-left origin, PDF points, clockwise rotation.
// Either a stored signatureId or an inline base64 PNG (rasterized text box).
export interface PlacementInput {
  signatureId?: string;
  imageData?: string;
  page: number;
  x: number;
  y: number;
  w: number;
  h: number;
  rotation: number;
}
