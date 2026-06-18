import { useEffect, useState } from "react";
import { api, ApiError } from "../api/client";
import type { TrashItem } from "../api/types";
import { Button, Card, ErrorText, Spinner } from "../components/ui";
import { formatBytes, formatDate } from "../lib/format";

const kindLabel: Record<TrashItem["kind"], string> = {
  signature: "Signature",
  document: "Document",
  export: "Signed copy",
};

export function TrashPage() {
  const [items, setItems] = useState<TrashItem[] | null>(null);
  const [retentionDays, setRetentionDays] = useState(30);
  const [error, setError] = useState("");

  const reload = async () => {
    try {
      const res = await api.get<{ items: TrashItem[]; retentionDays: number }>(
        "/trash",
      );
      setItems(res.items ?? []);
      setRetentionDays(res.retentionDays || 30);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load.");
    }
  };

  useEffect(() => {
    void reload();
  }, []);

  const restore = async (it: TrashItem) => {
    try {
      await api.post(`/trash/${it.kind}/${it.id}/restore`);
      await reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Restore failed.");
    }
  };

  const purge = async (it: TrashItem) => {
    if (!confirm(`Permanently delete "${it.name}"? This cannot be undone.`)) return;
    try {
      await api.del(`/trash/${it.kind}/${it.id}`);
      await reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Delete failed.");
    }
  };

  const empty = async () => {
    if (!confirm("Permanently delete everything in the trash? This cannot be undone.")) return;
    try {
      await api.post("/trash/empty");
      await reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to empty trash.");
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-semibold text-gray-900">Trash</h1>
          <p className="text-sm text-gray-500">
            Deleted items are kept for {retentionDays} days, then permanently
            removed.
          </p>
        </div>
        {items && items.length > 0 && (
          <Button variant="danger" onClick={empty}>
            Empty trash
          </Button>
        )}
      </div>

      <ErrorText>{error}</ErrorText>

      {items === null ? (
        <Spinner />
      ) : items.length === 0 ? (
        <Card className="p-10 text-center text-sm text-gray-500">
          The trash is empty.
        </Card>
      ) : (
        <Card className="divide-y divide-gray-100">
          {items.map((it) => (
            <div
              key={`${it.kind}-${it.id}`}
              className="flex flex-wrap items-center justify-between gap-3 p-4"
            >
              <div className="min-w-0">
                <p className="flex items-center gap-2 font-medium text-gray-800">
                  <span className="truncate">{it.name}</span>
                  <span className="rounded bg-gray-100 px-1.5 py-0.5 text-xs font-medium text-gray-500">
                    {kindLabel[it.kind]}
                  </span>
                </p>
                <p className="text-xs text-gray-400">
                  {formatBytes(it.byteSize)} · deleted {formatDate(it.deletedAt)}{" "}
                  · auto-deletes {formatDate(it.purgeAt)}
                </p>
              </div>
              <div className="flex shrink-0 gap-1">
                <Button variant="secondary" onClick={() => restore(it)}>
                  Restore
                </Button>
                <Button variant="ghost" onClick={() => purge(it)}>
                  Delete now
                </Button>
              </div>
            </div>
          ))}
        </Card>
      )}
    </div>
  );
}
