const fetchOpts: RequestInit = {
  cache: "no-store",
  credentials: "same-origin",
  headers: { "X-Requested-With": "fetch" },
};

// fetchArrayBuffer fetches a protected resource with no-store and returns its bytes,
// keeping them in memory only (no object URL, no HTTP cache entry).
export async function fetchArrayBuffer(url: string): Promise<ArrayBuffer> {
  const res = await fetch(url, fetchOpts);
  if (!res.ok) throw new Error(`request failed: ${res.status}`);
  return res.arrayBuffer();
}
