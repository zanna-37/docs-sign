import { useEffect, useRef, useState } from "react";
import { api, ApiError } from "../api/client";
import type { Signature } from "../api/types";
import { Button, Card, ErrorText, Spinner } from "../components/ui";
import { SignatureImage } from "../components/SignatureImage";
import { formatBytes, formatDate } from "../lib/format";

export function SignaturesPage() {
  const [items, setItems] = useState<Signature[] | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const fileRef = useRef<HTMLInputElement>(null);

  const reload = async () => {
    try {
      const res = await api.get<{ signatures: Signature[] }>("/signatures");
      setItems(res.signatures ?? []);
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
      await api.upload<Signature>("/signatures", file, file.name);
      await reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Upload failed.");
    } finally {
      setBusy(false);
    }
  };

  const rename = async (s: Signature) => {
    const name = prompt("Rename signature", s.name);
    if (!name || name === s.name) return;
    try {
      await api.patch(`/signatures/${s.id}`, { name });
      await reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Rename failed.");
    }
  };

  const remove = async (s: Signature) => {
    if (!confirm(`Delete signature "${s.name}"?`)) return;
    try {
      await api.del(`/signatures/${s.id}`);
      await reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Delete failed.");
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold text-gray-900">Signatures</h1>
          <p className="text-sm text-gray-500">
            Upload transparent PNG signatures to place on your documents.
          </p>
        </div>
        <input
          ref={fileRef}
          type="file"
          accept="image/png"
          className="hidden"
          onChange={onUpload}
        />
        <Button onClick={() => fileRef.current?.click()} disabled={busy}>
          {busy ? "Uploading…" : "Upload PNG"}
        </Button>
      </div>

      <ErrorText>{error}</ErrorText>

      {items === null ? (
        <Spinner />
      ) : items.length === 0 ? (
        <Card className="p-10 text-center text-sm text-gray-500">
          No signatures yet. Upload a transparent PNG to get started.
        </Card>
      ) : (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {items.map((s) => (
            <Card key={s.id} className="overflow-hidden">
              <div
                className="flex h-36 items-center justify-center p-4"
                style={{
                  backgroundColor: "#fff",
                  backgroundImage:
                    "linear-gradient(45deg,#eee 25%,transparent 25%),linear-gradient(-45deg,#eee 25%,transparent 25%),linear-gradient(45deg,transparent 75%,#eee 75%),linear-gradient(-45deg,transparent 75%,#eee 75%)",
                  backgroundSize: "16px 16px",
                  backgroundPosition: "0 0,0 8px,8px -8px,-8px 0",
                }}
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
                  <Button variant="ghost" onClick={() => rename(s)}>
                    Rename
                  </Button>
                  <Button variant="ghost" onClick={() => remove(s)}>
                    Delete
                  </Button>
                </div>
              </div>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}
