import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { api } from "../api/client";
import type { VersionInfo } from "../api/types";

// The version never changes within a running build, so fetch it once and share it across
// every mount.
let cached: VersionInfo | null = null;

// `git describe` appends "-<commits>-g<sha>" once HEAD is past a tag and "-dirty" for a
// modified tree; a bare SHA or "dev" is the no-tag fallback. Only a clean tag (e.g.
// "v1.2.3" or "v1.2.3-rc1") has a GitHub release page, so we link those and leave every
// intermediate/dev build as plain text.
const DESCRIBE_AHEAD = /-\d+-g[0-9a-f]+$/;

function releaseURL(version: string, repoURL?: string): string | null {
  if (!repoURL || !version.startsWith("v")) return null;
  if (version.endsWith("-dirty") || DESCRIBE_AHEAD.test(version)) return null;
  return `${repoURL.replace(/\/+$/, "")}/releases/tag/${encodeURIComponent(version)}`;
}

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
  const href = releaseURL(info.version, info.repoURL);

  if (href) {
    const linkClass = [
      "underline decoration-dotted underline-offset-2 transition-colors hover:text-gray-600",
      className,
    ]
      .filter(Boolean)
      .join(" ");
    return (
      <a
        className={linkClass}
        href={href}
        target="_blank"
        rel="noopener noreferrer"
        title={detail}
        aria-label={label}
      >
        {info.version}
      </a>
    );
  }

  return (
    <span className={className} title={detail} aria-label={label}>
      {info.version}
    </span>
  );
}
