import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { api, errMessage } from "../../api/client";
import type { Folder, FolderKind, FolderListResponse } from "../../api/types";
import { Button, ErrorText, Modal, Spinner } from "../ui";
import { FolderIcon } from "../icons";
import { Breadcrumb } from "./Breadcrumb";

// MoveDialog is the accessible / touch fallback to drag-and-drop: navigate the folder tree and
// pick a destination. The folder being moved (excludeFolderId) is disabled, which also blocks
// descending into its own subtree.
export function MoveDialog({
  kind,
  title,
  excludeFolderId,
  onMove,
  onClose,
}: Readonly<{
  kind: FolderKind;
  title: string;
  excludeFolderId?: string;
  onMove: (folderId: string | null) => void;
  onClose: () => void;
}>) {
  const { t } = useTranslation();
  const [folderId, setFolderId] = useState<string | null>(null);
  const [path, setPath] = useState<Folder[]>([]);
  const [folders, setFolders] = useState<Folder[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    const q = folderId
      ? `?kind=${kind}&parent=${encodeURIComponent(folderId)}`
      : `?kind=${kind}`;
    api
      .get<FolderListResponse>(`/folders${q}`)
      .then((res) => {
        if (cancelled) return;
        setFolders(res.folders ?? []);
        setPath(res.path ?? []);
      })
      .catch((err) => !cancelled && setError(errMessage(err, t("common.failedLoad"))))
      .finally(() => !cancelled && setLoading(false));
    return () => {
      cancelled = true;
    };
  }, [kind, folderId, t]);

  return (
    <Modal open title={title} onClose={onClose}>
      <div className="space-y-4">
        <Breadcrumb path={path} onNavigate={setFolderId} />
        <div className="max-h-60 min-h-24 divide-y divide-gray-100 overflow-auto rounded-lg border border-gray-200">
          {loading ? (
            <div className="flex justify-center p-6">
              <Spinner />
            </div>
          ) : folders.length === 0 ? (
            <p className="p-4 text-center text-sm text-gray-400">
              {t("folders.noSubfolders")}
            </p>
          ) : (
            folders.map((f) => {
              const disabled = f.id === excludeFolderId;
              return (
                <button
                  key={f.id}
                  type="button"
                  disabled={disabled}
                  onClick={() => setFolderId(f.id)}
                  className="flex w-full items-center gap-2 p-2.5 text-left text-sm text-gray-700 hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-40"
                >
                  <FolderIcon className="h-4 w-4 shrink-0 text-blue-500" />
                  <span className="truncate">{f.name}</span>
                </button>
              );
            })
          )}
        </div>
        <ErrorText>{error}</ErrorText>
        <div className="flex items-center justify-between gap-2">
          <span className="text-xs text-gray-400">
            {t("folders.moveHereHint")}
          </span>
          <div className="flex gap-2">
            <Button variant="secondary" onClick={onClose}>
              {t("common.cancel")}
            </Button>
            <Button onClick={() => onMove(folderId)}>
              {t("folders.moveHere")}
            </Button>
          </div>
        </div>
      </div>
    </Modal>
  );
}
