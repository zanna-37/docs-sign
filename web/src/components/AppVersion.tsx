import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { api } from "../api/client";
import type { VersionInfo } from "../api/types";

// The version never changes within a running build, so fetch it once and share it across
// every mount.
let cached: VersionInfo | null = null;

export function AppVersion({ className }: Readonly<{ className?: string }>) {
  const { t } = useTranslation();
  const [info, setInfo] = useState<VersionInfo | null>(cached);

  useEffect(() => {
    if (cached) return;
    let alive = true;
    api
      .get<VersionInfo>("/version")
      .then((v) => {
        cached = v;
        if (alive) setInfo(v);
      })
      // The version is non-essential chrome; stay silent if it can't be fetched.
      .catch(() => {});
    return () => {
      alive = false;
    };
  }, []);

  if (!info?.version) return null;

  const label = `${t("common.version")} ${info.version}`;
  const detail = info.commit ? `${info.version} (${info.commit})` : info.version;
  return (
    <span className={className} title={detail} aria-label={label}>
      {info.version}
    </span>
  );
}
