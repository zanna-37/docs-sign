export type TextFamily = "sans" | "serif" | "mono";

// TextBox: center-based in PDF points, like signature placements. w/h are derived from the
// text + font so the box hugs the text; resizing scales the font.
export interface TextBox {
  id: string;
  page: number;
  cx: number;
  cy: number;
  w: number;
  h: number;
  rotation: number;
  text: string;
  fontSize: number; // points
  color: string;
  family: TextFamily;
  bold: boolean;
}

export const TEXT_PAD = 3; // points of padding inside the box
export const LINE_HEIGHT = 1.3;

export function cssFamily(family: TextFamily): string {
  switch (family) {
    case "serif":
      return 'Georgia, "Times New Roman", serif';
    case "mono":
      return 'ui-monospace, "Courier New", monospace';
    default:
      return 'system-ui, -apple-system, "Segoe UI", Arial, sans-serif';
  }
}

function fontString(tb: Pick<TextBox, "fontSize" | "family" | "bold">): string {
  return `${tb.bold ? "bold " : ""}${tb.fontSize}px ${cssFamily(tb.family)}`;
}

let shared: HTMLCanvasElement | null = null;
function measureCtx(): CanvasRenderingContext2D {
  if (!shared) shared = document.createElement("canvas");
  return shared.getContext("2d")!;
}

// measureBox returns the box size (points) needed to fit the text at its font size. The DOM
// preview and the export canvas use the same font, so all three agree.
export function measureBox(
  tb: Pick<TextBox, "text" | "fontSize" | "family" | "bold">,
): { w: number; h: number } {
  const ctx = measureCtx();
  ctx.font = fontString(tb);
  const lines = (tb.text || " ").split("\n");
  let maxW = 0;
  for (const line of lines) {
    const m = ctx.measureText(line.length ? line : " ").width;
    if (m > maxW) maxW = m;
  }
  const w = Math.ceil(maxW) + TEXT_PAD * 2;
  const h = Math.ceil(lines.length * tb.fontSize * LINE_HEIGHT) + TEXT_PAD * 2;
  return { w: Math.max(w, tb.fontSize), h: Math.max(h, tb.fontSize) };
}

export function newTextBox(
  id: string,
  page: number,
  cx: number,
  cy: number,
  text: string,
): TextBox {
  const base = { text, fontSize: 16, family: "sans" as TextFamily, bold: false, color: "#111827" };
  const { w, h } = measureBox(base);
  return { id, page, cx, cy, rotation: 0, w, h, ...base };
}

// Recompute w/h after a text/font change, keeping the center fixed.
export function refit(tb: TextBox): TextBox {
  const { w, h } = measureBox(tb);
  return { ...tb, w, h };
}

// renderPng rasterizes the text box to a base64 PNG data URL at pxPerPt pixels per point.
export function renderTextPng(tb: TextBox, pxPerPt: number): string {
  const canvas = document.createElement("canvas");
  canvas.width = Math.max(1, Math.round(tb.w * pxPerPt));
  canvas.height = Math.max(1, Math.round(tb.h * pxPerPt));
  const ctx = canvas.getContext("2d")!;
  ctx.scale(pxPerPt, pxPerPt);
  ctx.textBaseline = "top";
  ctx.fillStyle = tb.color;
  ctx.font = fontString(tb);
  const lines = tb.text.split("\n");
  const lh = tb.fontSize * LINE_HEIGHT;
  // Match the browser: text within a line box sits below half of the line's leading.
  const halfLeading = (lh - tb.fontSize) / 2;
  lines.forEach((line, i) =>
    ctx.fillText(line, TEXT_PAD, TEXT_PAD + halfLeading + i * lh),
  );
  return canvas.toDataURL("image/png");
}
