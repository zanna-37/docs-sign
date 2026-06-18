import { useEffect, useRef } from "react";
import type { PDFDocumentProxy } from "pdfjs-dist";
import type { PageSize } from "./pdf";
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
  onPlace,
  onSelect,
  onChange,
  onDelete,
  onTextChange,
  onTextDelete,
  onStartEditText,
  onStopEditText,
}: Props) {
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
        ref={overlayRef}
        className={`absolute inset-0 ${armed ? "cursor-copy" : ""}`}
        onPointerDown={onOverlayPointerDown}
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
            toPoint={toPoint}
            onSelect={() => onSelect(p.id)}
            onChange={onChange}
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
            toPoint={toPoint}
            onSelect={() => onSelect(b.id)}
            onChange={onTextChange}
            onStartEdit={() => onStartEditText(b.id)}
            onStopEdit={onStopEditText}
            onDelete={() => onTextDelete(b.id)}
          />
        ))}
      </div>
    </div>
  );
}
