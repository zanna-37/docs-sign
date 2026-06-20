import { SignatureCanvas } from "../components/SignatureCanvas";
import { beginDrag, beginMove, beginRotate, rotate, type ResolveMove } from "./drag";
import { SelectionHandles } from "./SelectionHandles";
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
}: Readonly<Props>) {
  const onMoveStart = (e: React.PointerEvent) => {
    e.stopPropagation();
    onSelect();
    beginMove(p, e.clientX, e.clientY, toPoint, resolveMove, onChange);
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
    beginRotate(p, toPoint, onChange);
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
        <SelectionHandles
          onResizeStart={onResizeStart}
          onRotateStart={onRotateStart}
          onDelete={onDelete}
        />
      )}
    </div>
  );
}
