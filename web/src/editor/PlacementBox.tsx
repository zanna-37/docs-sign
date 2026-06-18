import { SignatureCanvas } from "../components/SignatureCanvas";
import { beginDrag, CORNERS, rotate, type ResolveMove } from "./drag";
import type { Placement } from "./types";

interface Props {
  placement: Placement;
  scale: number;
  selected: boolean;
  bitmap: ImageBitmap | null;
  lockAspect: boolean;
  aspect: number; // native signature width/height, used when lockAspect is true
  pageW: number;
  pageH: number;
  toPoint: (clientX: number, clientY: number) => { x: number; y: number };
  resolveMove: ResolveMove;
  onSelect: () => void;
  onChange: (p: Placement) => void;
  onDelete: () => void;
}

export function PlacementBox({
  placement: p,
  scale,
  selected,
  bitmap,
  lockAspect,
  aspect,
  pageW,
  pageH,
  toPoint,
  resolveMove,
  onSelect,
  onChange,
  onDelete,
}: Props) {
  const onMoveStart = (e: React.PointerEvent) => {
    e.stopPropagation();
    onSelect();
    // Grab offset from the pointer to the box center (in this page's points).
    const start = toPoint(e.clientX, e.clientY);
    const grabX = p.cx - start.x;
    const grabY = p.cy - start.y;
    beginDrag((ev) => {
      const r = resolveMove(ev.clientX, ev.clientY, grabX, grabY, p.w, p.h);
      onChange({ ...p, page: r.page, cx: r.cx, cy: r.cy });
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
      let w = Math.max(12, local.x * sx);
      let h = Math.max(12, local.y * sy);
      if (lockAspect && aspect > 0) {
        // Constrain to the native ratio, driven by whichever axis the user pulls more.
        if (w / aspect >= h) {
          h = w / aspect;
        } else {
          w = h * aspect;
        }
      }
      // Cap the size so the box can't grow past the page: the distance from the anchored
      // corner A to the page edge bounds each dimension.
      const maxW = sx > 0 ? pageW - A.x : A.x;
      const maxH = sy > 0 ? pageH - A.y : A.y;
      if (lockAspect && aspect > 0) {
        const s = Math.min(1, maxW / w, maxH / h);
        w *= s;
        h *= s;
      } else {
        w = Math.min(w, maxW);
        h = Math.min(h, maxH);
      }
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
      <SignatureCanvas
        bitmap={bitmap}
        style={{ width: "100%", height: "100%", objectFit: "fill" }}
        className="pointer-events-none select-none"
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
