import { useEffect, useState } from "react";

const fetchOpts: RequestInit = {
  cache: "no-store",
  credentials: "same-origin",
  headers: { "X-Requested-With": "fetch" },
};

// useBlobUrl fetches a protected resource with no-store and exposes an in-memory object
// URL for it, revoking the URL on unmount/change so the decrypted bytes are not retained
// in the browser's HTTP cache and don't linger once unused.
export function useBlobUrl(url: string | null): string | null {
  const [obj, setObj] = useState<string | null>(null);
  useEffect(() => {
    if (!url) {
      setObj(null);
      return;
    }
    let cancelled = false;
    let created: string | null = null;
    (async () => {
      try {
        const res = await fetch(url, fetchOpts);
        if (!res.ok) return;
        const blob = await res.blob();
        if (cancelled) return;
        created = URL.createObjectURL(blob);
        setObj(created);
      } catch {
        /* ignore */
      }
    })();
    return () => {
      cancelled = true;
      if (created) URL.revokeObjectURL(created);
      setObj(null);
    };
  }, [url]);
  return obj;
}

// useBlobUrlMap fetches several resources and returns a map keyed by the caller's key,
// revoking every created URL on unmount/change.
export function useBlobUrlMap(
  items: { key: string; url: string }[],
): Record<string, string> {
  const dep = items.map((i) => `${i.key}|${i.url}`).join(",");
  const [map, setMap] = useState<Record<string, string>>({});
  useEffect(() => {
    let cancelled = false;
    const created: string[] = [];
    (async () => {
      const entries = await Promise.all(
        items.map(async (it) => {
          try {
            const res = await fetch(it.url, fetchOpts);
            if (!res.ok) return null;
            const blob = await res.blob();
            const obj = URL.createObjectURL(blob);
            created.push(obj);
            return [it.key, obj] as const;
          } catch {
            return null;
          }
        }),
      );
      if (cancelled) {
        created.forEach(URL.revokeObjectURL);
        return;
      }
      setMap(
        Object.fromEntries(
          entries.filter((e): e is [string, string] => e !== null),
        ),
      );
    })();
    return () => {
      cancelled = true;
      created.forEach(URL.revokeObjectURL);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [dep]);
  return map;
}

// fetchArrayBuffer fetches a protected resource with no-store and returns its bytes,
// keeping them in memory only (no object URL, no HTTP cache entry).
export async function fetchArrayBuffer(url: string): Promise<ArrayBuffer> {
  const res = await fetch(url, fetchOpts);
  if (!res.ok) throw new Error(`request failed: ${res.status}`);
  return res.arrayBuffer();
}
