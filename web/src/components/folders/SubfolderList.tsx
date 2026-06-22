import { type DragEvent } from "react";
import { useTranslation } from "react-i18next";
import type { Folder } from "../../api/types";
import { getDragItem, setDragItem, type DragItem } from "../../lib/dragItem";
import { FolderIcon, TrashIcon } from "../icons";

// SubfolderList renders the folders inside the current folder as a grid of draggable cards that
// double as drop targets (drop an item or folder onto one to move it inside).
export function SubfolderList({
  folders,
  onOpen,
  onRename,
  onMove,
  onDelete,
  onDropInto,
}: Readonly<{
  folders: Folder[];
  onOpen: (id: string) => void;
  onRename: (f: Folder) => void;
  onMove: (f: Folder) => void;
  onDelete: (f: Folder) => void;
  onDropInto: (folderId: string, item: DragItem) => void;
}>) {
  const { t } = useTranslation();
  if (folders.length === 0) return null;

  const onDragStart = (e: DragEvent, f: Folder) =>
    setDragItem(e, { kind: "folder", id: f.id, name: f.name });

  return (
    <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 lg:grid-cols-3">
      {folders.map((f) => (
        <div
          key={f.id}
          draggable
          onDragStart={(e) => onDragStart(e, f)}
          onDragOver={(e) => e.preventDefault()}
          onDrop={(e) => {
            e.preventDefault();
            const item = getDragItem(e);
            if (item && item.id !== f.id) onDropInto(f.id, item);
          }}
          className="group flex items-center justify-between gap-2 rounded-lg border border-gray-200 bg-white px-3 py-2.5 shadow-sm transition hover:border-blue-300 hover:shadow"
        >
          <button
            type="button"
            onClick={() => onOpen(f.id)}
            className="flex min-w-0 flex-1 items-center gap-2 text-left"
          >
            <FolderIcon className="h-5 w-5 shrink-0 text-blue-500" />
            <span className="truncate text-sm font-medium text-gray-800">
              {f.name}
            </span>
          </button>
          <div className="flex shrink-0 items-center gap-0.5">
            <button
              type="button"
              onClick={() => onRename(f)}
              className="rounded px-2 py-1 text-xs font-medium text-gray-500 hover:bg-gray-100 hover:text-gray-700"
            >
              {t("common.rename")}
            </button>
            <button
              type="button"
              onClick={() => onMove(f)}
              className="rounded px-2 py-1 text-xs font-medium text-gray-500 hover:bg-gray-100 hover:text-gray-700"
            >
              {t("folders.move")}
            </button>
            <button
              type="button"
              onClick={() => onDelete(f)}
              title={t("common.delete")}
              aria-label={t("common.delete")}
              className="rounded p-1.5 hover:bg-gray-100"
            >
              <TrashIcon className="h-4 w-4 text-red-600" />
            </button>
          </div>
        </div>
      ))}
    </div>
  );
}
