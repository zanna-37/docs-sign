import { useLayoutEffect, useRef } from "react";
import { beginDrag, CORNERS, rotate } from "./drag";
import { cssFamily, LINE_HEIGHT, refit, TEXT_PAD, type TextBox } from "./text";

interface Props {
  box: TextBox;
  scale: number;
  selected: boolean;
  editing: boolean;
  toPoint: (clientX: number, clientY: number) => { x: number; y: number };
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
  toPoint,
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
    const start = toPoint(e.clientX, e.clientY);
    const c0 = { cx: b.cx, cy: b.cy };
    beginDrag((ev) => {
      const cur = toPoint(ev.clientX, ev.clientY);
      onChange({ ...b, cx: c0.cx + (cur.x - start.x), cy: c0.cy + (cur.y - start.y) });
    });
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
      const factor = w / w0;
      const fontSize = Math.max(6, fs0 * factor);
      const half = rotate((sx * w) / 2, (sy * h) / 2, theta);
      onChange({ ...b, w, h, fontSize, cx: A.x + half.x, cy: A.y + half.y });
    });
  };

  const onRotateStart = (e: React.PointerEvent) => {
    e.stopPropagation();
    onSelect();
    beginDrag((ev) => {
      const P = toPoint(ev.clientX, ev.clientY);
      let deg = (Math.atan2(P.y - b.cy, P.x - b.cx) * 180) / Math.PI + 90;
      deg = ((deg % 360) + 360) % 360;
      onChange({ ...b, rotation: deg });
    });
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
        <div style={textStyle} className="pointer-events-none select-none">
          {b.text || " "}
        </div>
      )}

      {selected && !editing && (
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
          >
            ×
          </button>
        </>
      )}
    </div>
  );
}
