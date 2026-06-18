import { useEffect, useRef, type CSSProperties } from "react";

// SignatureCanvas paints an ImageBitmap onto a canvas. The signature is never an <img> or
// object URL — only canvas pixels — so it isn't a saveable/cacheable browser resource. The
// drawing buffer is the bitmap's natural size; CSS (object-fit) scales it to the box.
export function SignatureCanvas({
  bitmap,
  className,
  style,
  ariaLabel,
}: {
  bitmap: ImageBitmap | null;
  className?: string;
  style?: CSSProperties;
  ariaLabel?: string;
}) {
  const ref = useRef<HTMLCanvasElement>(null);
  useEffect(() => {
    const canvas = ref.current;
    if (!canvas || !bitmap) return;
    canvas.width = bitmap.width;
    canvas.height = bitmap.height;
    const ctx = canvas.getContext("2d");
    if (ctx) {
      ctx.clearRect(0, 0, canvas.width, canvas.height);
      ctx.drawImage(bitmap, 0, 0);
    }
  }, [bitmap]);
  return (
    <canvas
      ref={ref}
      className={className}
      style={style}
      role="img"
      aria-label={ariaLabel}
    />
  );
}
