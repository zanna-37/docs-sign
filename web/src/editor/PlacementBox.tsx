import type { CSSProperties } from "react";
import type { Placement } from "./types";

interface Corner {
  sx: 1 | -1;
  sy: 1 | -1;
  pos: CSSProperties;
  cursor: string;
}

const CORNERS: Corner[] = [
  { sx: -1, sy: -1, pos: { left: -6, top: -6 }, cursor: "nwse-resize" },
  { sx: 1, sy: -1, pos: { right: -6, top: -6 }, cursor: "nesw-resize" },
  { sx: -1, sy: 1, pos: { left: -6, bottom: -6 }, cursor: "nesw-resize" },
  { sx: 1, sy: 1, pos: { right: -6, bottom: -6 }, cursor: "nwse-resize" },
];

// rotate applies a clockwise rotation (screen coordinates, y-down) by deg degrees.
function rotate(x: number, y: number, deg: number) {
  const r = (deg * Math.PI) / 180;
  const c = Math.cos(r);
  const s = Math.sin(r);
  return { x: x * c - y * s, y: x * s + y * c };
}

interface Props {
  placement: Placement;
  scale: number;
  selected: boolean;
  imageUrl: string;
  toPoint: (clientX: number, clientY: number) => { x: number; y: number };
  onSelect: () => void;
  onChange: (p: Placement) => void;
  onDelete: () => void;
}

// beginDrag wires window-level pointer listeners for the duration of a drag gesture.
function beginDrag(onMove: (e: PointerEvent) => void) {
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

export function PlacementBox({
  placement: p,
  scale,
  selected,
  imageUrl,
  toPoint,
  onSelect,
  onChange,
  onDelete,
}: Props) {
  const onMoveStart = (e: React.PointerEvent) => {
    e.stopPropagation();
    onSelect();
    const start = toPoint(e.clientX, e.clientY);
    const c0 = { cx: p.cx, cy: p.cy };
    beginDrag((ev) => {
      const cur = toPoint(ev.clientX, ev.clientY);
      onChange({ ...p, cx: c0.cx + (cur.x - start.x), cy: c0.cy + (cur.y - start.y) });
    });
  };

  const onResizeStart = (e: React.PointerEvent, sx: number, sy: number) => {
    e.stopPropagation();
    onSelect();
    const theta = p.rotation;
    const w0 = p.w;
    const h0 = p.h;
    // The opposite corner stays anchored while dragging this corner.
    const aLocal = rotate((-sx * w0) / 2, (-sy * h0) / 2, theta);
    const A = { x: p.cx + aLocal.x, y: p.cy + aLocal.y };
    beginDrag((ev) => {
      const P = toPoint(ev.clientX, ev.clientY);
      const local = rotate(P.x - A.x, P.y - A.y, -theta);
      const w = Math.max(12, local.x * sx);
      const h = Math.max(12, local.y * sy);
      const half = rotate((sx * w) / 2, (sy * h) / 2, theta);
      onChange({ ...p, w, h, cx: A.x + half.x, cy: A.y + half.y });
    });
  };

  const onRotateStart = (e: React.PointerEvent) => {
    e.stopPropagation();
    onSelect();
    beginDrag((ev) => {
      const P = toPoint(ev.clientX, ev.clientY);
      let deg = (Math.atan2(P.y - p.cy, P.x - p.cx) * 180) / Math.PI + 90;
      deg = ((deg % 360) + 360) % 360;
      onChange({ ...p, rotation: deg });
    });
  };

  const left = (p.cx - p.w / 2) * scale;
  const top = (p.cy - p.h / 2) * scale;
  const width = p.w * scale;
  const height = p.h * scale;

  return (
    <div
      style={{
        position: "absolute",
        left,
        top,
        width,
        height,
        transform: `rotate(${p.rotation}deg)`,
        transformOrigin: "center center",
        touchAction: "none",
      }}
      onPointerDown={onMoveStart}
      className={
        selected
          ? "cursor-move outline outline-2 outline-blue-500"
          : "cursor-move outline-1 outline-blue-300 hover:outline"
      }
    >
      <img
        src={imageUrl}
        draggable={false}
        style={{ width: "100%", height: "100%", objectFit: "fill" }}
        className="pointer-events-none select-none"
        alt=""
      />
      {selected && (
        <>
          {CORNERS.map((c, i) => (
            <div
              key={i}
              onPointerDown={(e) => onResizeStart(e, c.sx, c.sy)}
              style={{ position: "absolute", ...c.pos, cursor: c.cursor, touchAction: "none" }}
              className="h-3 w-3 rounded-full border border-blue-500 bg-white"
            />
          ))}
          <div
            onPointerDown={onRotateStart}
            style={{ position: "absolute", left: "50%", top: -30, transform: "translateX(-50%)", touchAction: "none" }}
            className="flex h-6 w-6 cursor-grab items-center justify-center rounded-full border border-blue-500 bg-white text-xs text-blue-600"
            title="Rotate"
          >
            ⟳
          </div>
          <button
            onPointerDown={(e) => e.stopPropagation()}
            onClick={onDelete}
            style={{ position: "absolute", right: -10, top: -10 }}
            className="flex h-5 w-5 items-center justify-center rounded-full bg-red-500 text-xs text-white"
            title="Remove"
          >
            ×
          </button>
        </>
      )}
    </div>
  );
}
