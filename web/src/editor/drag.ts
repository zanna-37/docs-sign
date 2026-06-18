// clampCenter keeps a box's center so its (unrotated) bounds stay within the page. A box
// larger than the page is centered on that axis.
export function clampCenter(
  cx: number,
  cy: number,
  w: number,
  h: number,
  pageW: number,
  pageH: number,
): { cx: number; cy: number } {
  const hw = Math.min(w, pageW) / 2;
  const hh = Math.min(h, pageH) / 2;
  return {
    cx: Math.max(hw, Math.min(pageW - hw, cx)),
    cy: Math.max(hh, Math.min(pageH - hh, cy)),
  };
}

// rotate applies a clockwise rotation (screen coordinates, y-down) by deg degrees.
export function rotate(x: number, y: number, deg: number) {
  const r = (deg * Math.PI) / 180;
  const c = Math.cos(r);
  const s = Math.sin(r);
  return { x: x * c - y * s, y: x * s + y * c };
}

// beginDrag wires window-level pointer listeners for the duration of a drag gesture.
export function beginDrag(onMove: (e: PointerEvent) => void) {
  const move = (e: PointerEvent) => {
    e.preventDefault();
    onMove(e);
  };
  const up = () => {
    window.removeEventListener("pointermove", move);
    window.removeEventListener("pointerup", up);
  };
  window.addEventListener("pointermove", move);
  window.addEventListener("pointerup", up);
}

export interface Corner {
  sx: 1 | -1;
  sy: 1 | -1;
  pos: React.CSSProperties;
  cursor: string;
}

export const CORNERS: Corner[] = [
  { sx: -1, sy: -1, pos: { left: -6, top: -6 }, cursor: "nwse-resize" },
  { sx: 1, sy: -1, pos: { right: -6, top: -6 }, cursor: "nesw-resize" },
  { sx: -1, sy: 1, pos: { left: -6, bottom: -6 }, cursor: "nesw-resize" },
  { sx: 1, sy: 1, pos: { right: -6, bottom: -6 }, cursor: "nwse-resize" },
];
