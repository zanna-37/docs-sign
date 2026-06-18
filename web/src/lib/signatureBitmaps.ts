import { useEffect, useState } from "react";
import { fetchArrayBuffer } from "./blobUrls";

// decode fetches a signature PNG with no-store and decodes it straight into an ImageBitmap.
// No Blob object URL and no <img> resource are created, so the decoded image lives only as
// pixels in memory (a canvas backing store) that the app owns and can release.
async function decode(id: string): Promise<ImageBitmap> {
  const buf = await fetchArrayBuffer(`/api/signatures/${id}/image`);
  // The Blob here is ephemeral input to the decoder and is never exposed as a URL.
  return createImageBitmap(new Blob([buf], { type: "image/png" }));
}

export function useSignatureBitmap(id: string | null): ImageBitmap | null {
  const [bmp, setBmp] = useState<ImageBitmap | null>(null);
  useEffect(() => {
    if (!id) {
      setBmp(null);
      return;
    }
    let cancelled = false;
    let current: ImageBitmap | null = null;
    decode(id)
      .then((b) => {
        if (cancelled) {
          b.close();
          return;
        }
        current = b;
        setBmp(b);
      })
      .catch(() => {});
    return () => {
      cancelled = true;
      current?.close();
      setBmp(null);
    };
  }, [id]);
  return bmp;
}

export function useSignatureBitmaps(ids: string[]): Record<string, ImageBitmap> {
  const dep = ids.join(",");
  const [map, setMap] = useState<Record<string, ImageBitmap>>({});
  useEffect(() => {
    let cancelled = false;
    const created: ImageBitmap[] = [];
    (async () => {
      const entries = await Promise.all(
        ids.map(async (id) => {
          try {
            const b = await decode(id);
            created.push(b);
            return [id, b] as const;
          } catch {
            return null;
          }
        }),
      );
      if (cancelled) {
        created.forEach((b) => b.close());
        return;
      }
      setMap(
        Object.fromEntries(
          entries.filter((e): e is [string, ImageBitmap] => e !== null),
        ),
      );
    })();
    return () => {
      cancelled = true;
      created.forEach((b) => b.close());
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [dep]);
  return map;
}
