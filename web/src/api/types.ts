export interface User {
  id: string;
  username: string;
  isAdmin: boolean;
  mustChangePassword: boolean;
  language: string;
}

export interface Signature {
  id: string;
  name: string;
  width: number;
  height: number;
  byteSize: number;
  createdAt: string;
}

export interface DocumentItem {
  id: string;
  name: string;
  pageCount: number;
  byteSize: number;
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

export interface TrashItem {
  id: string;
  kind: "signature" | "document" | "export";
  name: string;
  byteSize: number;
  deletedAt: string;
  purgeAt: string;
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
