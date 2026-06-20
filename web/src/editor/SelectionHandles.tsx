import { CORNERS } from "./drag";

interface Props {
  onResizeStart: (e: React.PointerEvent, sx: number, sy: number) => void;
  onRotateStart: (e: React.PointerEvent) => void;
  onDelete: () => void;
}

// SelectionHandles renders the resize dots, rotate handle and delete button drawn over the
// currently selected editor box (signature placement or text box). The resize and rotate dots
// are pointer-only affordances (mouse + touch via pointer events) and are hidden from
// assistive tech, because their functions are fully keyboard-operable from the focused box
// itself (arrow keys move, shift + arrows resize, "[" / "]" rotate, Delete removes).
export function SelectionHandles({ onResizeStart, onRotateStart, onDelete }: Readonly<Props>) {
  return (
    <>
      {CORNERS.map((c, i) => (
        <div
          key={i}
          aria-hidden
          onPointerDown={(e) => onResizeStart(e, c.sx, c.sy)}
          style={{ position: "absolute", ...c.pos, cursor: c.cursor, touchAction: "none" }}
          className="h-3 w-3 rounded-full border border-blue-500 bg-white"
        />
      ))}
      <div
        aria-hidden
        onPointerDown={onRotateStart}
        style={{ position: "absolute", left: "50%", top: -30, transform: "translateX(-50%)", touchAction: "none" }}
        className="flex h-6 w-6 cursor-grab items-center justify-center rounded-full border border-blue-500 bg-white text-xs text-blue-600"
        title="Rotate"
      >
        ⟳
      </div>
      <button
        type="button"
        aria-label="Remove"
        onPointerDown={(e) => e.stopPropagation()}
        onClick={onDelete}
        style={{ position: "absolute", right: -10, top: -10 }}
        className="flex h-5 w-5 items-center justify-center rounded-full bg-red-500 text-xs text-white"
        title="Remove"
      >
        ×
      </button>
    </>
  );
}
