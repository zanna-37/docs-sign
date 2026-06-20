import { useEffect, useMemo, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { api, errMessage } from "../api/client";
import type { DocumentItem, ExportItem } from "../api/types";
import { Button, Card, ErrorText, Spinner } from "../components/ui";
import { Dropzone } from "../components/Dropzone";
import { useDialog } from "../components/Dialog";
import { ChevronIcon, TrashIcon } from "../components/icons";
import { formatBytes, formatDate } from "../lib/format";

const pdfName = (name: string) => (/\.pdf$/i.test(name) ? name : `${name}.pdf`);

export function DocumentsPage() {
  const navigate = useNavigate();
  const { t } = useTranslation();
  const dialog = useDialog();
  const [items, setItems] = useState<DocumentItem[] | null>(null);
  const [exports, setExports] = useState<ExportItem[]>([]);
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const fileRef = useRef<HTMLInputElement>(null);

  const reload = async () => {
    try {
      const [docs, exp] = await Promise.all([
        api.get<{ documents: DocumentItem[] }>("/documents"),
        api.get<{ exports: ExportItem[] }>("/exports"),
      ]);
      setItems(docs.documents ?? []);
      setExports(exp.exports ?? []);
    } catch (err) {
      setError(errMessage(err, t("common.failedLoad")));
    }
  };

  useEffect(() => {
    void reload();
  }, []);

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

  const uploadFiles = async (files: File[]) => {
    setError("");
    setBusy(true);
    try {
      for (const file of files) {
        const isPdf =
          file.type === "application/pdf" || /\.pdf$/i.test(file.name);
        if (!isPdf) {
          setError(t("documents.notPdf", { name: file.name }));
          continue;
        }
        await api.upload<DocumentItem>("/documents", file, file.name);
      }
      await reload();
    } catch (err) {
      setError(errMessage(err, t("common.uploadFailed")));
    } finally {
      setBusy(false);
    }
  };

  const onUpload = (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(e.target.files || []);
    e.target.value = "";
    if (files.length) void uploadFiles(files);
  };

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

  const fallback =
    items === null ? (
      <Spinner />
    ) : (
      <Card className="p-10 text-center text-sm text-gray-500">
        {t("documents.empty")}
      </Card>
    );

  return (
    <Dropzone onFiles={uploadFiles} label={t("documents.drop")}>
      <div className="space-y-6">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h1 className="text-xl font-semibold text-gray-900">
              {t("documents.title")}
            </h1>
            <p className="text-sm text-gray-500">{t("documents.subtitle")}</p>
          </div>
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

        <ErrorText>{error}</ErrorText>

        {items === null || items.length === 0 ? (
          fallback
        ) : (
          <Card className="divide-y divide-gray-100">
            {items.map((d) => {
              const docExports = exportsByDoc[d.id] ?? [];
              const isOpen = expanded[d.id];
              return (
                <div key={d.id}>
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
                                <a
                                  href={`/api/exports/${x.id}/file`}
                                  download
                                >
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
    </Dropzone>
  );
}

