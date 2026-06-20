// ResolveMove maps a pointer position (and the grab offset) to a target page and clamped
// center, so a box can be dragged across pages.
export type ResolveMove = (
  clientX: number,
  clientY: number,
  grabX: number,
  grabY: number,
  w: number,
  h: number,
) => { page: number; cx: number; cy: number };

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

// normalizeDeg wraps an angle into the [0, 360) range.
export function normalizeDeg(deg: number): number {
  return ((deg % 360) + 360) % 360;
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

// ToPoint maps a client (screen) position to this page's coordinate space (PDF points).
export type ToPoint = (clientX: number, clientY: number) => { x: number; y: number };

// BoxGeom is the position/orientation that the move and rotate gestures read and write; any
// editor box (signature placement or text box) satisfies it.
export interface BoxGeom {
  page: number;
  cx: number;
  cy: number;
  w: number;
  h: number;
  rotation: number;
}

// beginMove drags a box across the page (and between pages), writing the resolved page and
// clamped center back through onChange for the duration of the gesture.
export function beginMove<T extends BoxGeom>(
  box: T,
  startClientX: number,
  startClientY: number,
  toPoint: ToPoint,
  resolveMove: ResolveMove,
  onChange: (next: T) => void,
) {
  // Grab offset from the pointer to the box center (in this page's points).
  const start = toPoint(startClientX, startClientY);
  const grabX = box.cx - start.x;
  const grabY = box.cy - start.y;
  beginDrag((ev) => {
    const r = resolveMove(ev.clientX, ev.clientY, grabX, grabY, box.w, box.h);
    onChange({ ...box, page: r.page, cx: r.cx, cy: r.cy });
  });
}

// beginRotate spins a box about its center to follow the pointer for the gesture's duration.
export function beginRotate<T extends BoxGeom>(
  box: T,
  toPoint: ToPoint,
  onChange: (next: T) => void,
) {
  beginDrag((ev) => {
    const P = toPoint(ev.clientX, ev.clientY);
    const deg = normalizeDeg((Math.atan2(P.y - box.cy, P.x - box.cx) * 180) / Math.PI + 90);
    onChange({ ...box, rotation: deg });
  });
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

// Keyboard nudge steps for a focused box: translation (points), resize (points) and rotation
// (degrees). They mirror, in discrete increments, the gestures the pointer handles provide.
export const MOVE_STEP = 2;
export const RESIZE_STEP = 4;
export const ROTATE_STEP = 2;

// arrowDelta maps an arrow key to a unit step in page space (y points downward); it returns
// null for any other key.
export function arrowDelta(key: string): { x: number; y: number } | null {
  switch (key) {
    case "ArrowLeft":
      return { x: -1, y: 0 };
    case "ArrowRight":
      return { x: 1, y: 0 };
    case "ArrowUp":
      return { x: 0, y: -1 };
    case "ArrowDown":
      return { x: 0, y: 1 };
    default:
      return null;
  }
}

// applyBoxKey translates a keydown on a focused box into the matching editor gesture, the
// keyboard twin of the pointer handlers: arrows move, shift + arrows resize, "[" / "]" rotate,
// Delete/Backspace removes, Enter/Space activates. Move, rotate and delete are shared here;
// resize (geometry vs font) and activate (select vs select-and-edit) differ per box type, so
// the caller supplies those. Returns true when the key was handled.
export function applyBoxKey<T extends BoxGeom>(
  e: React.KeyboardEvent,
  box: T,
  handlers: {
    onChange: (next: T) => void;
    onDelete: () => void;
    onActivate: () => void;
    onResize: (grow: number, axis: "x" | "y") => void;
  },
): boolean {
  const arrow = arrowDelta(e.key);
  if (arrow) {
    e.preventDefault();
    if (e.shiftKey) {
      handlers.onResize(arrow.x > 0 || arrow.y < 0 ? 1 : -1, arrow.x === 0 ? "y" : "x");
    } else {
      handlers.onChange({ ...box, cx: box.cx + arrow.x * MOVE_STEP, cy: box.cy + arrow.y * MOVE_STEP });
    }
  } else if (e.key === "[" || e.key === "]") {
    e.preventDefault();
    handlers.onChange({ ...box, rotation: normalizeDeg(box.rotation + (e.key === "[" ? -ROTATE_STEP : ROTATE_STEP)) });
  } else if (e.key === "Delete" || e.key === "Backspace") {
    e.preventDefault();
    handlers.onDelete();
  } else if (e.key === "Enter" || e.key === " ") {
    e.preventDefault();
    handlers.onActivate();
  } else {
    return false;
  }
  return true;
}
