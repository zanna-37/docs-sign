import { useTranslation } from "react-i18next";
import { formatBytes } from "../lib/format";
import type { UploadProgressState } from "../lib/useUploads";

// UploadProgress renders a determinate progress bar for an in-flight upload batch: which file of
// how many is uploading, the overall percentage, and the byte totals across the batch.
export function UploadProgress({ progress }: Readonly<{ progress: UploadProgressState }>) {
  const { t } = useTranslation();
  const pct = progress.bytes > 0 ? Math.round((progress.loaded / progress.bytes) * 100) : 0;

  return (
    <div className="rounded-xl border border-blue-100 bg-blue-50/70 p-4">
      <div className="mb-2 flex items-center justify-between text-sm">
        <span className="font-medium text-blue-800">
          {t("uploads.progress", {
            done: Math.min(progress.done + 1, progress.total),
            total: progress.total,
          })}
        </span>
        <span className="tabular-nums text-blue-700">{pct}%</span>
      </div>
      {progress.current && (
        <p className="mb-2 truncate text-xs text-blue-700/80">{progress.current}</p>
      )}
      <div
        className="h-2 w-full overflow-hidden rounded-full bg-blue-100"
        role="progressbar"
        aria-valuenow={pct}
        aria-valuemin={0}
        aria-valuemax={100}
      >
        <div
          className="h-full rounded-full bg-blue-600 transition-all duration-150"
          style={{ width: `${pct}%` }}
        />
      </div>
      <p className="mt-1 text-right text-xs tabular-nums text-blue-700/70">
        {formatBytes(progress.loaded)} / {formatBytes(progress.bytes)}
      </p>
    </div>
  );
}
