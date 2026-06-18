import { useEffect, useLayoutEffect, useRef } from "react";
import { beginDrag, beginMove, beginRotate, rotate, type ResolveMove } from "./drag";
import { SelectionHandles } from "./SelectionHandles";
import { cssFamily, drawText, LINE_HEIGHT, refit, TEXT_PAD, type TextBox } from "./text";

// TextDisplay renders the text on a canvas using the same routine as the export, so the
// on-page preview position matches the flattened output exactly.
function TextDisplay({ box, scale }: { box: TextBox; scale: number }) {
  const ref = useRef<HTMLCanvasElement>(null);
  useEffect(() => {
    const c = ref.current;
    if (!c) return;
    const pxPerPt = scale * (window.devicePixelRatio || 1);
    c.width = Math.max(1, Math.round(box.w * pxPerPt));
    c.height = Math.max(1, Math.round(box.h * pxPerPt));
    const ctx = c.getContext("2d");
    if (!ctx) return;
    ctx.scale(pxPerPt, pxPerPt);
    drawText(ctx, box);
  }, [box, scale]);
  return (
    <canvas
      ref={ref}
      className="pointer-events-none select-none"
      style={{ width: "100%", height: "100%", display: "block" }}
    />
  );
}

interface Props {
  box: TextBox;
  scale: number;
  selected: boolean;
  editing: boolean;
  pageW: number;
  pageH: number;
  toPoint: (clientX: number, clientY: number) => { x: number; y: number };
  resolveMove: ResolveMove;
  onSelect: () => void;
  onChange: (b: TextBox) => void;
  onStartEdit: () => void;
  onStopEdit: () => void;
  onDelete: () => void;
}

export function TextBoxItem({
  box: b,
  scale,
  selected,
  editing,
  pageW,
  pageH,
  toPoint,
  resolveMove,
  onSelect,
  onChange,
  onStartEdit,
  onStopEdit,
  onDelete,
}: Props) {
  const taRef = useRef<HTMLTextAreaElement>(null);

  // Focus and select all when entering edit mode (so typing replaces the default text).
  useLayoutEffect(() => {
    if (editing && taRef.current) {
      const ta = taRef.current;
      ta.focus();
      ta.select();
    }
  }, [editing]);

  const onMoveStart = (e: React.PointerEvent) => {
    e.stopPropagation();
    onSelect();
    beginMove(b, e.clientX, e.clientY, toPoint, resolveMove, onChange);
  };

  const onResizeStart = (e: React.PointerEvent, sx: number, sy: number) => {
    e.stopPropagation();
    onSelect();
    const theta = b.rotation;
    const w0 = b.w;
    const h0 = b.h;
    const fs0 = b.fontSize;
    const aspect = w0 / h0;
    const aLocal = rotate((-sx * w0) / 2, (-sy * h0) / 2, theta);
    const A = { x: b.cx + aLocal.x, y: b.cy + aLocal.y };
    beginDrag((ev) => {
      const P = toPoint(ev.clientX, ev.clientY);
      const local = rotate(P.x - A.x, P.y - A.y, -theta);
      let w = Math.max(12, local.x * sx);
      let h = Math.max(12, local.y * sy);
      if (w / aspect >= h) h = w / aspect;
      else w = h * aspect;
      // Cap so the text box can't grow past the page (anchored at A); the font scales with it.
      const maxW = sx > 0 ? pageW - A.x : A.x;
      const maxH = sy > 0 ? pageH - A.y : A.y;
      const s = Math.min(1, maxW / w, maxH / h);
      w *= s;
      h *= s;
      const factor = w / w0;
      const fontSize = Math.max(6, fs0 * factor);
      const half = rotate((sx * w) / 2, (sy * h) / 2, theta);
      onChange({ ...b, w, h, fontSize, cx: A.x + half.x, cy: A.y + half.y });
    });
  };

  const onRotateStart = (e: React.PointerEvent) => {
    e.stopPropagation();
    onSelect();
    beginRotate(b, toPoint, onChange);
  };

  const left = (b.cx - b.w / 2) * scale;
  const top = (b.cy - b.h / 2) * scale;
  const width = b.w * scale;
  const height = b.h * scale;

  const textStyle: React.CSSProperties = {
    width: "100%",
    height: "100%",
    fontFamily: cssFamily(b.family),
    fontSize: b.fontSize * scale,
    fontWeight: b.bold ? 700 : 400,
    color: b.color,
    lineHeight: LINE_HEIGHT,
    whiteSpace: "pre",
    padding: TEXT_PAD * scale,
    boxSizing: "border-box",
    overflow: "hidden",
  };

  return (
    <div
      style={{
        position: "absolute",
        left,
        top,
        width,
        height,
        transform: `rotate(${b.rotation}deg)`,
        transformOrigin: "center center",
        touchAction: "none",
      }}
      onPointerDown={editing ? (e) => e.stopPropagation() : onMoveStart}
      onDoubleClick={(e) => {
        e.stopPropagation();
        onSelect();
        onStartEdit();
      }}
      className={
        editing
          ? "cursor-text outline outline-2 outline-blue-500"
          : selected
            ? "cursor-move outline outline-2 outline-blue-500"
            : "cursor-move outline-1 outline-dashed outline-blue-300 hover:outline"
      }
    >
      {editing ? (
        <textarea
          ref={taRef}
          value={b.text}
          wrap="off"
          spellCheck={false}
          onPointerDown={(e) => e.stopPropagation()}
          onChange={(e) => onChange(refit({ ...b, text: e.target.value }))}
          onKeyDown={(e) => {
            e.stopPropagation();
            if (e.key === "Escape") {
              e.preventDefault();
              onStopEdit();
            }
          }}
          style={{
            ...textStyle,
            border: "none",
            outline: "none",
            background: "transparent",
            resize: "none",
          }}
        />
      ) : (
        <TextDisplay box={b} scale={scale} />
      )}

      {selected && !editing && (
        <SelectionHandles
          onResizeStart={onResizeStart}
          onRotateStart={onRotateStart}
          onDelete={onDelete}
        />
      )}
    </div>
  );
}
