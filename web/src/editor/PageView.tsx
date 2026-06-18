import { useEffect, useRef } from "react";
import type { PDFDocumentProxy } from "pdfjs-dist";
import type { PageSize } from "./pdf";
import type { Placement } from "./types";
import { PlacementBox } from "./PlacementBox";

interface Props {
  doc: PDFDocumentProxy;
  pageIndex: number;
  size: PageSize;
  scale: number;
  placements: Placement[];
  selectedId: string | null;
  bitmapFor: (signatureId: string) => ImageBitmap | null;
  armed: boolean;
  onPlace: (pageIndex: number, point: { x: number; y: number }) => void;
  onSelect: (id: string | null) => void;
  onChange: (p: Placement) => void;
  onDelete: (id: string) => void;
}

export function PageView({
  doc,
  pageIndex,
  size,
  scale,
  placements,
  selectedId,
  bitmapFor,
  armed,
  onPlace,
  onSelect,
  onChange,
  onDelete,
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
            toPoint={toPoint}
            onSelect={() => onSelect(p.id)}
            onChange={onChange}
            onDelete={() => onDelete(p.id)}
          />
        ))}
      </div>
    </div>
  );
}
