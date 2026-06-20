import { useEffect, useRef } from "react";
import type { PDFDocumentProxy } from "pdfjs-dist";
import type { PageSize } from "./pdf";
import { clampCenter, type ResolveMove } from "./drag";
import type { Placement } from "./types";
import { PlacementBox } from "./PlacementBox";
import type { TextBox } from "./text";
import { TextBoxItem } from "./TextBoxItem";

interface Props {
  doc: PDFDocumentProxy;
  pageIndex: number;
  size: PageSize;
  scale: number;
  placements: Placement[];
  textboxes: TextBox[];
  selectedId: string | null;
  editingTextId: string | null;
  bitmapFor: (signatureId: string) => ImageBitmap | null;
  aspectFor: (signatureId: string) => number;
  lockAspect: boolean;
  armed: boolean;
  resolveMove: ResolveMove;
  registerOverlay: (el: HTMLDivElement | null) => void;
  onPlace: (pageIndex: number, point: { x: number; y: number }) => void;
  onSelect: (id: string | null) => void;
  onChange: (p: Placement) => void;
  onDelete: (id: string) => void;
  onTextChange: (b: TextBox) => void;
  onTextDelete: (id: string) => void;
  onStartEditText: (id: string) => void;
  onStopEditText: () => void;
}

export function PageView({
  doc,
  pageIndex,
  size,
  scale,
  placements,
  textboxes,
  selectedId,
  editingTextId,
  bitmapFor,
  aspectFor,
  lockAspect,
  armed,
  resolveMove,
  registerOverlay,
  onPlace,
  onSelect,
  onChange,
  onDelete,
  onTextChange,
  onTextDelete,
  onStartEditText,
  onStopEditText,
}: Readonly<Props>) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const overlayRef = useRef<HTMLDivElement>(null);

  // Render the page, cancelling any in-flight render (handles StrictMode + scale changes).
  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    let task: { cancel: () => void; promise: Promise<void> } | null = null;
    let cancelled = false;
    (async () => {
      const page = await doc.getPage(pageIndex + 1);
      if (cancelled) {
        page.cleanup();
        return;
      }
      const vp = page.getViewport({ scale });
      canvas.width = Math.floor(vp.width);
      canvas.height = Math.floor(vp.height);
      task = page.render({ canvas, viewport: vp });
      try {
        await task.promise;
      } catch {
        /* render cancelled */
      }
      page.cleanup();
    })();
    return () => {
      cancelled = true;
      task?.cancel();
    };
  }, [doc, pageIndex, scale]);

  const toPoint = (clientX: number, clientY: number) => {
    const rect = overlayRef.current!.getBoundingClientRect();
    return { x: (clientX - rect.left) / scale, y: (clientY - rect.top) / scale };
  };

  // Clamp resize/rotate to this page. Moves are resolved (and clamped) globally by
  // resolveMove, which may reassign the page — pass those through untouched.
  const clamp = <
    T extends { cx: number; cy: number; w: number; h: number; page: number },
  >(
    o: T,
  ): T => {
    if (o.page !== pageIndex) return o;
    const c = clampCenter(o.cx, o.cy, o.w, o.h, size.widthPt, size.heightPt);
    return { ...o, cx: c.cx, cy: c.cy };
  };

  const onOverlayPointerDown = (e: React.PointerEvent) => {
    if (armed) {
      onPlace(pageIndex, toPoint(e.clientX, e.clientY));
    } else {
      onSelect(null);
    }
  };

  const widthPx = size.widthPt * scale;
  const heightPx = size.heightPt * scale;

  return (
    <div className="relative mx-auto shadow-md ring-1 ring-gray-200" style={{ width: widthPx, height: heightPx }}>
      <canvas ref={canvasRef} className="block bg-white" style={{ width: widthPx, height: heightPx }} />
      <div
        ref={(el) => {
          overlayRef.current = el;
          registerOverlay(el);
        }}
        className={`absolute inset-0 ${armed ? "cursor-copy" : ""}`}
        onPointerDown={onOverlayPointerDown}
        onKeyDown={(e) => {
          // Escape (bubbling from a focused box) deselects. The overlay itself is not a tab
          // stop — keyboard manipulation lives on the focused box; this is a placement surface.
          if (e.key === "Escape" && !editingTextId) onSelect(null);
        }}
      >
        {placements.map((p) => (
          <PlacementBox
            key={p.id}
            placement={p}
            scale={scale}
            selected={p.id === selectedId}
            bitmap={bitmapFor(p.signatureId)}
            lockAspect={lockAspect}
            aspect={aspectFor(p.signatureId)}
            pageW={size.widthPt}
            pageH={size.heightPt}
            toPoint={toPoint}
            resolveMove={resolveMove}
            onSelect={() => onSelect(p.id)}
            onChange={(np) => onChange(clamp(np))}
            onDelete={() => onDelete(p.id)}
          />
        ))}
        {textboxes.map((b) => (
          <TextBoxItem
            key={b.id}
            box={b}
            scale={scale}
            selected={b.id === selectedId}
            editing={b.id === editingTextId}
            pageW={size.widthPt}
            pageH={size.heightPt}
            toPoint={toPoint}
            resolveMove={resolveMove}
            onSelect={() => onSelect(b.id)}
            onChange={(nb) => onTextChange(clamp(nb))}
            onStartEdit={() => onStartEditText(b.id)}
            onStopEdit={onStopEditText}
            onDelete={() => onTextDelete(b.id)}
          />
        ))}
      </div>
    </div>
  );
}
