import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { api, errMessage } from "../api/client";
import type { Folder, Signature } from "../api/types";
import { Button, Card, ErrorText, Spinner } from "../components/ui";
import { SignatureImage } from "../components/SignatureImage";
import { Dropzone } from "../components/Dropzone";
import { UploadProgress } from "../components/UploadProgress";
import { useDialog } from "../components/Dialog";
import { FolderPlusIcon, FolderUploadIcon, TrashIcon } from "../components/icons";
import { Breadcrumb } from "../components/folders/Breadcrumb";
import { SubfolderList } from "../components/folders/SubfolderList";
import { MoveDialog } from "../components/folders/MoveDialog";
import { ConflictDialog } from "../components/folders/ConflictDialog";
import { formatBytes, formatDate } from "../lib/format";
import { checkerBackground } from "../lib/checker";
import { useFolders } from "../lib/useFolders";
import { useUploads } from "../lib/useUploads";
import { entriesFromInput } from "../lib/uploads";
import {
  createFolder,
  deleteFolder,
  moveFolder,
  moveItem,
  renameFolder,
} from "../lib/folderApi";
import { setDragItem, type DragItem } from "../lib/dragItem";

const checker = checkerBackground(16);

// isPng accepts a file that is a PNG by MIME type or by extension; matches the server's check.
const isPng = (file: File) =>
  file.type === "image/png" || /\.png$/i.test(file.name);

// The directory-picker attributes are non-standard and missing from React's input typings.
const directoryInputProps = { webkitdirectory: "", directory: "" } as Record<string, string>;

type MoveTarget =
  | { type: "folder"; id: string; name: string }
  | { type: "signature"; id: string; name: string };

export function SignaturesPage() {
  const { t } = useTranslation();
  const dialog = useDialog();
  const { folderId, setFolderId, path, folders, loadFolders } =
    useFolders("signature");
  const [items, setItems] = useState<Signature[] | null>(null);
  const [error, setError] = useState("");
  const [moving, setMoving] = useState<MoveTarget | null>(null);
  const fileRef = useRef<HTMLInputElement>(null);
  const folderUploadRef = useRef<HTMLInputElement>(null);

  const folderQuery = folderId ? `?folder=${encodeURIComponent(folderId)}` : "";
  const pathLabel = "/" + path.map((f) => f.name).join("/");

  const reload = useCallback(async () => {
    try {
      await loadFolders();
      const res = await api.get<{ signatures: Signature[] }>(
        `/signatures${folderQuery}`,
      );
      setItems(res.signatures ?? []);
    } catch (err) {
      setError(errMessage(err, t("common.failedLoad")));
    }
  }, [folderQuery, loadFolders, t]);

  useEffect(() => {
    setItems(null);
    void reload();
  }, [reload]);

  // --- uploads (with progress + folder-structure recreation) ---

  const uploads = useUploads({
    kind: "signature",
    folderId,
    pathLabel,
    validate: (file) => (isPng(file) ? null : t("signatures.notPng", { name: file.name })),
    reload,
    setError,
  });

  const onPick = (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = e.target.files;
    if (files && files.length) uploads.uploadEntries(entriesFromInput(files));
    e.target.value = "";
  };

  // --- folder operations ---

  const newFolder = async () => {
    const name = await dialog.prompt({
      title: t("folders.newPrompt"),
      confirmLabel: t("common.create"),
    });
    if (!name) return;
    try {
      await createFolder("signature", folderId, name);
      await reload();
    } catch (err) {
      setError(errMessage(err, t("folders.createFailed")));
    }
  };

  const renameFolderItem = async (f: Folder) => {
    const name = await dialog.prompt({
      title: t("folders.renamePrompt"),
      defaultValue: f.name,
      confirmLabel: t("common.save"),
    });
    if (!name || name === f.name) return;
    try {
      await renameFolder(f.id, name);
      await reload();
    } catch (err) {
      setError(errMessage(err, t("common.renameFailed")));
    }
  };

  const deleteFolderItem = async (f: Folder) => {
    if (
      !(await dialog.confirm({
        title: t("folders.confirmDelete", { name: f.name }),
        confirmLabel: t("common.delete"),
        danger: true,
      }))
    )
      return;
    try {
      await deleteFolder(f.id);
      await reload();
    } catch (err) {
      setError(errMessage(err, t("common.deleteFailed")));
    }
  };

  // --- moves ---

  const handleDrop = async (targetFolderId: string | null, item: DragItem) => {
    setError("");
    try {
      if (item.kind === "folder") {
        if (item.id === targetFolderId) return;
        await moveFolder(item.id, targetFolderId);
      } else {
        await moveItem("signature", item.id, targetFolderId);
      }
      await reload();
    } catch (err) {
      setError(errMessage(err, t("folders.moveFailed")));
    }
  };

  const applyMove = async (target: string | null) => {
    if (!moving) return;
    try {
      if (moving.type === "folder") await moveFolder(moving.id, target);
      else await moveItem("signature", moving.id, target);
      await reload();
    } catch (err) {
      setError(errMessage(err, t("folders.moveFailed")));
    } finally {
      setMoving(null);
    }
  };

  // --- signature operations ---

  const rename = async (s: Signature) => {
    const name = await dialog.prompt({
      title: t("signatures.renamePrompt"),
      defaultValue: s.name,
      confirmLabel: t("common.save"),
    });
    if (!name || name === s.name) return;
    try {
      await api.patch(`/signatures/${s.id}`, { name });
      await reload();
    } catch (err) {
      setError(errMessage(err, t("common.renameFailed")));
    }
  };

  const remove = async (s: Signature) => {
    if (
      !(await dialog.confirm({
        title: t("signatures.confirmDelete", { name: s.name }),
        confirmLabel: t("common.delete"),
        danger: true,
      }))
    )
      return;
    try {
      await api.del(`/signatures/${s.id}`);
      await reload();
    } catch (err) {
      setError(errMessage(err, t("common.deleteFailed")));
    }
  };

  const isEmpty = items !== null && items.length === 0 && folders.length === 0;

  return (
    <Dropzone onUpload={uploads.uploadEntries} label={t("signatures.drop")}>
      <div className="space-y-5">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h1 className="text-xl font-semibold text-gray-900">
              {t("signatures.title")}
            </h1>
            <p className="text-sm text-gray-500">{t("signatures.subtitle")}</p>
          </div>
          <div className="flex items-center gap-2">
            <Button variant="secondary" onClick={newFolder}>
              <FolderPlusIcon className="h-4 w-4" />
              {t("folders.new")}
            </Button>
            <input
              ref={fileRef}
              type="file"
              accept="image/png"
              multiple
              className="hidden"
              onChange={onPick}
            />
            <input
              ref={folderUploadRef}
              type="file"
              accept="image/png"
              multiple
              className="hidden"
              onChange={onPick}
              {...directoryInputProps}
            />
            <Button
              variant="secondary"
              onClick={() => folderUploadRef.current?.click()}
              disabled={uploads.busy}
            >
              <FolderUploadIcon className="h-4 w-4" />
              {t("uploads.uploadFolder")}
            </Button>
            <Button onClick={() => fileRef.current?.click()} disabled={uploads.busy}>
              {uploads.busy ? t("signatures.uploading") : t("signatures.upload")}
            </Button>
          </div>
        </div>

        <Breadcrumb path={path} onNavigate={setFolderId} onDropInto={handleDrop} />

        {uploads.progress && <UploadProgress progress={uploads.progress} />}

        <ErrorText>{error}</ErrorText>

        <SubfolderList
          folders={folders}
          onOpen={setFolderId}
          onRename={renameFolderItem}
          onMove={(f) => setMoving({ type: "folder", id: f.id, name: f.name })}
          onDelete={deleteFolderItem}
          onDropInto={handleDrop}
        />

        {items === null ? (
          <Spinner />
        ) : isEmpty ? (
          <Card className="p-10 text-center text-sm text-gray-500">
            {t("signatures.empty")}
          </Card>
        ) : items.length === 0 ? null : (
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {items.map((s) => (
              <div
                key={s.id}
                draggable
                onDragStart={(e) =>
                  setDragItem(e, { kind: "signature", id: s.id, name: s.name })
                }
              >
                <Card className="overflow-hidden">
                  <div
                    className="flex h-36 items-center justify-center p-4"
                    style={checker}
                  >
                    <SignatureImage
                      id={s.id}
                      alt={s.name}
                      className="max-h-full max-w-full object-contain"
                    />
                  </div>
                  <div className="flex items-center justify-between gap-2 border-t border-gray-100 p-3">
                    <div className="min-w-0">
                      <p className="truncate text-sm font-medium text-gray-800">
                        {s.name}
                      </p>
                      <p className="text-xs text-gray-400">
                        {s.width}×{s.height} · {formatBytes(s.byteSize)} ·{" "}
                        {formatDate(s.createdAt)}
                      </p>
                    </div>
                    <div className="flex shrink-0 gap-1">
                      <Button variant="secondary" onClick={() => rename(s)}>
                        {t("common.rename")}
                      </Button>
                      <Button
                        variant="secondary"
                        onClick={() =>
                          setMoving({ type: "signature", id: s.id, name: s.name })
                        }
                      >
                        {t("folders.move")}
                      </Button>
                      <Button
                        variant="secondary"
                        className="px-2"
                        title={t("common.delete")}
                        aria-label={t("common.delete")}
                        onClick={() => remove(s)}
                      >
                        <TrashIcon className="h-4 w-4 text-red-600" />
                      </Button>
                    </div>
                  </div>
                </Card>
              </div>
            ))}
          </div>
        )}
      </div>

      {moving && (
        <MoveDialog
          kind="signature"
          title={t("folders.moveTitleNamed", { name: moving.name })}
          excludeFolderId={moving.type === "folder" ? moving.id : undefined}
          onMove={applyMove}
          onClose={() => setMoving(null)}
        />
      )}

      {uploads.pending && (
        <ConflictDialog
          conflicts={uploads.pending.conflicts}
          onCancel={uploads.cancelPending}
          onResolve={uploads.resolvePending}
        />
      )}
    </Dropzone>
  );
}
