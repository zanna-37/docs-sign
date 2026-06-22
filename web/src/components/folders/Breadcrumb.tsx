import { useTranslation } from "react-i18next";
import type { Folder } from "../../api/types";
import { getDragItem, type DragItem } from "../../lib/dragItem";

export function Breadcrumb({
  path,
  onNavigate,
  onDropInto,
}: Readonly<{
  path: Folder[];
  onNavigate: (folderId: string | null) => void;
  // When provided, each crumb becomes a drop target so dragged items can move up the tree.
  onDropInto?: (folderId: string | null, item: DragItem) => void;
}>) {
  const { t } = useTranslation();

  const crumb = (label: string, folderId: string | null, key: string) => (
    <button
      key={key}
      type="button"
      onClick={() => onNavigate(folderId)}
      onDragOver={onDropInto ? (e) => e.preventDefault() : undefined}
      onDrop={
        onDropInto
          ? (e) => {
              e.preventDefault();
              const item = getDragItem(e);
              if (item) onDropInto(folderId, item);
            }
          : undefined
      }
      className="rounded px-1.5 py-0.5 font-medium text-gray-600 hover:bg-gray-100 hover:text-gray-900"
    >
      {label}
    </button>
  );

  return (
    <nav className="flex flex-wrap items-center gap-0.5 text-sm">
      {crumb(t("folders.root"), null, "root")}
      {path.map((f) => (
        <span key={f.id} className="flex items-center gap-0.5">
          <span className="text-gray-300">/</span>
          {crumb(f.name, f.id, f.id)}
        </span>
      ))}
    </nav>
  );
}
