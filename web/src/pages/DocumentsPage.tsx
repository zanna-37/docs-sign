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
                      <Button
                        variant="secondary"
                        onClick={() =>
                          setExpanded((m) => ({ ...m, [d.id]: !m[d.id] }))
                        }
                      >
                        {isOpen ? "▾" : "▸"} Signed ({docExports.length})
                      </Button>
                      <Button variant="ghost" onClick={() => rename(d)}>
                        Rename
                      </Button>
                      <Button variant="ghost" onClick={() => remove(d)}>
                        Delete
                      </Button>
                    </div>
                  </div>

                  {isOpen && (
                    <div className="border-t border-gray-100 bg-gray-50 px-4 py-3">
                      {docExports.length === 0 ? (
                        <p className="text-sm text-gray-400">
                          No signed copies yet. Use “Sign” to create one.
                        </p>
                      ) : (
                        <ul className="space-y-2">
                          {docExports.map((x) => (
                            <li
                              key={x.id}
                              className="flex items-center justify-between gap-3"
                            >
                              <div className="min-w-0">
                                <p className="truncate text-sm text-gray-700">
                                  {x.name}
                                </p>
                                <p className="text-xs text-gray-400">
                                  {formatBytes(x.byteSize)} ·{" "}
                                  {formatDate(x.createdAt)}
                                </p>
                              </div>
                              <div className="flex shrink-0 gap-1">
                                <a href={`/api/exports/${x.id}/file`}>
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
