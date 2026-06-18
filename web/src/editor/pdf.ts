import * as pdfjsLib from "pdfjs-dist";
import workerUrl from "pdfjs-dist/build/pdf.worker.min.mjs?url";
import type { PDFDocumentProxy } from "pdfjs-dist";

pdfjsLib.GlobalWorkerOptions.workerSrc = workerUrl;

export interface PageSize {
  widthPt: number;
  heightPt: number;
}

// loadPdf opens a PDF from in-memory bytes (fetched with no-store by the caller) and
// returns the document plus each page's size in points (pdf.js scale 1 => 1 unit = 1
// point). Passing bytes rather than a URL keeps the plaintext PDF out of the HTTP cache.
export async function loadPdf(
  data: ArrayBuffer,
): Promise<{ doc: PDFDocumentProxy; pages: PageSize[]; destroy: () => void }> {
  const task = pdfjsLib.getDocument({ data: new Uint8Array(data) });
  const doc = await task.promise;
  const pages: PageSize[] = [];
  for (let i = 1; i <= doc.numPages; i++) {
    const page = await doc.getPage(i);
    const vp = page.getViewport({ scale: 1 });
    pages.push({ widthPt: vp.width, heightPt: vp.height });
    page.cleanup();
  }
  // destroy() lives on the loading task and tears down the worker + document.
  return { doc, pages, destroy: () => void task.destroy() };
}

export async function renderPageToCanvas(
  doc: PDFDocumentProxy,
  pageNumber1: number,
  scale: number,
  canvas: HTMLCanvasElement,
): Promise<void> {
  const page = await doc.getPage(pageNumber1);
  const vp = page.getViewport({ scale });
  canvas.width = Math.floor(vp.width);
  canvas.height = Math.floor(vp.height);
  await page.render({ canvas, viewport: vp }).promise;
  page.cleanup();
}
