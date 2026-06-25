import { useState } from "react";
import { useTranslation } from "react-i18next";
import type { ConflictAction, ConflictResolution } from "../../api/types";
import { Button, Input, Modal } from "../ui";

export interface ConflictItem {
  id: string;
  name: string;
  destPath: string;
}

// suggestName proposes a free name by inserting " (1)" before the extension.
function suggestName(name: string): string {
  const dot = name.lastIndexOf(".");
  if (dot > 0) return `${name.slice(0, dot)} (1)${name.slice(dot)}`;
  return `${name} (1)`;
}

// ConflictDialog resolves name collisions (on upload or restore) one file at a time, with an
// "apply to all" shortcut. It returns a resolution per conflict id: override, skip, or rename.
export function ConflictDialog({
  conflicts,
  onResolve,
  onCancel,
}: Readonly<{
  conflicts: ConflictItem[];
  onResolve: (res: Record<string, ConflictResolution>) => void;
  onCancel: () => void;
}>) {
  const { t } = useTranslation();
  const [res, setRes] = useState<Record<string, ConflictResolution>>({});

  const pick = (c: ConflictItem, action: ConflictAction) =>
    setRes((r) => ({
      ...r,
      [c.id]: {
        action,
        newName: action === "rename" ? (r[c.id]?.newName ?? suggestName(c.name)) : undefined,
      },
    }));

  const setName = (id: string, newName: string) =>
    setRes((r) => ({ ...r, [id]: { action: "rename", newName } }));

  const applyAll = (action: ConflictAction) =>
    setRes(Object.fromEntries(conflicts.map((c) => [c.id, { action }])));

  const complete = conflicts.every((c) => {
    const r = res[c.id];
    return r && (r.action !== "rename" || !!r.newName?.trim());
  });

  const actionBtn = (c: ConflictItem, action: ConflictAction, label: string) => {
    const active = res[c.id]?.action === action;
    return (
      <button
        type="button"
        onClick={() => pick(c, action)}
        className={`rounded-md px-2.5 py-1 text-xs font-medium transition ${
          active
            ? "bg-blue-600 text-white"
            : "bg-gray-100 text-gray-600 hover:bg-gray-200"
        }`}
      >
        {label}
      </button>
    );
  };

  return (
    <Modal open title={t("conflicts.title")} onClose={onCancel}>
      <div className="space-y-4">
        <p className="text-sm text-gray-500">{t("conflicts.body")}</p>

        <div className="flex items-center gap-2 text-xs">
          <span className="text-gray-400">{t("conflicts.applyAll")}</span>
          <button
            type="button"
            onClick={() => applyAll("override")}
            className="rounded-md bg-gray-100 px-2.5 py-1 font-medium text-gray-600 hover:bg-gray-200"
          >
            {t("conflicts.overrideAll")}
          </button>
          <button
            type="button"
            onClick={() => applyAll("skip")}
            className="rounded-md bg-gray-100 px-2.5 py-1 font-medium text-gray-600 hover:bg-gray-200"
          >
            {t("conflicts.skipAll")}
          </button>
        </div>

        <ul className="max-h-72 space-y-3 overflow-auto">
          {conflicts.map((c) => (
            <li key={c.id} className="rounded-lg border border-gray-200 p-3">
              <p className="truncate text-sm font-medium text-gray-800">{c.name}</p>
              <p className="mb-2 truncate text-xs text-gray-400">
                {t("conflicts.in", { path: c.destPath })}
              </p>
              <div className="flex flex-wrap items-center gap-1.5">
                {actionBtn(c, "override", t("conflicts.override"))}
                {actionBtn(c, "skip", t("conflicts.skip"))}
                {actionBtn(c, "rename", t("conflicts.rename"))}
              </div>
              {res[c.id]?.action === "rename" && (
                <Input
                  className="mt-2"
                  autoFocus
                  value={res[c.id]?.newName ?? ""}
                  onChange={(e) => setName(c.id, e.target.value)}
                />
              )}
            </li>
          ))}
        </ul>

        <div className="flex justify-end gap-2">
          <Button variant="secondary" onClick={onCancel}>
            {t("common.cancel")}
          </Button>
          <Button disabled={!complete} onClick={() => onResolve(res)}>
            {t("conflicts.apply")}
          </Button>
        </div>
      </div>
    </Modal>
  );
}
