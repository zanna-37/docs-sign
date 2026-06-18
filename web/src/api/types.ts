export interface User {
  id: string;
  username: string;
  isAdmin: boolean;
  mustChangePassword: boolean;
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

// PlacementInput is sent to the server: top-left origin, PDF points, clockwise rotation.
export interface PlacementInput {
  signatureId: string;
  page: number;
  x: number;
  y: number;
  w: number;
  h: number;
  rotation: number;
}
