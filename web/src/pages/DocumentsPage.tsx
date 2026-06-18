import { useEffect, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { api, ApiError } from "../api/client";
import type { DocumentItem } from "../api/types";
import { Button, Card, ErrorText, Spinner } from "../components/ui";
import { formatBytes, formatDate } from "../lib/format";

export function DocumentsPage() {
  const navigate = useNavigate();
  const [items, setItems] = useState<DocumentItem[] | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const fileRef = useRef<HTMLInputElement>(null);

  const reload = async () => {
    try {
      const res = await api.get<{ documents: DocumentItem[] }>("/documents");
      setItems(res.documents ?? []);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load.");
    }
  };

  useEffect(() => {
    void reload();
  }, []);

  const onUpload = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    e.target.value = "";
    if (!file) return;
    setError("");
    setBusy(true);
    try {
      await api.upload<DocumentItem>("/documents", file, file.name);
      await reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Upload failed.");
    } finally {
      setBusy(false);
    }
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
    if (!confirm(`Delete document "${d.name}"?`)) return;
    try {
      await api.del(`/documents/${d.id}`);
      await reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Delete failed.");
    }
  };

  return (
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
          No documents yet. Upload a PDF to get started.
        </Card>
      ) : (
        <Card className="divide-y divide-gray-100">
          {items.map((d) => (
            <div
              key={d.id}
              className="flex items-center justify-between gap-4 p-4"
            >
              <div className="min-w-0">
                <p className="truncate font-medium text-gray-800">{d.name}</p>
                <p className="text-xs text-gray-400">
                  {d.pageCount} page{d.pageCount === 1 ? "" : "s"} ·{" "}
                  {formatBytes(d.byteSize)} · {formatDate(d.createdAt)}
                </p>
              </div>
              <div className="flex shrink-0 gap-1">
                <Button onClick={() => navigate(`/documents/${d.id}/sign`)}>
                  Sign
                </Button>
                <Button variant="ghost" onClick={() => rename(d)}>
                  Rename
                </Button>
                <Button variant="ghost" onClick={() => remove(d)}>
                  Delete
                </Button>
              </div>
            </div>
          ))}
        </Card>
      )}
    </div>
  );
}
