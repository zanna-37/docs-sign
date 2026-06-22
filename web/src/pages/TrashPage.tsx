import { useCallback, useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { ApiError, api, errMessage } from "../api/client";
import type {
  RestoreConflict,
  TrashEntry,
  TrashEvent,
  TrashKind,
} from "../api/types";
import { Button, Card, ErrorText, Spinner } from "../components/ui";
import { useDialog } from "../components/Dialog";
import { FolderIcon } from "../components/icons";
import {
  ConflictDialog,
  type ConflictItem,
} from "../components/folders/ConflictDialog";
import { formatBytes, formatDate } from "../lib/format";

const KIND_LABEL_KEYS: Record<TrashKind, string> = {
  folder: "trash.kindFolder",
  signature: "trash.kindSignature",
  document: "trash.kindDocument",
  export: "trash.kindExport",
};

interface RestoreTarget {
  kind: TrashKind;
  id: string;
  conflicts: ConflictItem[];
}

export function TrashPage() {
  const { t } = useTranslation();
  const dialog = useDialog();
  const [events, setEvents] = useState<TrashEvent[] | null>(null);
  const [retentionDays, setRetentionDays] = useState(30);
  const [error, setError] = useState("");

  // Walking state: the event we are inside and the folder trail within it.
  const [openEventId, setOpenEventId] = useState<string | null>(null);
  const [trail, setTrail] = useState<{ id: string; name: string }[]>([]);
  const [entries, setEntries] = useState<TrashEntry[]>([]);
  const [walkVersion, setWalkVersion] = useState(0);

  const [restoring, setRestoring] = useState<RestoreTarget | null>(null);

  const kindLabel = (kind: TrashKind) => t(KIND_LABEL_KEYS[kind]);

  const reloadEvents = useCallback(async () => {
    const res = await api.get<{ events: TrashEvent[]; retentionDays: number }>(
      "/trash",
    );
    setEvents(res.events ?? []);
    setRetentionDays(res.retentionDays || 30);
    return res.events ?? [];
  }, []);

  useEffect(() => {
    reloadEvents().catch((err) =>
      setError(errMessage(err, t("common.failedLoad"))),
    );
  }, [reloadEvents, t]);

  // Load the children of the folder at the tip of the trail.
  useEffect(() => {
    if (!openEventId || trail.length === 0) {
      setEntries([]);
      return;
    }
    const fid = trail[trail.length - 1].id;
    let cancelled = false;
    api
      .get<{ entries: TrashEntry[] }>(
        `/trash/events/${openEventId}?folder=${encodeURIComponent(fid)}`,
      )
      .then((r) => !cancelled && setEntries(r.entries ?? []))
      .catch((err) => !cancelled && setError(errMessage(err, t("common.failedLoad"))));
    return () => {
      cancelled = true;
    };
  }, [openEventId, trail, walkVersion, t]);

  // refresh re-reads the event list and, if the open event still exists, re-walks it; otherwise
  // it closes the walk (the event was fully restored or purged away).
  const refresh = async () => {
    const evs = await reloadEvents();
    if (openEventId && !evs.some((e) => e.eventId === openEventId)) {
      setOpenEventId(null);
      setTrail([]);
    } else {
      setWalkVersion((v) => v + 1);
    }
  };

  const doRestore = async (kind: TrashKind, id: string) => {
    setError("");
    try {
      await api.post(`/trash/${kind}/${id}/restore`, {});
      await refresh();
    } catch (err) {
      if (err instanceof ApiError && err.status === 409) {
        const conflicts = (err.data as { conflicts: RestoreConflict[] }).conflicts;
        setRestoring({
          kind,
          id,
          conflicts: conflicts.map((c) => ({
            id: c.id,
            name: c.name,
            destPath: c.destPath,
          })),
        });
        return;
      }
      setError(errMessage(err, t("trash.restoreFailed")));
    }
  };

  const resolveRestore = async (
    resolutions: Record<string, { action: string; newName?: string }>,
  ) => {
    if (!restoring) return;
    const { kind, id } = restoring;
    setRestoring(null);
    try {
      await api.post(`/trash/${kind}/${id}/restore`, { resolutions });
      await refresh();
    } catch (err) {
      setError(errMessage(err, t("trash.restoreFailed")));
    }
  };

  const purge = async (ev: TrashEvent) => {
    if (
      !(await dialog.confirm({
        title: t("trash.confirmPurge", { name: ev.label }),
        confirmLabel: t("common.deleteNow"),
        danger: true,
      }))
    )
      return;
    try {
      await api.del(`/trash/events/${ev.eventId}`);
      await refresh();
    } catch (err) {
      setError(errMessage(err, t("common.deleteFailed")));
    }
  };

  const empty = async () => {
    if (
      !(await dialog.confirm({
        title: t("trash.confirmEmpty"),
        confirmLabel: t("trash.emptyButton"),
        danger: true,
      }))
    )
      return;
    try {
      await api.post("/trash/empty");
      await refresh();
    } catch (err) {
      setError(errMessage(err, t("trash.emptyFailed")));
    }
  };

  const openEvent = (ev: TrashEvent) => {
    setOpenEventId(ev.eventId);
    setTrail([{ id: ev.rootId, name: ev.label }]);
  };

  const inWalk = openEventId !== null && trail.length > 0;

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
        {!inWalk && events && events.length > 0 && (
          <Button variant="danger" onClick={empty}>
            {t("trash.emptyButton")}
          </Button>
        )}
      </div>

      <ErrorText>{error}</ErrorText>

      {inWalk ? (
        <TrashWalk
          trail={trail}
          entries={entries}
          kindLabel={kindLabel}
          onClose={() => {
            setOpenEventId(null);
            setTrail([]);
          }}
          onCrumb={(i) => setTrail((tr) => tr.slice(0, i + 1))}
          onOpenFolder={(e) => setTrail((tr) => [...tr, { id: e.id, name: e.name }])}
          onRestore={(e) => doRestore(e.kind, e.id)}
        />
      ) : events === null ? (
        <Spinner />
      ) : events.length === 0 ? (
        <Card className="p-10 text-center text-sm text-gray-500">
          {t("trash.empty")}
        </Card>
      ) : (
        <Card className="divide-y divide-gray-100">
          {events.map((ev) => (
            <div
              key={ev.eventId}
              className="flex flex-wrap items-center justify-between gap-3 p-4"
            >
              <div className="min-w-0">
                <p className="flex items-center gap-2 font-medium text-gray-800">
                  {ev.rootKind === "folder" && (
                    <FolderIcon className="h-4 w-4 shrink-0 text-blue-500" />
                  )}
                  <span className="truncate">{ev.label}</span>
                  <span className="rounded bg-gray-100 px-1.5 py-0.5 text-xs font-medium text-gray-500">
                    {kindLabel(ev.rootKind)}
                  </span>
                </p>
                <p className="text-xs text-gray-400">
                  {ev.rootKind === "folder"
                    ? t("trash.folderMeta", {
                        count: ev.itemCount,
                        size: formatBytes(ev.byteSize),
                        deleted: formatDate(ev.deletedAt),
                        purge: formatDate(ev.purgeAt),
                      })
                    : t("trash.meta", {
                        size: formatBytes(ev.byteSize),
                        deleted: formatDate(ev.deletedAt),
                        purge: formatDate(ev.purgeAt),
                      })}
                </p>
              </div>
              <div className="flex shrink-0 gap-1">
                {ev.rootKind === "folder" && (
                  <Button variant="secondary" onClick={() => openEvent(ev)}>
                    {t("trash.open")}
                  </Button>
                )}
                <Button
                  variant="secondary"
                  onClick={() => doRestore(ev.rootKind, ev.rootId)}
                >
                  {t("common.restore")}
                </Button>
                <Button variant="ghost" onClick={() => purge(ev)}>
                  {t("common.deleteNow")}
                </Button>
              </div>
            </div>
          ))}
        </Card>
      )}

      {restoring && (
        <ConflictDialog
          conflicts={restoring.conflicts}
          onCancel={() => setRestoring(null)}
          onResolve={resolveRestore}
        />
      )}
    </div>
  );
}

// TrashWalk renders the contents of a trashed folder, navigable like a read-only file browser,
// with per-node restore.
function TrashWalk({
  trail,
  entries,
  kindLabel,
  onClose,
  onCrumb,
  onOpenFolder,
  onRestore,
}: Readonly<{
  trail: { id: string; name: string }[];
  entries: TrashEntry[];
  kindLabel: (kind: TrashKind) => string;
  onClose: () => void;
  onCrumb: (index: number) => void;
  onOpenFolder: (entry: TrashEntry) => void;
  onRestore: (entry: TrashEntry) => void;
}>) {
  const { t } = useTranslation();
  return (
    <div className="space-y-4">
      <nav className="flex flex-wrap items-center gap-0.5 text-sm">
        <button
          type="button"
          onClick={onClose}
          className="rounded px-1.5 py-0.5 font-medium text-gray-600 hover:bg-gray-100 hover:text-gray-900"
        >
          {t("trash.title")}
        </button>
        {trail.map((c, i) => (
          <span key={c.id} className="flex items-center gap-0.5">
            <span className="text-gray-300">/</span>
            <button
              type="button"
              onClick={() => onCrumb(i)}
              className="rounded px-1.5 py-0.5 font-medium text-gray-600 hover:bg-gray-100 hover:text-gray-900"
            >
              {c.name}
            </button>
          </span>
        ))}
      </nav>

      {entries.length === 0 ? (
        <Card className="p-10 text-center text-sm text-gray-500">
          {t("trash.folderEmpty")}
        </Card>
      ) : (
        <Card className="divide-y divide-gray-100">
          {entries.map((e) => (
            <div
              key={`${e.kind}-${e.id}`}
              className="flex flex-wrap items-center justify-between gap-3 p-4"
            >
              <button
                type="button"
                disabled={!e.isFolder}
                onClick={() => e.isFolder && onOpenFolder(e)}
                className="flex min-w-0 items-center gap-2 text-left disabled:cursor-default"
              >
                {e.isFolder && (
                  <FolderIcon className="h-4 w-4 shrink-0 text-blue-500" />
                )}
                <span className="truncate font-medium text-gray-800">
                  {e.name}
                </span>
                <span className="rounded bg-gray-100 px-1.5 py-0.5 text-xs font-medium text-gray-500">
                  {e.isFolder ? kindLabel("folder") : kindLabel(e.kind)}
                </span>
              </button>
              <div className="flex shrink-0 gap-1">
                {!e.isFolder && (
                  <span className="self-center text-xs text-gray-400">
                    {formatBytes(e.byteSize)}
                  </span>
                )}
                <Button variant="secondary" onClick={() => onRestore(e)}>
                  {t("common.restore")}
                </Button>
              </div>
            </div>
          ))}
        </Card>
      )}
    </div>
  );
}
