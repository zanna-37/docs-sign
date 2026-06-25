import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { api, errMessage } from "../api/client";
import type { DocumentItem, ExportItem, Folder } from "../api/types";
import { Button, Card, ErrorText, Spinner } from "../components/ui";
import { Dropzone } from "../components/Dropzone";
import { useDialog } from "../components/Dialog";
import { ChevronIcon, FolderPlusIcon, TrashIcon } from "../components/icons";
import { Breadcrumb } from "../components/folders/Breadcrumb";
import { SubfolderList } from "../components/folders/SubfolderList";
import { MoveDialog } from "../components/folders/MoveDialog";
import {
  ConflictDialog,
  type ConflictItem,
} from "../components/folders/ConflictDialog";
import { formatBytes, formatDate } from "../lib/format";
import { useFolders } from "../lib/useFolders";
import {
  createFolder,
  deleteFolder,
  moveFolder,
  moveItem,
  renameFolder,
} from "../lib/folderApi";
import { setDragItem, type DragItem } from "../lib/dragItem";

const pdfName = (name: string) => (/\.pdf$/i.test(name) ? name : `${name}.pdf`);

type MoveTarget =
  | { type: "folder"; id: string; name: string }
  | { type: "document"; id: string; name: string };

export function DocumentsPage() {
  const navigate = useNavigate();
  const { t } = useTranslation();
  const dialog = useDialog();
  const { folderId, setFolderId, path, folders, loadFolders } =
    useFolders("document");
  const [items, setItems] = useState<DocumentItem[] | null>(null);
  const [exports, setExports] = useState<ExportItem[]>([]);
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [moving, setMoving] = useState<MoveTarget | null>(null);
  const [pendingUpload, setPendingUpload] = useState<{
    files: File[];
    conflicts: ConflictItem[];
  } | null>(null);
  const fileRef = useRef<HTMLInputElement>(null);

  const folderQuery = folderId ? `?folder=${encodeURIComponent(folderId)}` : "";
  const pathLabel = "/" + path.map((f) => f.name).join("/");

  const reload = useCallback(async () => {
    try {
      await loadFolders();
      const [docs, exp] = await Promise.all([
        api.get<{ documents: DocumentItem[] }>(`/documents${folderQuery}`),
        api.get<{ exports: ExportItem[] }>("/exports"),
      ]);
      setItems(docs.documents ?? []);
      setExports(exp.exports ?? []);
    } catch (err) {
      setError(errMessage(err, t("common.failedLoad")));
    }
  }, [folderQuery, loadFolders, t]);

  useEffect(() => {
    setItems(null);
    void reload();
  }, [reload]);

  const exportsByDoc = useMemo(() => {
    const m: Record<string, ExportItem[]> = {};
    for (const e of exports) {
      if (e.documentId) {
        m[e.documentId] ??= [];
        m[e.documentId].push(e);
      }
    }
    return m;
  }, [exports]);

  // --- uploads (with name-collision handling) ---

  const docUploadPath = (overwrite: boolean) => {
    const params = new URLSearchParams();
    if (folderId) params.set("folder", folderId);
    if (overwrite) params.set("overwrite", "true");
    const qs = params.toString();
    return `/documents${qs ? `?${qs}` : ""}`;
  };

  const doUpload = async (
    files: File[],
    resolutions: Record<string, { action: string; newName?: string }>,
  ) => {
    setError("");
    setBusy(true);
    try {
      for (const file of files) {
        const r = resolutions[file.name];
        if (r?.action === "skip") continue;
        const name = r?.action === "rename" && r.newName ? r.newName : file.name;
        await api.upload<DocumentItem>(
          docUploadPath(r?.action === "override"),
          file,
          name,
        );
      }
      await reload();
    } catch (err) {
      setError(errMessage(err, t("common.uploadFailed")));
    } finally {
      setBusy(false);
    }
  };

  const uploadFiles = (files: File[]) => {
    setError("");
    const pdfs: File[] = [];
    for (const file of files) {
      if (file.type === "application/pdf" || /\.pdf$/i.test(file.name)) {
        pdfs.push(file);
      } else {
        setError(t("documents.notPdf", { name: file.name }));
      }
    }
    if (pdfs.length === 0) return;
    const existing = new Set((items ?? []).map((d) => d.name));
    const conflicts = pdfs
      .filter((f) => existing.has(f.name))
      .map((f) => ({ id: f.name, name: f.name, destPath: pathLabel }));
    if (conflicts.length > 0) {
      setPendingUpload({ files: pdfs, conflicts });
    } else {
      void doUpload(pdfs, {});
    }
  };

  const onUpload = (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(e.target.files || []);
    e.target.value = "";
    if (files.length) uploadFiles(files);
  };

  // --- folder operations ---

  const newFolder = async () => {
    const name = await dialog.prompt({
      title: t("folders.newPrompt"),
      confirmLabel: t("common.create"),
    });
    if (!name) return;
    try {
      await createFolder("document", folderId, name);
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

  // --- moves (drag-and-drop + dialog) ---

  const handleDrop = async (targetFolderId: string | null, item: DragItem) => {
    setError("");
    try {
      if (item.kind === "folder") {
        if (item.id === targetFolderId) return;
        await moveFolder(item.id, targetFolderId);
      } else {
        await moveItem("document", item.id, targetFolderId);
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
      else await moveItem("document", moving.id, target);
      await reload();
    } catch (err) {
      setError(errMessage(err, t("folders.moveFailed")));
    } finally {
      setMoving(null);
    }
  };

  // --- document operations ---

  const rename = async (d: DocumentItem) => {
    const name = await dialog.prompt({
      title: t("documents.renamePrompt"),
      defaultValue: d.name,
      confirmLabel: t("common.save"),
    });
    if (!name || name === d.name) return;
    try {
      await api.patch(`/documents/${d.id}`, { name });
      await reload();
    } catch (err) {
      setError(errMessage(err, t("common.renameFailed")));
    }
  };

  const remove = async (d: DocumentItem) => {
    if (
      !(await dialog.confirm({
        title: t("documents.confirmDelete", { name: d.name }),
        confirmLabel: t("common.delete"),
        danger: true,
      }))
    )
      return;
    try {
      await api.del(`/documents/${d.id}`);
      await reload();
    } catch (err) {
      setError(errMessage(err, t("common.deleteFailed")));
    }
  };

  const removeExport = async (x: ExportItem) => {
    if (
      !(await dialog.confirm({
        title: t("documents.confirmDeleteExport", { name: x.name }),
        confirmLabel: t("common.delete"),
        danger: true,
      }))
    )
      return;
    try {
      await api.del(`/exports/${x.id}`);
      await reload();
    } catch (err) {
      setError(errMessage(err, t("common.deleteFailed")));
    }
  };

  const isEmpty = items !== null && items.length === 0 && folders.length === 0;

  return (
    <Dropzone onFiles={uploadFiles} label={t("documents.drop")}>
      <div className="space-y-5">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h1 className="text-xl font-semibold text-gray-900">
              {t("documents.title")}
            </h1>
            <p className="text-sm text-gray-500">{t("documents.subtitle")}</p>
          </div>
          <div className="flex items-center gap-2">
            <Button variant="secondary" onClick={newFolder}>
              <FolderPlusIcon className="h-4 w-4" />
              {t("folders.new")}
            </Button>
            <input
              ref={fileRef}
              type="file"
              accept="application/pdf"
              multiple
              className="hidden"
              onChange={onUpload}
            />
            <Button onClick={() => fileRef.current?.click()} disabled={busy}>
              {busy ? t("documents.uploading") : t("documents.upload")}
            </Button>
          </div>
        </div>

        <Breadcrumb path={path} onNavigate={setFolderId} onDropInto={handleDrop} />

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
            {t("documents.empty")}
          </Card>
        ) : items.length === 0 ? null : (
          <Card className="divide-y divide-gray-100">
            {items.map((d) => {
              const docExports = exportsByDoc[d.id] ?? [];
              const isOpen = expanded[d.id];
              return (
                <div
                  key={d.id}
                  draggable
                  onDragStart={(e) =>
                    setDragItem(e, { kind: "document", id: d.id, name: d.name })
                  }
                >
                  <div className="flex flex-wrap items-center justify-between gap-3 p-4">
                    <div className="min-w-0">
                      <p className="truncate font-medium text-gray-800">
                        {d.name}
                      </p>
                      <p className="text-xs text-gray-400">
                        {t("documents.page", { count: d.pageCount })} ·{" "}
                        {formatBytes(d.byteSize)} · {formatDate(d.createdAt)}
                      </p>
                    </div>
                    <div className="flex flex-wrap items-center gap-1">
                      <Button onClick={() => navigate(`/documents/${d.id}/sign`)}>
                        {t("documents.sign")}
                      </Button>
                      <a
                        href={`/api/documents/${d.id}/file`}
                        download={pdfName(d.name)}
                      >
                        <Button variant="secondary">
                          {t("common.download")}
                        </Button>
                      </a>
                      <Button variant="secondary" onClick={() => rename(d)}>
                        {t("common.rename")}
                      </Button>
                      <Button
                        variant="secondary"
                        onClick={() =>
                          setMoving({ type: "document", id: d.id, name: d.name })
                        }
                      >
                        {t("folders.move")}
                      </Button>
                      <Button
                        variant="secondary"
                        className="px-2"
                        title={t("common.delete")}
                        aria-label={t("common.delete")}
                        onClick={() => remove(d)}
                      >
                        <TrashIcon className="h-4 w-4 text-red-600" />
                      </Button>
                      <button
                        onClick={() =>
                          setExpanded((m) => ({ ...m, [d.id]: !m[d.id] }))
                        }
                        aria-expanded={isOpen}
                        className={`inline-flex items-center gap-1.5 rounded-lg border px-3 py-2 text-sm font-medium transition ${
                          isOpen
                            ? "border-blue-200 bg-blue-50 text-blue-700"
                            : "border-gray-300 bg-white text-gray-700 hover:bg-gray-50"
                        }`}
                      >
                        {t("documents.signed")}
                        <span
                          className={`rounded-full px-1.5 py-0.5 text-xs font-semibold ${
                            docExports.length
                              ? "bg-blue-100 text-blue-700"
                              : "bg-gray-100 text-gray-500"
                          }`}
                        >
                          {docExports.length}
                        </span>
                        <ChevronIcon
                          className={`h-4 w-4 transition-transform duration-150 ${
                            isOpen ? "rotate-180" : ""
                          }`}
                        />
                      </button>
                    </div>
                  </div>

                  {isOpen && (
                    <div className="border-t border-gray-100 bg-gray-50/70 px-4 py-3">
                      <p className="mb-2 px-1 text-xs font-semibold uppercase tracking-wide text-gray-400">
                        {t("documents.signedCopies")}
                      </p>
                      {docExports.length === 0 ? (
                        <div className="rounded-lg border border-dashed border-gray-200 bg-white px-4 py-6 text-center text-sm text-gray-400">
                          {t("documents.noSignedCopies")}
                        </div>
                      ) : (
                        <ul className="space-y-2">
                          {docExports.map((x) => (
                            <li
                              key={x.id}
                              className="flex items-center justify-between gap-3 rounded-lg border border-gray-200 bg-white px-3 py-2.5 shadow-sm transition hover:border-gray-300 hover:shadow"
                            >
                              <div className="min-w-0">
                                <p className="truncate text-sm font-medium text-gray-800">
                                  {x.name}
                                </p>
                                <p className="text-xs text-gray-400">
                                  {t("documents.page", { count: x.pageCount })} ·{" "}
                                  {formatBytes(x.byteSize)} ·{" "}
                                  {formatDate(x.createdAt)}
                                </p>
                              </div>
                              <div className="flex shrink-0 items-center gap-1">
                                <Button
                                  variant="secondary"
                                  onClick={() =>
                                    window.open(
                                      `/api/exports/${x.id}/file?inline=1`,
                                      "_blank",
                                      "noopener",
                                    )
                                  }
                                >
                                  {t("common.preview")}
                                </Button>
                                <a href={`/api/exports/${x.id}/file`} download>
                                  <Button variant="secondary">
                                    {t("common.download")}
                                  </Button>
                                </a>
                                <Button
                                  variant="secondary"
                                  className="px-2"
                                  title={t("common.delete")}
                                  aria-label={t("common.delete")}
                                  onClick={() => removeExport(x)}
                                >
                                  <TrashIcon className="h-4 w-4 text-red-600" />
                                </Button>
                              </div>
                            </li>
                          ))}
                        </ul>
                      )}
                    </div>
                  )}
                </div>
              );
            })}
          </Card>
        )}
      </div>

      {moving && (
        <MoveDialog
          kind="document"
          title={t("folders.moveTitleNamed", { name: moving.name })}
          excludeFolderId={moving.type === "folder" ? moving.id : undefined}
          onMove={applyMove}
          onClose={() => setMoving(null)}
        />
      )}

      {pendingUpload && (
        <ConflictDialog
          conflicts={pendingUpload.conflicts}
          onCancel={() => setPendingUpload(null)}
          onResolve={(res) => {
            const files = pendingUpload.files;
            setPendingUpload(null);
            void doUpload(files, res);
          }}
        />
      )}
    </Dropzone>
  );
}
