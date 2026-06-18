import { useEffect, useMemo, useRef, useState } from "react";
import { Link, useBlocker, useNavigate, useParams } from "react-router-dom";
import { useTranslation } from "react-i18next";
import type { PDFDocumentProxy } from "pdfjs-dist";
import { api, ApiError } from "../api/client";
import type {
  DocumentItem,
  ExportItem,
  PlacementInput,
  Signature,
} from "../api/types";
import { loadPdf, type PageSize } from "../editor/pdf";
import { clampCenter, type ResolveMove } from "../editor/drag";
import { fetchArrayBuffer } from "../lib/blobUrls";
import { useSignatureBitmaps } from "../lib/signatureBitmaps";
import { uid } from "../lib/uid";
import type { Placement, SignatureMeta } from "../editor/types";
import {
  newTextBox,
  refit,
  renderTextPng,
  type TextBox,
  type TextFamily,
} from "../editor/text";
import { PageView } from "../editor/PageView";
import { SignatureCanvas } from "../components/SignatureCanvas";
import { useDialog } from "../components/Dialog";
import { Button, ErrorText, Modal, Spinner } from "../components/ui";

const TARGET_WIDTH = 720;
const EXPORT_PX_PER_PT = 3; // ~216 DPI text rasterization for crisp output

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
  const { t } = useTranslation();
  const dialog = useDialog();
  const destroyRef = useRef<(() => void) | null>(null);
  const overlaysRef = useRef<Array<HTMLDivElement | null>>([]);

  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [docName, setDocName] = useState("");
  const [doc, setDoc] = useState<PDFDocumentProxy | null>(null);
  const [pages, setPages] = useState<PageSize[]>([]);
  const [scale, setScale] = useState(1);
  const [signatures, setSignatures] = useState<SignatureMeta[]>([]);
  const [placements, setPlacements] = useState<Placement[]>([]);
  const [textboxes, setTextboxes] = useState<TextBox[]>([]);
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [editingTextId, setEditingTextId] = useState<string | null>(null);
  const [armed, setArmed] = useState<string | null>(null); // signature id
  const [armText, setArmText] = useState(false);
  const [lockAspect, setLockAspect] = useState(true);
  const [exporting, setExporting] = useState(false);
  const [confirmExport, setConfirmExport] = useState(false);
  const [notice, setNotice] = useState("");

  useEffect(() => {
    let active = true;
    (async () => {
      try {
        const [docList, sigRes] = await Promise.all([
          api.get<{ documents: DocumentItem[] }>("/documents"),
          api.get<{ signatures: Signature[] }>("/signatures"),
        ]);
        if (!active) return;
        setDocName(docList.documents?.find((d) => d.id === id)?.name ?? "");
        setSignatures(
          (sigRes.signatures ?? []).map((s) => ({
            id: s.id,
            name: s.name,
            width: s.width,
            height: s.height,
          })),
        );
        const pdfBytes = await fetchArrayBuffer(`/api/documents/${id}/file`);
        const loaded = await loadPdf(pdfBytes);
        if (!active) {
          loaded.destroy();
          return;
        }
        destroyRef.current = loaded.destroy;
        setDoc(loaded.doc);
        setPages(loaded.pages);
        const maxW = Math.max(...loaded.pages.map((p) => p.widthPt), 1);
        const target = Math.min(TARGET_WIDTH, (window.innerWidth || 800) - 48);
        setScale(Math.min(2, Math.max(0.35, target / maxW)));
      } catch (err) {
        if (active)
          setError(err instanceof ApiError ? err.message : t("editor.openFailed"));
      } finally {
        if (active) setLoading(false);
      }
    })();
    return () => {
      active = false;
    };
  }, [id, t]);

  useEffect(() => () => destroyRef.current?.(), []);

  // Delete key removes the selected item (signature or text box).
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        setArmed(null);
        setArmText(false);
      }
      if ((e.key === "Delete" || e.key === "Backspace") && selectedId) {
        const tag = (e.target as HTMLElement)?.tagName;
        if (tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT") return;
        setPlacements((ps) => ps.filter((p) => p.id !== selectedId));
        setTextboxes((ts) => ts.filter((b) => b.id !== selectedId));
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
  const sigIds = useMemo(() => signatures.map((s) => s.id), [signatures]);
  const sigBitmaps = useSignatureBitmaps(sigIds);

  const aspectFor = (signatureId: string) => {
    const m = sigMap.get(signatureId);
    return m && m.height > 0 ? m.width / m.height : 1;
  };

  const toggleLock = (v: boolean) => {
    setLockAspect(v);
    if (v) {
      setPlacements((ps) =>
        ps.map((p) => {
          const a = aspectFor(p.signatureId);
          return a > 0 ? { ...p, h: p.w / a } : p;
        }),
      );
    }
  };

  // Map a pointer (during a move) to whichever page it's over, so a box can be dragged
  // across pages. Returns the target page and clamped center.
  const resolveMove: ResolveMove = (clientX, clientY, grabX, grabY, w, h) => {
    let page = 0;
    let best = Infinity;
    for (let i = 0; i < pages.length; i++) {
      const el = overlaysRef.current[i];
      if (!el) continue;
      const r = el.getBoundingClientRect();
      if (clientY >= r.top && clientY <= r.bottom) {
        page = i;
        break;
      }
      const d = clientY < r.top ? r.top - clientY : clientY - r.bottom;
      if (d < best) {
        best = d;
        page = i;
      }
    }
    const el = overlaysRef.current[page];
    if (!el) return { page, cx: grabX, cy: grabY };
    const r = el.getBoundingClientRect();
    const sz = pages[page];
    const x = (clientX - r.left) / scale + grabX;
    const y = (clientY - r.top) / scale + grabY;
    const c = clampCenter(x, y, w, h, sz.widthPt, sz.heightPt);
    return { page, cx: c.cx, cy: c.cy };
  };

  // Selecting something other than the box being edited exits edit mode.
  const select = (id: string | null) => {
    setSelectedId(id);
    if (id !== editingTextId) setEditingTextId(null);
  };

  const onPlace = (pageIndex: number, point: { x: number; y: number }) => {
    const sz = pages[pageIndex];
    const fit = (cx: number, cy: number, w: number, h: number) =>
      clampCenter(cx, cy, w, h, sz.widthPt, sz.heightPt);

    if (armText) {
      const base = newTextBox(uid(), pageIndex, point.x, point.y, t("editor.text.default"));
      const tb = { ...base, ...fit(base.cx, base.cy, base.w, base.h) };
      setTextboxes((ts) => [...ts, tb]);
      setSelectedId(tb.id);
      setEditingTextId(tb.id); // start editing the new box immediately
      setArmText(false);
      return;
    }
    if (!armed) return;
    const meta = sigMap.get(armed);
    if (!meta) return;
    const w = 180;
    const aspect = meta.width > 0 ? meta.height / meta.width : 0.4;
    const h = w * aspect;
    const c = fit(point.x, point.y, w, h);
    const np: Placement = {
      id: uid(),
      signatureId: armed,
      page: pageIndex,
      cx: c.cx,
      cy: c.cy,
      w,
      h,
      rotation: 0,
    };
    setPlacements((ps) => [...ps, np]);
    select(np.id);
    setArmed(null);
  };

  const selectedText = textboxes.find((b) => b.id === selectedId) ?? null;
  const patchText = (patch: Partial<TextBox>, doRefit = true) =>
    setTextboxes((ts) =>
      ts.map((b) => {
        if (b.id !== selectedId) return b;
        const nb = { ...b, ...patch };
        return doRefit ? refit(nb) : nb;
      }),
    );

  const itemCount =
    placements.length + textboxes.filter((b) => b.text.trim()).length;

  // Unsaved-changes guard: dirty when the current layout differs from the last export.
  const snapshot = useMemo(
    () => JSON.stringify({ p: placements, t: textboxes }),
    [placements, textboxes],
  );
  const savedRef = useRef<string | null>(null);
  useEffect(() => {
    if (savedRef.current === null) savedRef.current = snapshot;
  }, [snapshot]);
  const dirty = savedRef.current !== null && snapshot !== savedRef.current;

  const blocker = useBlocker(dirty);
  const leavingRef = useRef(false);
  useEffect(() => {
    if (blocker.state === "blocked" && !leavingRef.current) {
      leavingRef.current = true;
      void dialog
        .confirm({
          title: t("editor.confirmLeave"),
          confirmLabel: t("common.confirm"),
          danger: true,
        })
        .then((ok) => {
          leavingRef.current = false;
          if (ok) blocker.proceed();
          else blocker.reset();
        });
    }
  }, [blocker, dialog, t]);

  useEffect(() => {
    const handler = (e: BeforeUnloadEvent) => {
      if (dirty) {
        e.preventDefault();
        e.returnValue = "";
      }
    };
    window.addEventListener("beforeunload", handler);
    return () => window.removeEventListener("beforeunload", handler);
  }, [dirty]);

  const doExport = async () => {
    setExporting(true);
    setError("");
    try {
      const input: PlacementInput[] = [
        ...placements.map((p) => ({
          signatureId: p.signatureId,
          page: p.page,
          x: p.cx - p.w / 2,
          y: p.cy - p.h / 2,
          w: p.w,
          h: p.h,
          rotation: p.rotation,
        })),
        ...textboxes
          .filter((b) => b.text.trim())
          .map((b) => ({
            imageData: renderTextPng(b, EXPORT_PX_PER_PT),
            page: b.page,
            x: b.cx - b.w / 2,
            y: b.cy - b.h / 2,
            w: b.w,
            h: b.h,
            rotation: b.rotation,
          })),
      ];
      const body: { placements: PlacementInput[]; name?: string } = {
        placements: input,
      };
      if (docName) body.name = t("editor.exportName", { name: docName });
      const exp = await api.post<ExportItem>(`/documents/${id}/sign`, body);
      savedRef.current = snapshot; // exported -> no longer dirty (before the download)
      triggerDownload(exp.id);
      setConfirmExport(false);
      setNotice(t("editor.savedNotice"));
    } catch (err) {
      setError(err instanceof ApiError ? err.message : t("editor.exportFailed"));
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
          ← {t("editor.back")}
        </Button>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-3">
          <Button variant="secondary" onClick={() => navigate("/documents")}>
            ← {t("editor.back")}
          </Button>
          <h1 className="truncate text-lg font-semibold text-gray-900">
            {docName}
          </h1>
        </div>
        <div className="flex items-center gap-3">
          <span className="text-sm text-gray-500">
            {t("editor.placed", { count: itemCount })}
          </span>
          <Button
            onClick={() => {
              setNotice("");
              setConfirmExport(true);
            }}
            disabled={itemCount === 0}
          >
            {t("editor.export")}
          </Button>
        </div>
      </div>

      {error && <ErrorText>{error}</ErrorText>}
      {notice && (
        <p className="rounded-lg bg-green-50 px-4 py-2 text-sm text-green-700">
          {notice}
        </p>
      )}
      {armed && (
        <div className="rounded-lg bg-blue-50 px-4 py-2 text-sm text-blue-700">
          {t("editor.armSignature")}
        </div>
      )}
      {armText && (
        <div className="rounded-lg bg-blue-50 px-4 py-2 text-sm text-blue-700">
          {t("editor.armText")}
        </div>
      )}

      <div className="grid grid-cols-1 gap-6 md:grid-cols-[240px_1fr] md:items-start">
        <aside className="space-y-4 md:sticky md:top-6 md:max-h-[calc(100vh-3rem)] md:self-start md:overflow-y-auto md:pr-1">

          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <h2 className="text-sm font-medium text-gray-700">
                {t("editor.yourSignatures")}
              </h2>
              <Button
                variant="secondary"
                className="px-2 py-1 text-xs"
                onClick={() => {
                  setArmText(true);
                  setArmed(null);
                }}
              >
                + {t("editor.addText")}
              </Button>
            </div>
            {signatures.length === 0 ? (
              <p className="text-sm text-gray-500">
                {t("editor.noSignatures")}{" "}
                <Link to="/signatures" className="text-blue-600">
                  {t("editor.addOne")}
                </Link>
                .
              </p>
            ) : (
              <div className="space-y-2">
                {signatures.map((s) => (
                  <button
                    key={s.id}
                    onClick={() => {
                      setArmed(armed === s.id ? null : s.id);
                      setArmText(false);
                    }}
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
          </div>

          {selectedText && (
            <div className="fixed inset-x-0 bottom-0 z-30 max-h-[60vh] space-y-3 overflow-y-auto border-t border-gray-200 bg-white p-3 shadow-[0_-4px_16px_rgba(0,0,0,0.12)] md:static md:inset-auto md:z-auto md:max-h-none md:overflow-visible md:rounded-lg md:border md:shadow-none">
              <p className="text-xs text-gray-400">{t("editor.text.hint")}</p>
              <div className="grid grid-cols-2 gap-2">
                <label className="text-xs text-gray-600">
                  {t("editor.text.font")}
                  <select
                    value={selectedText.family}
                    onChange={(e) =>
                      patchText({ family: e.target.value as TextFamily })
                    }
                    className="mt-1 w-full rounded border border-gray-300 px-2 py-1 text-sm"
                  >
                    <option value="sans">{t("editor.text.sans")}</option>
                    <option value="serif">{t("editor.text.serif")}</option>
                    <option value="mono">{t("editor.text.mono")}</option>
                  </select>
                </label>
                <label className="text-xs text-gray-600">
                  {t("editor.text.size")}
                  <input
                    type="number"
                    min={6}
                    max={200}
                    value={Math.round(selectedText.fontSize)}
                    onChange={(e) =>
                      patchText({
                        fontSize: Math.max(6, Number(e.target.value) || 6),
                      })
                    }
                    className="mt-1 w-full rounded border border-gray-300 px-2 py-1 text-sm"
                  />
                </label>
              </div>
              <div className="flex items-center justify-between">
                <label className="flex items-center gap-2 text-xs text-gray-600">
                  {t("editor.text.color")}
                  <input
                    type="color"
                    value={selectedText.color}
                    onChange={(e) =>
                      patchText({ color: e.target.value }, false)
                    }
                    className="h-7 w-10 rounded border border-gray-300"
                  />
                </label>
                <label className="flex items-center gap-2 text-xs text-gray-600">
                  <input
                    type="checkbox"
                    checked={selectedText.bold}
                    onChange={(e) => patchText({ bold: e.target.checked })}
                  />
                  {t("editor.text.bold")}
                </label>
              </div>
            </div>
          )}

          <label className="flex items-center gap-2 border-t border-gray-200 pt-3 text-sm text-gray-700">
            <input
              type="checkbox"
              checked={lockAspect}
              onChange={(e) => toggleLock(e.target.checked)}
            />
            {t("editor.lockRatio")}
          </label>
          <p className="pt-1 text-xs text-gray-400">
            {t("editor.tip")}
            {lockAspect ? t("editor.tipLocked") : t("editor.tipFree")}
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
                textboxes={textboxes.filter((b) => b.page === i)}
                selectedId={selectedId}
                editingTextId={editingTextId}
                bitmapFor={(sid) => sigBitmaps[sid] ?? null}
                aspectFor={aspectFor}
                lockAspect={lockAspect}
                armed={!!armed || armText}
                resolveMove={resolveMove}
                registerOverlay={(el) => {
                  overlaysRef.current[i] = el;
                }}
                onPlace={onPlace}
                onSelect={select}
                onChange={(p) =>
                  setPlacements((ps) => ps.map((x) => (x.id === p.id ? p : x)))
                }
                onDelete={(pid) => {
                  setPlacements((ps) => ps.filter((p) => p.id !== pid));
                  setSelectedId(null);
                }}
                onTextChange={(b) =>
                  setTextboxes((ts) => ts.map((x) => (x.id === b.id ? b : x)))
                }
                onTextDelete={(tid) => {
                  setTextboxes((ts) => ts.filter((b) => b.id !== tid));
                  setSelectedId(null);
                  setEditingTextId(null);
                }}
                onStartEditText={(tid) => setEditingTextId(tid)}
                onStopEditText={() => setEditingTextId(null)}
              />
            ))}
        </div>
      </div>

      {confirmExport && (
        <Modal open title={t("editor.confirmTitle")}>
          <div className="space-y-4">
            <p className="text-sm text-gray-600">{t("editor.confirmBody")}</p>
            <div className="flex gap-2">
              <Button
                className="flex-1"
                onClick={() => void doExport()}
                disabled={exporting}
              >
                {exporting ? t("editor.flattening") : t("common.download")}
              </Button>
              <Button
                variant="secondary"
                className="flex-1"
                onClick={() => setConfirmExport(false)}
                disabled={exporting}
              >
                {t("editor.keepEditing")}
              </Button>
            </div>
          </div>
        </Modal>
      )}
    </div>
  );
}

// triggerDownload starts a browser download of an export. The download attribute makes
// the browser treat it as a download (not a navigation), so it doesn't fire the
// unsaved-changes beforeunload prompt.
function triggerDownload(exportId: string) {
  const a = document.createElement("a");
  a.href = `/api/exports/${exportId}/file`;
  a.download = "";
  a.rel = "noopener";
  document.body.appendChild(a);
  a.click();
  a.remove();
}
