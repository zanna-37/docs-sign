import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { api, ApiError } from "../api/client";
import type { TrashItem } from "../api/types";
import { Button, Card, ErrorText, Spinner } from "../components/ui";
import { formatBytes, formatDate } from "../lib/format";

export function TrashPage() {
  const { t } = useTranslation();
  const [items, setItems] = useState<TrashItem[] | null>(null);
  const [retentionDays, setRetentionDays] = useState(30);
  const [error, setError] = useState("");

  const kindLabel = (kind: TrashItem["kind"]) =>
    t(
      kind === "signature"
        ? "trash.kindSignature"
        : kind === "document"
          ? "trash.kindDocument"
          : "trash.kindExport",
    );

  const reload = async () => {
    try {
      const res = await api.get<{ items: TrashItem[]; retentionDays: number }>(
        "/trash",
      );
      setItems(res.items ?? []);
      setRetentionDays(res.retentionDays || 30);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.failedLoad"));
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
      setError(err instanceof ApiError ? err.message : t("trash.restoreFailed"));
    }
  };

  const purge = async (it: TrashItem) => {
    if (!confirm(t("trash.confirmPurge", { name: it.name }))) return;
    try {
      await api.del(`/trash/${it.kind}/${it.id}`);
      await reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("common.deleteFailed"));
    }
  };

  const empty = async () => {
    if (!confirm(t("trash.confirmEmpty"))) return;
    try {
      await api.post("/trash/empty");
      await reload();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("trash.emptyFailed"));
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-xl font-semibold text-gray-900">
            {t("trash.title")}
          </h1>
          <p className="text-sm text-gray-500">
            {t("trash.subtitle", { days: retentionDays })}
          </p>
        </div>
        {items && items.length > 0 && (
          <Button variant="danger" onClick={empty}>
            {t("trash.emptyButton")}
          </Button>
        )}
      </div>

      <ErrorText>{error}</ErrorText>

      {items === null ? (
        <Spinner />
      ) : items.length === 0 ? (
        <Card className="p-10 text-center text-sm text-gray-500">
          {t("trash.empty")}
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
                    {kindLabel(it.kind)}
                  </span>
                </p>
                <p className="text-xs text-gray-400">
                  {t("trash.meta", {
                    size: formatBytes(it.byteSize),
                    deleted: formatDate(it.deletedAt),
                    purge: formatDate(it.purgeAt),
                  })}
                </p>
              </div>
              <div className="flex shrink-0 gap-1">
                <Button variant="secondary" onClick={() => restore(it)}>
                  {t("common.restore")}
                </Button>
                <Button variant="ghost" onClick={() => purge(it)}>
                  {t("common.deleteNow")}
                </Button>
              </div>
            </div>
          ))}
        </Card>
      )}
    </div>
  );
}
