import { useEffect, useMemo, useRef, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import type { PDFDocumentProxy } from "pdfjs-dist";
import { api, ApiError } from "../api/client";
import type {
  DocumentItem,
  ExportItem,
  PlacementInput,
  Signature,
} from "../api/types";
import { loadPdf, type PageSize } from "../editor/pdf";
import { fetchArrayBuffer } from "../lib/blobUrls";
import { useSignatureBitmaps } from "../lib/signatureBitmaps";
import type { Placement, SignatureMeta } from "../editor/types";
import { PageView } from "../editor/PageView";
import { SignatureCanvas } from "../components/SignatureCanvas";
import { Button, ErrorText, Modal, Spinner } from "../components/ui";

const TARGET_WIDTH = 720;

const checkerStyle: React.CSSProperties = {
  backgroundColor: "#fff",
  backgroundImage:
    "linear-gradient(45deg,#eee 25%,transparent 25%),linear-gradient(-45deg,#eee 25%,transparent 25%),linear-gradient(45deg,transparent 75%,#eee 75%),linear-gradient(-45deg,transparent 75%,#eee 75%)",
  backgroundSize: "12px 12px",
  backgroundPosition: "0 0,0 6px,6px -6px,-6px 0",
};

export function EditorPage() {
  const { id = "" } = useParams();
  const navigate = useNavigate();
  const destroyRef = useRef<(() => void) | null>(null);

  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [docName, setDocName] = useState("");
  const [doc, setDoc] = useState<PDFDocumentProxy | null>(null);
  const [pages, setPages] = useState<PageSize[]>([]);
  const [scale, setScale] = useState(1);
  const [signatures, setSignatures] = useState<SignatureMeta[]>([]);
  const [placements, setPlacements] = useState<Placement[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [armed, setArmed] = useState<string | null>(null);
  const [exporting, setExporting] = useState(false);
  const [done, setDone] = useState<ExportItem | null>(null);

  useEffect(() => {
    let active = true;
    (async () => {
      try {
        const [docList, sigRes] = await Promise.all([
          api.get<{ documents: DocumentItem[] }>("/documents"),
          api.get<{ signatures: Signature[] }>("/signatures"),
        ]);
        if (!active) return;
        const meta = docList.documents?.find((d) => d.id === id);
        setDocName(meta?.name ?? "Document");
        setSignatures(
          (sigRes.signatures ?? []).map((s) => ({
            id: s.id,
            name: s.name,
            width: s.width,
            height: s.height,
          })),
        );

        const pdfBytes = await fetchArrayBuffer(sigDocUrl(id));
        const loaded = await loadPdf(pdfBytes);
        if (!active) {
          loaded.destroy();
          return;
        }
        destroyRef.current = loaded.destroy;
        setDoc(loaded.doc);
        setPages(loaded.pages);
        const maxW = Math.max(...loaded.pages.map((p) => p.widthPt), 1);
        setScale(Math.min(2, Math.max(0.4, TARGET_WIDTH / maxW)));
      } catch (err) {
        if (active)
          setError(
            err instanceof ApiError ? err.message : "Failed to open document.",
          );
      } finally {
        if (active) setLoading(false);
      }
    })();
    return () => {
      active = false;
    };
  }, [id]);

  useEffect(
    () => () => {
      destroyRef.current?.();
    },
    [],
  );

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setArmed(null);
      if ((e.key === "Delete" || e.key === "Backspace") && selectedId) {
        const tag = (e.target as HTMLElement)?.tagName;
        if (tag === "INPUT" || tag === "TEXTAREA") return;
        setPlacements((ps) => ps.filter((p) => p.id !== selectedId));
        setSelectedId(null);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [selectedId]);

  const sigMap = useMemo(
    () => new Map(signatures.map((s) => [s.id, s])),
    [signatures],
  );

  // Signatures are decoded into in-memory bitmaps (no-store fetch, no object URL) and
  // shared by both the palette and the placed boxes; they render only as canvas pixels.
  const sigIds = useMemo(() => signatures.map((s) => s.id), [signatures]);
  const sigBitmaps = useSignatureBitmaps(sigIds);

  const onPlace = (pageIndex: number, point: { x: number; y: number }) => {
    if (!armed) return;
    const meta = sigMap.get(armed);
    if (!meta) return;
    const w = 180;
    const aspect = meta.width > 0 ? meta.height / meta.width : 0.4;
    const np: Placement = {
      id: crypto.randomUUID(),
      signatureId: armed,
      page: pageIndex,
      cx: point.x,
      cy: point.y,
      w,
      h: w * aspect,
      rotation: 0,
    };
    setPlacements((ps) => [...ps, np]);
    setSelectedId(np.id);
    setArmed(null);
  };

  const onChange = (p: Placement) =>
    setPlacements((ps) => ps.map((x) => (x.id === p.id ? p : x)));

  const onDelete = (pid: string) => {
    setPlacements((ps) => ps.filter((p) => p.id !== pid));
    setSelectedId(null);
  };

  const doExport = async () => {
    setExporting(true);
    setError("");
    try {
      const input: PlacementInput[] = placements.map((p) => ({
        signatureId: p.signatureId,
        page: p.page,
        x: p.cx - p.w / 2,
        y: p.cy - p.h / 2,
        w: p.w,
        h: p.h,
        rotation: p.rotation,
      }));
      const exp = await api.post<ExportItem>(`/documents/${id}/sign`, {
        placements: input,
      });
      setDone(exp);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Export failed.");
    } finally {
      setExporting(false);
    }
  };

  if (loading) {
    return (
      <div className="flex justify-center py-20">
        <Spinner className="h-8 w-8" />
      </div>
    );
  }

  if (error && !doc) {
    return (
      <div className="space-y-4">
        <ErrorText>{error}</ErrorText>
        <Button variant="secondary" onClick={() => navigate("/documents")}>
          ← Back to documents
        </Button>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-3">
          <Button variant="secondary" onClick={() => navigate("/documents")}>
            ← Back
          </Button>
          <h1 className="truncate text-lg font-semibold text-gray-900">
            {docName}
          </h1>
        </div>
        <div className="flex items-center gap-3">
          <span className="text-sm text-gray-500">
            {placements.length} placed
          </span>
          <Button onClick={doExport} disabled={exporting || placements.length === 0}>
            {exporting ? "Flattening…" : "Export signed PDF"}
          </Button>
        </div>
      </div>

      {error && <ErrorText>{error}</ErrorText>}

      {armed && (
        <div className="rounded-lg bg-blue-50 px-4 py-2 text-sm text-blue-700">
          Click on a page to place the signature. Press Esc to cancel.
        </div>
      )}

      <div className="grid grid-cols-1 gap-6 md:grid-cols-[220px_1fr]">
        <aside className="space-y-3">
          <h2 className="text-sm font-medium text-gray-700">Your signatures</h2>
          {signatures.length === 0 ? (
            <p className="text-sm text-gray-500">
              No signatures yet.{" "}
              <Link to="/signatures" className="text-blue-600">
                Add one
              </Link>
              .
            </p>
          ) : (
            <div className="space-y-2">
              {signatures.map((s) => (
                <button
                  key={s.id}
                  onClick={() => setArmed(armed === s.id ? null : s.id)}
                  className={`flex w-full items-center gap-2 rounded-lg border p-2 text-left transition ${
                    armed === s.id
                      ? "border-blue-500 bg-blue-50"
                      : "border-gray-200 bg-white hover:bg-gray-50"
                  }`}
                >
                  <span
                    className="flex h-10 w-16 shrink-0 items-center justify-center rounded"
                    style={checkerStyle}
                  >
                    {sigBitmaps[s.id] ? (
                      <SignatureCanvas
                        bitmap={sigBitmaps[s.id]}
                        ariaLabel={s.name}
                        className="max-h-full max-w-full object-contain"
                      />
                    ) : (
                      <span className="h-8 w-12 animate-pulse rounded bg-gray-100" />
                    )}
                  </span>
                  <span className="truncate text-sm text-gray-700">
                    {s.name}
                  </span>
                </button>
              ))}
            </div>
          )}
          <p className="pt-2 text-xs text-gray-400">
            Select a signature, click a page to place it, then drag to move,
            pull the corners to resize, or use the top handle to rotate.
          </p>
        </aside>

        <div className="space-y-6 overflow-auto rounded-xl bg-gray-100 p-4 sm:p-6">
          {doc &&
            pages.map((size, i) => (
              <PageView
                key={i}
                doc={doc}
                pageIndex={i}
                size={size}
                scale={scale}
                placements={placements.filter((p) => p.page === i)}
                selectedId={selectedId}
                bitmapFor={(sid) => sigBitmaps[sid] ?? null}
                armed={!!armed}
                onPlace={onPlace}
                onSelect={setSelectedId}
                onChange={onChange}
                onDelete={onDelete}
              />
            ))}
        </div>
      </div>

      {done && (
        <Modal open title="Document signed">
          <div className="space-y-4">
            <p className="text-sm text-gray-600">
              Your flattened, signed PDF was created and stored (encrypted). The
              signature is rasterized into the page and cannot be extracted.
            </p>
            <div className="flex gap-2">
              <a href={`/api/exports/${done.id}/file`} className="flex-1">
                <Button className="w-full">Download</Button>
              </a>
              <Button
                variant="secondary"
                className="flex-1"
                onClick={() => navigate("/history")}
              >
                Go to history
              </Button>
            </div>
            <Button
              variant="ghost"
              className="w-full"
              onClick={() => setDone(null)}
            >
              Keep editing
            </Button>
          </div>
        </Modal>
      )}
    </div>
  );
}

const sigDocUrl = (id: string) => `/api/documents/${id}/file`;
