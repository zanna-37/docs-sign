// uid returns a unique identifier for local-only element ids (not security-sensitive).
// crypto.randomUUID() is only available in secure contexts (HTTPS/localhost), so this
// falls back to crypto.getRandomValues (available everywhere) and finally to Math.random,
// keeping the editor working when the app is served over plain HTTP on a LAN address.
export function uid(): string {
  const c = globalThis.crypto as Crypto | undefined;
  if (c?.randomUUID) {
    return c.randomUUID();
  }
  if (c?.getRandomValues) {
    const b = new Uint8Array(16);
    c.getRandomValues(b);
    b[6] = (b[6] & 0x0f) | 0x40; // version 4
    b[8] = (b[8] & 0x3f) | 0x80; // variant
    const h = Array.from(b, (x) => x.toString(16).padStart(2, "0"));
    return `${h[0]}${h[1]}${h[2]}${h[3]}-${h[4]}${h[5]}-${h[6]}${h[7]}-${h[8]}${h[9]}-${h[10]}${h[11]}${h[12]}${h[13]}${h[14]}${h[15]}`;
  }
  return `id-${Date.now().toString(16)}-${Math.random().toString(16).slice(2)}`;
}
