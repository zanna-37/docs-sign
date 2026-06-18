import { useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { api, ApiError } from "../api/client";
import type { DocumentItem, ExportItem } from "../api/types";
import { Button, Card, ErrorText, Spinner } from "../components/ui";
import { Dropzone } from "../components/Dropzone";
import { formatBytes, formatDate } from "../lib/format";

export function DocumentsPage() {
  const navigate = useNavigate();
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
      setError(err instanceof ApiError ? err.message : "Failed to load.");
    }
  };

  useEffect(() => {
    void reload();
  }, []);

  const exportsFor = (docId: string) =>
    exports.filter((e) => e.documentId === docId);

  const uploadFiles = async (files: File[]) => {
    setError("");
    setBusy(true);
    try {
      for (const file of files) {
        const isPdf =
          file.type === "application/pdf" || /\.pdf$/i.test(file.name);
        if (!isPdf) {
          setError(`"${file.name}" is not a PDF.`);
          continue;
        }
        await api.upload<DocumentItem>("/documents", file, file.name);
      }
      await reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Upload failed.");
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
    const name = prompt("Rename document", d.name);
    if (!name || name === d.name) return;
    try {
      await api.patch(`/documents/${d.id}`, { name });
      await reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Rename failed.");
    }
  };

  const remove = async (d: DocumentItem) => {
    if (!confirm(`Move "${d.name}" and its signed copies to the trash?`)) return;
    try {
      await api.del(`/documents/${d.id}`);
      await reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Delete failed.");
    }
  };

  const removeExport = async (x: ExportItem) => {
    if (!confirm(`Move signed copy "${x.name}" to the trash?`)) return;
    try {
      await api.del(`/exports/${x.id}`);
      await reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Delete failed.");
    }
  };

  return (
    <Dropzone onFiles={uploadFiles} label="Drop PDF documents to upload">
      <div className="space-y-6">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-xl font-semibold text-gray-900">Documents</h1>
            <p className="text-sm text-gray-500">
              Upload PDFs, then place signatures and export a flattened copy.
            </p>
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
            {busy ? "Uploading…" : "Upload PDF"}
          </Button>
        </div>

        <ErrorText>{error}</ErrorText>

        {items === null ? (
          <Spinner />
        ) : items.length === 0 ? (
          <Card className="p-10 text-center text-sm text-gray-500">
            No documents yet. Upload a PDF or drag one here to get started.
          </Card>
        ) : (
          <Card className="divide-y divide-gray-100">
            {items.map((d) => {
              const docExports = exportsFor(d.id);
              const isOpen = expanded[d.id];
              return (
                <div key={d.id}>
                  <div className="flex items-center justify-between gap-4 p-4">
                    <div className="min-w-0">
                      <p className="truncate font-medium text-gray-800">
                        {d.name}
                      </p>
                      <p className="text-xs text-gray-400">
                        {d.pageCount} page{d.pageCount === 1 ? "" : "s"} ·{" "}
                        {formatBytes(d.byteSize)} · {formatDate(d.createdAt)}
                      </p>
                    </div>
                    <div className="flex shrink-0 items-center gap-1">
                      <Button onClick={() => navigate(`/documents/${d.id}/sign`)}>
                        Sign
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
                        <ChevronIcon
                          className={`h-4 w-4 transition-transform duration-150 ${
                            isOpen ? "rotate-90" : ""
                          }`}
                        />
                        Signed
                        <span
                          className={`rounded-full px-1.5 py-0.5 text-xs font-semibold ${
                            docExports.length
                              ? "bg-blue-100 text-blue-700"
                              : "bg-gray-100 text-gray-500"
                          }`}
                        >
                          {docExports.length}
                        </span>
                      </button>
                      <Button variant="ghost" onClick={() => rename(d)}>
                        Rename
                      </Button>
                      <Button variant="ghost" onClick={() => remove(d)}>
                        Delete
                      </Button>
                    </div>
                  </div>

                  {isOpen && (
                    <div className="border-t border-gray-100 bg-gray-50/70 px-4 py-3">
                      <p className="mb-2 px-1 text-xs font-semibold uppercase tracking-wide text-gray-400">
                        Signed copies
                      </p>
                      {docExports.length === 0 ? (
                        <div className="rounded-lg border border-dashed border-gray-200 bg-white px-4 py-6 text-center text-sm text-gray-400">
                          No signed copies yet. Use “Sign” to create one.
                        </div>
                      ) : (
                        <ul className="space-y-2">
                          {docExports.map((x) => (
                            <li
                              key={x.id}
                              className="flex items-center justify-between gap-3 rounded-lg border border-gray-200 bg-white px-3 py-2.5 shadow-sm transition hover:border-gray-300 hover:shadow"
                            >
                              <div className="flex min-w-0 items-center gap-3">
                                <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-red-50 text-red-500">
                                  <PdfIcon />
                                </span>
                                <div className="min-w-0">
                                  <p className="truncate text-sm font-medium text-gray-800">
                                    {x.name}
                                  </p>
                                  <p className="text-xs text-gray-400">
                                    {x.pageCount} page
                                    {x.pageCount === 1 ? "" : "s"} ·{" "}
                                    {formatBytes(x.byteSize)} ·{" "}
                                    {formatDate(x.createdAt)}
                                  </p>
                                </div>
                              </div>
                              <div className="flex shrink-0 items-center gap-1">
                                <a
                                  href={`/api/exports/${x.id}/file`}
                                  download
                                >
                                  <Button variant="secondary">Download</Button>
                                </a>
                                <Button
                                  variant="ghost"
                                  onClick={() => removeExport(x)}
                                >
                                  Delete
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

function ChevronIcon({ className }: { className?: string }) {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2.5"
      strokeLinecap="round"
      strokeLinejoin="round"
      className={className}
      aria-hidden="true"
    >
      <path d="M9 6l6 6-6 6" />
    </svg>
  );
}

function PdfIcon() {
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
      className="h-4 w-4"
      aria-hidden="true"
    >
      <path d="M14 3v4a1 1 0 0 0 1 1h4" />
      <path d="M17 21H7a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h7l5 5v11a2 2 0 0 1-2 2z" />
    </svg>
  );
}
