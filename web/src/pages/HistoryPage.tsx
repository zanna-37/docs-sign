import { useEffect, useState } from "react";
import { api, ApiError } from "../api/client";
import type { ExportItem } from "../api/types";
import { Button, Card, ErrorText, Spinner } from "../components/ui";
import { formatBytes, formatDate } from "../lib/format";

export function HistoryPage() {
  const [items, setItems] = useState<ExportItem[] | null>(null);
  const [error, setError] = useState("");

  const reload = async () => {
    try {
      const res = await api.get<{ exports: ExportItem[] }>("/exports");
      setItems(res.exports ?? []);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load.");
    }
  };

  useEffect(() => {
    void reload();
  }, []);

  const remove = async (x: ExportItem) => {
    if (!confirm(`Delete export "${x.name}"?`)) return;
    try {
      await api.del(`/exports/${x.id}`);
      await reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Delete failed.");
    }
  };

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl font-semibold text-gray-900">Export history</h1>
        <p className="text-sm text-gray-500">
          Your signed, flattened documents. All copies are stored encrypted.
        </p>
      </div>

      <ErrorText>{error}</ErrorText>

      {items === null ? (
        <Spinner />
      ) : items.length === 0 ? (
        <Card className="p-10 text-center text-sm text-gray-500">
          No exports yet. Sign a document to create one.
        </Card>
      ) : (
        <Card className="divide-y divide-gray-100">
          {items.map((x) => (
            <div
              key={x.id}
              className="flex items-center justify-between gap-4 p-4"
            >
              <div className="min-w-0">
                <p className="truncate font-medium text-gray-800">{x.name}</p>
                <p className="text-xs text-gray-400">
                  {x.pageCount} page{x.pageCount === 1 ? "" : "s"} ·{" "}
                  {formatBytes(x.byteSize)} · {formatDate(x.createdAt)}
                </p>
              </div>
              <div className="flex shrink-0 gap-1">
                <a href={`/api/exports/${x.id}/file`}>
                  <Button>Download</Button>
                </a>
                <Button variant="ghost" onClick={() => remove(x)}>
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
