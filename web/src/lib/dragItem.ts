import { type DragEvent } from "react";

// DragItem is the payload carried while dragging a folder or file onto a folder/breadcrumb to
// move it. The browser only exposes dataTransfer contents on drop (not during dragover), so
// validation (e.g. preventing a folder drop into its own subtree) happens on drop / server-side.
export interface DragItem {
  kind: "folder" | "document" | "signature";
  id: string;
  name: string;
}

const MIME = "application/x-docs-sign-item";

export function setDragItem(e: DragEvent, item: DragItem): void {
  e.dataTransfer.setData(MIME, JSON.stringify(item));
  e.dataTransfer.setData("text/plain", item.name);
  e.dataTransfer.effectAllowed = "move";
}

export function getDragItem(e: DragEvent): DragItem | null {
  const raw = e.dataTransfer.getData(MIME);
  if (!raw) return null;
  try {
    return JSON.parse(raw) as DragItem;
  } catch {
    return null;
  }
}
